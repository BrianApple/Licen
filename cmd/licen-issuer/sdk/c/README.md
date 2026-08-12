# Licen C SDK

C 语言授权客户端 SDK：注册 / 心跳 / 授权能力校验。

**双模式构建**：
- **libcurl 模式**（默认）：功能完整，依赖 `libcurl + pthread`
- **纯 socket 模式**（零依赖）：`make no_curl`，仅 POSIX socket，适配嵌入式/资源受限环境（无 TLS，内网部署）

核心实现零第三方依赖：SHA-256/HMAC 手写、极简 JSON 解析器内嵌。

## 构建

```bash
# libcurl 模式
make                 # 生成 liblicen.a + licen-demo

# 纯 socket 模式（零依赖）
make no_curl

# CMake 方式
cmake -B build && cmake --build build
cmake -B build -DLICEN_NO_CURL=ON && cmake --build build
```

## 使用

```c
#include <licen.h>

licen_config_t cfg;
memset(&cfg, 0, sizeof(cfg));
cfg.server_url = "http://10.0.0.10:8090";
cfg.app_key = "hxapigate";
cfg.app_secret = "xxx";
cfg.node_name = "edge-gw-1";

licen_client_t *c = licen_init(&cfg);
licen_start(c);                        // 注册 + 后台心跳线程

if (licen_is_valid(c)) {
    if (licen_has_feature(c, "ai-inference")) {
        // 执行业务
    }
}

licen_get_status(c, &st);              // 状态快照
licen_stop(c);
licen_destroy(c);
```

## API

| 函数 | 说明 |
|---|---|
| `licen_init` | 创建客户端（配置必填 server_url/app_key/app_secret） |
| `licen_start` | 注册 + 启动心跳线程（失败不阻塞，进入宽限期） |
| `licen_stop` / `licen_destroy` | 停止 / 释放 |
| `licen_is_valid` | 是否持有有效授权 |
| `licen_has_feature` | 是否拥有指定功能点 |
| `licen_is_degraded` | 授权中心不可达且过宽限期 |
| `licen_get_status` | 拷贝状态快照 |
| `licen_refresh` | 主动刷新 |
| `licen_on_status_change` | 状态变化回调 |
| `licen_last_error` | 最近错误信息 |

线程安全：所有查询 API 可在任意线程调用。协议契约见 `docs/protocol.md`。
