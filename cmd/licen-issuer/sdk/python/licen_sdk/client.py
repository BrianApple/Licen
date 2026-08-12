"""Licen 授权客户端核心实现（零第三方依赖）。

行为与 Go/Java/C SDK 一致（见 docs/protocol.md）：
- 启动注册，失败不阻塞（进入宽限期）
- 定时心跳（HMAC-SHA256 签名），断联宽限期内视为有效
- 节点被清理自动重新注册（自愈）
- 提供 is_valid / has_feature 能力校验
"""

import hashlib
import hmac
import json
import platform
import socket
import threading
import time
import uuid
from dataclasses import dataclass, field
from typing import Callable, List, Optional
from urllib import request, error as urlerror

__all__ = ["LicenClient", "Status"]


@dataclass
class Status:
    """授权状态快照"""

    valid: bool = False
    license_id: Optional[str] = None
    expires_at: Optional[str] = None
    max_nodes: int = -1
    online_nodes: int = -1
    features: List[str] = field(default_factory=list)
    last_sync_at: float = 0.0


class LicenError(Exception):
    """SDK 异常"""


# 默认产品标识：通用 SDK 为空；厂商定制版（签发服务下载）会把占位符
# 替换为所选产品 ID，注册/心跳默认携带该产品自声明。
DEFAULT_PRODUCT = "__LICEN_DEFAULT_PRODUCT__"


