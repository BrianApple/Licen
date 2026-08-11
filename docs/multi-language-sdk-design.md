# Licen 多语言 SDK 设计方案

> 版本：v1.0 草案
> 状态：待评审
> 关联：licen-server 授权服务（协议已实现并验证）

---

## 1. 设计目标

客户产品技术栈多样（Java / Python / Go / C），授权 SDK 需覆盖主要语言，且**行为一致、接入成本低、适配内网部署**。

- **一套协议，多语言覆盖**：Java（已有）、Python、Go、C 四语言 SDK 共享同一份协议契约
- **行为一致性**：注册、心跳、宽限期、自愈、能力校验在各语言 SDK 表现完全一致
- **零依赖优先**：客户环境多为内网/离线，SDK 尽量不引入第三方依赖
- **C 适配嵌入式**：C SDK 需兼顾资源受限环境（可无 libcurl）

## 2. 总体架构：以协议为中心

```
                    ┌─────────────────────────┐
                    │  licen-server (REST+HMAC) │
                    └────────────┬────────────┘
                                 │ 统一协议契约（唯一事实来源）
              ┌──────────────────┼───────────────────┐
              │                  │                   │
      licen-sdk-java      licen-sdk-python    licen-sdk-go
      (Spring Starter)      (pip, 零依赖)     (go module, 零依赖)
              │                  │                   │
      licen-sdk-c (libcurl 或纯 socket)         纯 REST 直连（任意语言兜底）
```

**核心原则**：协议契约是唯一事实来源（docs/protocol.md + OpenAPI），各语言 SDK 按契约独立实现，服务端新增字段向后兼容，不破坏旧 SDK。

## 3. 统一协议规范（现有契约固化）

### 3.1 端点

| 端点 | 方法 | 用途 | 是否需要签名 |
|---|---|---|---|
| `/api/v1/nodes/register` | POST | 节点注册/续约 | 否（用 appKey+appSecret 认证） |
| `/api/v1/nodes/heartbeat` | POST | 心跳保活 | 是（HMAC） |
| `/api/v1/license/status` | GET | 查询授权状态 | 否（内部网络；可加可选签名） |
| `/api/v1/health` | GET | 健康检查 | 否 |

### 3.2 签名算法（所有语言必须一致实现）

```
sign = HMAC-SHA256(appSecret, nodeId + ":" + timestamp)   // hex 小写
timestamp = 当前毫秒时间戳
服务端校验：常量时间比较 + 时间戳 ±5 分钟防重放
```

### 3.3 错误码枚举（SDK 需能识别）

```
APP_NOT_FOUND / APP_AUTH_FAILED / LICENSE_INVALID:<原因> / PRODUCT_MISMATCH
NODE_LIMIT_REACHED / NODE_NOT_FOUND / SIGN_INVALID / TIMESTAMP_REJECTED / TIMESTAMP_INVALID
```

### 3.4 统一状态机（跨语言一致）

```
        启动
         │
         ▼
     ┌────────┐  register 成功   ┌─────────┐  心跳成功   ┌─────────┐
     │ UNREGISTERED │───────────→│ HEALTHY │←───────────│(持续心跳)│
     └────────┘                 └─────────┘            └─────────┘
         │ 失败(不阻塞启动)            │ 心跳连续失败
         ▼                           ▼
     ┌─────────┐   超过宽限期    ┌─────────┐
     │  GRACE  │──────────────→│ DEGRADED │
     └─────────┘               └─────────┘
         ▲                            │
         └───── 心跳恢复 ─────────────┘
  心跳返回 NODE_NOT_FOUND → 自动重新注册（自愈）
```

## 4. 各语言 SDK 设计

### 4.0 通用功能矩阵（四语言全部实现）

| 功能 | 说明 | 优先级 |
|---|---|---|
| 配置加载 | server-url / app-key / app-secret / node-name / interval / grace / timeout | P0 |
| 启动注册 | 启动时一次，失败不阻塞（进宽限期） | P0 |
| 定时心跳 | 默认 30s，HMAC 签名 | P0 |
| 状态缓存 | 内存缓存最新授权状态（valid/expiresAt/features/maxNodes） | P0 |
| 能力校验 | `isValid()` / `hasFeature(name)` / `getStatus()` | P0 |
| 离线宽限期 | 断联 grace 秒内视为有效（默认 300s） | P0 |
| 自动自愈 | 心跳 NODE_NOT_FOUND → 自动重注册 | P0 |
| 状态持久化 | 本地文件缓存授权状态，重启先读缓存（防启动即断网误杀） | P1 |
| 降级回调 | 状态变化时回调/事件通知（产品可感知并决定行为） | P1 |
| 手动刷新 | `refresh()` 主动拉取 | P1 |

### 4.1 Python SDK（licen-sdk-python）

- **形态**：pip 包，`pip install licen-sdk`，Python 3.9+
- **依赖策略**：零第三方依赖（stdlib：`urllib.request` / `hmac` / `hashlib` / `threading` / `json`）
  - 理由：客户内网环境 `pip install` 不便，AI 推理服务多为 Python，部署环境受控
- **线程模型**：后台 daemon 线程心跳，主线程查询状态
- **API 示例**：
  ```python
  from licen_sdk import LicenClient
  client = LicenClient(
      server_url="http://10.0.0.10:8090",
      app_key="ai-engine",
      app_secret="xxx",
  )
  client.start()
  client.is_valid()               # bool
  client.has_feature("ai-inference")  # bool
  client.get_status()             # dict
  client.stop()
  ```
- **适配场景**：AI 推理服务、数据处理服务、模型服务
- **可选**：异步模式（asyncio）二期；`licen-sdk-python` 同时提供 CLI 子命令 `licen-sdk status`

