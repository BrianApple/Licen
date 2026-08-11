# Licen Python SDK

零第三方依赖的授权客户端 SDK（注册 / 心跳 / 授权能力校验）。

## 安装

```bash
pip install .        # 或 pip install licen-sdk（发布 PyPI 后）
```

## 使用

```python
from licen_sdk import LicenClient

client = LicenClient(
    server_url="http://10.0.0.10:8090",   # 授权服务地址（客户 VM）
    app_key="ai-engine",                   # 授权服务管理端创建的应用
    app_secret="xxx",
    node_name="ai-inference-1",            # 可选
)
client.start()

# 业务侧能力校验
if client.is_valid():
    if client.has_feature("ai-inference"):
        run_inference()

# 状态变化回调（可选）
def on_change(status):
    print("授权状态变化:", status.valid)

client.on_status_change(on_change)

# 退出时
client.stop()
```

## 配置项

| 参数 | 默认 | 说明 |
|---|---|---|
| server_url | - | 授权服务地址（必填） |
| app_key | - | 应用标识（必填） |
| app_secret | - | 应用密钥（必填） |
| node_name | hostname | 节点名称 |
| heartbeat_interval | 30 | 心跳间隔（秒） |
| grace_period | 300 | 离线宽限期（秒） |
| connect_timeout | 3 | 连接超时（秒） |

协议契约见 `docs/protocol.md`。