class LicenClient:
    """Licen 客户端"""

    def __init__(
        self,
        server_url: str,
        app_key: str,
        app_secret: str,
        product: Optional[str] = None,
        node_name: Optional[str] = None,
        heartbeat_interval: float = 30.0,
        grace_period: float = 300.0,
        connect_timeout: float = 3.0,
    ):
        if not server_url or not app_key or not app_secret:
            raise ValueError("server_url / app_key / app_secret 为必填项")
        self._server_url = server_url.rstrip("/")
        self._app_key = app_key
        self._app_secret = app_secret
        self._product = product or DEFAULT_PRODUCT  # 产品标识（自声明，须与管理端 App 绑定产品一致）
        self._node_name = node_name or socket.gethostname()
        self._heartbeat_interval = heartbeat_interval
        self._grace_period = grace_period
        self._connect_timeout = connect_timeout

        self._node_id = str(uuid.uuid4())
        self._status = Status()
        self._degraded = False
        self._last_contact_at = 0.0
        self._lock = threading.RLock()
        self._on_change: Optional[Callable[[Status], None]] = None

        self._thread: Optional[threading.Thread] = None
        self._stop_event = threading.Event()

    # ---------- 对外 API ----------

    @property
    def node_id(self) -> str:
        return self._node_id

    def start(self) -> None:
        """启动：注册 + 后台心跳线程"""
        if self._thread and self._thread.is_alive():
            return
        self._stop_event.clear()
        # 首次注册（失败不阻断启动，进入宽限期重试）
        try:
            self._register()
            self._set_degraded(False)
        except Exception as exc:  # noqa: BLE001
            self._mark_degraded(f"注册失败: {exc}")

        self._thread = threading.Thread(target=self._heartbeat_loop, daemon=True, name="licen-sdk-heartbeat")
        self._thread.start()

    def stop(self) -> None:
        """停止心跳线程"""
        self._stop_event.set()
        if self._thread:
            self._thread.join(timeout=5)

    def is_valid(self) -> bool:
        """是否持有有效授权（最近一次同步状态）"""
        with self._lock:
            return self._status.valid

    def has_feature(self, feature: str) -> bool:
        """是否拥有指定功能点"""
        with self._lock:
            return self._status.valid and feature in self._status.features

    def is_degraded(self) -> bool:
        """授权中心是否不可达且已过宽限期"""
        with self._lock:
            return self._degraded

    def status(self) -> Status:
        """当前授权状态"""
        with self._lock:
            return Status(**self._status.__dict__)

    def refresh(self) -> None:
        """主动向授权中心拉取最新状态"""
        self._refresh()

    def on_status_change(self, callback: Callable[[Status], None]) -> None:
        """注册状态变化回调"""
        self._on_change = callback

    # ---------- 内部实现 ----------

    def _heartbeat_loop(self) -> None:
        while not self._stop_event.is_set():
            time.sleep(self._heartbeat_interval)
            try:
                self._heartbeat()
                self._set_degraded(False)
            except Exception as exc:  # noqa: BLE001
                # 节点被清理/服务重启 → 自动重新注册（自愈）
                if "NODE_NOT_FOUND" in str(exc):
                    try:
                        self._register()
                        self._set_degraded(False)
                        continue
                    except Exception:  # noqa: BLE001
                        pass
                if time.time() - self._last_contact_at > self._grace_period:
                    self._mark_degraded(f"心跳连续失败: {exc}")

    def _register(self) -> None:
        body = json.dumps({
            "appKey": self._app_key,
            "appSecret": self._app_secret,
            "product": self._product,
            "nodeId": self._node_id,
            "nodeName": self._node_name,
            "ip": self._local_ip(),
            "version": "licen-sdk-python-1.0.0",
        }).encode("utf-8")
        data = self._post("/api/v1/nodes/register", body)
        self._apply_register_response(data)
        self._last_contact_at = time.time()
        try:
            self._refresh()  # 拉取完整状态（含 features）
        except Exception:  # noqa: BLE001
            pass

    def _heartbeat(self) -> None:
        ts = int(time.time() * 1000)
        sign = self._hmac(f"{self._node_id}:{ts}")
        body = json.dumps({
            "appKey": self._app_key,
            "product": self._product,
            "nodeId": self._node_id,
            "timestamp": str(ts),
            "sign": sign,
        }).encode("utf-8")
        data = self._post("/api/v1/nodes/heartbeat", body)
        if not data.get("success", False):
            raise LicenError(data.get("message", "UNKNOWN"))
        self._last_contact_at = time.time()
        self._apply_register_response(data)

    def _refresh(self) -> None:
        data = self._get("/api/v1/license/status")
        with self._lock:
            old_valid = self._status.valid
            old_license = self._status.license_id
            self._status = Status(
                valid=bool(data.get("valid", False)),
                license_id=data.get("licenseId"),
                expires_at=data.get("expiresAt"),
                max_nodes=int(data.get("maxNodes", -1)),
                online_nodes=int(data.get("onlineNodes", -1)),
                features=list(data.get("features", []) or []),
                last_sync_at=time.time(),
            )
            changed = old_valid != self._status.valid or old_license != self._status.license_id
        if changed:
            self._notify_change()

    def _apply_register_response(self, data: dict) -> None:
        with self._lock:
            old_valid = self._status.valid
            old_license = self._status.license_id
            self._status.valid = bool(data.get("success", False))
            self._status.license_id = data.get("licenseId") or self._status.license_id
            self._status.expires_at = data.get("expiresAt") or self._status.expires_at
            self._status.online_nodes = int(data.get("onlineNodes", self._status.online_nodes))
            self._status.max_nodes = int(data.get("maxNodes", self._status.max_nodes))
            self._status.last_sync_at = time.time()
            changed = old_valid != self._status.valid or old_license != self._status.license_id
        if changed:
            self._notify_change()

    def _post(self, path: str, body: bytes) -> dict:
        req = request.Request(
            f"{self._server_url}{path}",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        return self._send(req)

    def _get(self, path: str) -> dict:
        req = request.Request(f"{self._server_url}{path}", method="GET")
        return self._send(req)

    def _send(self, req: request.Request) -> dict:
        try:
            with request.urlopen(req, timeout=self._connect_timeout) as resp:
                raw = resp.read().decode("utf-8")
        except urlerror.HTTPError as exc:
            raise LicenError(f"HTTP {exc.code}") from exc
        except urlerror.URLError as exc:
            raise LicenError(f"网络错误: {exc.reason}") from exc
        return json.loads(raw)

    def _hmac(self, data: str) -> str:
        return hmac.new(
            self._app_secret.encode("utf-8"),
            data.encode("utf-8"),
            hashlib.sha256,
        ).hexdigest()

    def _local_ip(self) -> str:
        try:
            s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            s.connect(("8.8.8.8", 80))
            ip = s.getsockname()[0]
            s.close()
            return ip
        except Exception:  # noqa: BLE001
            return ""

    def _mark_degraded(self, reason: str) -> None:
        with self._lock:
            was = self._degraded
            self._degraded = True
        if not was:
            print(f"[licen-sdk] ⚠️ 授权中心不可达，进入宽限期: {reason}")

    def _set_degraded(self, value: bool) -> None:
        with self._lock:
            self._degraded = value

    def _notify_change(self) -> None:
        callback = self._on_change
        if callback:
            try:
                callback(self.status())
            except Exception:  # noqa: BLE001
                pass