### 4.2 Go SDK（licen-sdk-go）

- **形态**：Go module，`go get github.com/BrianApple/licen-sdk-go`
- **依赖策略**：仅标准库 `net/http`（零第三方依赖）
- **并发模型**：goroutine 心跳 + `context.Context` 取消；`sync.RWMutex` 保护状态
- **API 示例**：
  ```go
  client, err := licen.NewClient(licen.Config{
      ServerURL: "http://10.0.0.10:8090",
      AppKey:    "ai-engine",
      AppSecret: "xxx",
  })
  client.Start(ctx)
  client.IsValid()                  // bool
  client.HasFeature("ai-inference") // bool
  client.Stop()
  ```
- **适配场景**：Go 微服务、网关、k8s sidecar、云原生组件
- **附加**：提供 `licen.StatusListener` 接口实现降级回调

### 4.3 C SDK（licen-sdk-c）

- **形态**：静态库 `.a` / 动态库 `.so`（Windows `.lib`/`.dll` 二期）+ 头文件 `licen.h`；CMake + Makefile 双构建；支持交叉编译（arm64）
- **依赖策略（关键决策，需确认）**：
  - **模式 A（默认）**：libcurl + pthread —— Linux 普遍存在，功能完整，TLS 可扩展
  - **模式 B（可选）**：`LICEN_NO_CURL=1` 时用纯 POSIX socket 自实现最小 HTTP 客户端 —— 零依赖，适配嵌入式/资源受限；无 TLS（内网部署可接受）
  - 建议：**A 默认 + B 宏开关**，同一套 API 两种实现
- **线程模型**：pthread 后台心跳线程；提供 `licen_thread_safe` 保证
- **内存模型**：句柄式 + 查询式 API（无回调也能用），可选回调通知
  ```c
  #include <licen.h>
  licen_handle_t h = licen_init(&(licen_config_t){
      .server_url = "http://10.0.0.10:8090",
      .app_key = "ai-engine",
      .app_secret = "xxx",
      .node_name = "edge-gw-1",
  });
  licen_start(h);
  bool ok  = licen_is_valid(h);
  bool fea = licen_has_feature(h, "ai-inference");
  licen_stop(h);
  licen_destroy(h);
  ```
- **适配场景**：边缘网关、嵌入式引擎、IOTGate 类 C/C++ 产品
- **附加价值**：C ABI 稳定后，C++/Rust/Node(N-API)/.NET 可通过 FFI 复用

### 4.4 兜底方案：纯 REST 直连

任何语言（含未覆盖的 Rust/Node.js/PHP 等）可直接按 `docs/protocol.md` 调用 REST API，无 SDK 也能接入；SDK 的价值在于心跳/宽限期/自愈/缓存逻辑封装。

## 5. 发布与版本管理

| 语言 | 渠道 | 备注 |
|---|---|---|
| Java | Maven Central（或私有仓库） | 已有，保持 1.0.0-SNAPSHOT |
| Python | PyPI `licen-sdk` | 开源项目建议公开 |
| Go | GitHub Releases + Go Proxy | `github.com/BrianApple/licen-sdk-go` |
| C | GitHub Releases 源码包 + 编译产物（linux-x86_64/arm64） | 附 CMake/Makefile |

- **版本策略**：所有 SDK 与 server 协议版本 v1 兼容；server 只增不改字段；SDK 各自独立版本号，遵循语义化版本
- **协议契约**：`docs/protocol.md`（人类可读）+ `docs/openapi.yaml`（机器可读）为唯一事实来源，SDK 变更必须同步契约

## 6. 测试策略

1. **契约测试**：每个语言 SDK 对同一 mock 响应集合断言相同行为（字段解析/错误码映射）
2. **集成测试**：启动真实 licen-server，各语言 SDK 依次验证：
   - 注册成功 → 心跳保活 → 状态查询
   - 超名额拒绝（NODE_LIMIT_REACHED）
   - 错误密钥/错误签名拒绝
   - 断联 → 宽限期 → 降级 → 恢复
   - 节点被清理 → 自动重注册（自愈）
3. **CI**：GitHub Actions 矩阵（java/python/go/c × linux），server 升级回归
4. **测试仓库**：`licen-sdk-tests` 共享测试场景定义

## 7. 里程碑与工作量

| 里程碑 | 内容 | 工作量 |
|---|---|---|
| M1 | 协议契约文档 + OpenAPI 固化 | 0.5 人日 |
| M2 | Python SDK（零依赖） | 1 人日 |
| M3 | Go SDK（零依赖） | 1 人日 |
| M4 | C SDK（libcurl + 纯 socket 双模式） | 2 人日 |
| M5 | 三语言集成测试 + CI 矩阵 | 1 人日 |
| **合计** | | **约 5.5 人日**（M2/M3/M4 可并行） |

## 8. 关键决策点（需产品确认）

| # | 决策点 | 默认建议 | 备选 |
|---|---|---|---|
| 1 | C SDK 依赖策略 | libcurl 默认 + `LICEN_NO_CURL` 纯 socket 双模式 | 只做其中一种 |
| 2 | 是否追加 Node.js / Rust SDK | 一期不做（纯 REST 兜底） | 按客户技术栈追加 |
| 3 | 宽限期过后产品行为 | SDK 只报告降级状态（回调），**不强制停止**，由产品决定 | 支持 SDK 强制停服配置 |
| 4 | 发布渠道 | 公开（PyPI / GitHub Releases / Go Proxy） | 私有仓库（商业授权产品） |
| 5 | 本地状态持久化 | 做（重启先读缓存，防启动断网误杀） | 不做（每次启动必须连服务器） |
| 6 | 状态查询鉴权 | `/license/status` 保持开放（内网） | 加可选签名 |
