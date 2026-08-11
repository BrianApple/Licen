# Licen —— 私有化部署产品授权服务

为交付到客户现场的私有化产品（AI 引擎、应用服务、边缘网关等）提供**统一授权管理**：

- 🔒 **硬件绑定**：授权服务仅允许部署在指定物理机/VM（主板/CPU/MAC/磁盘 → SHA-256 机器码）
- 📊 **节点并发控制**：License 限定最大并发节点数，心跳保活、超时自动回收
- 🎛️ **功能点管理**：按 License 授权功能点（如 ai-inference、nlp），SDK 一键校验
- ⏳ **有效期控制**：按时间授权，到期自动失效
- 🛡️ **防篡改防伪造**：RSA-2048 私钥签名（厂商自留），篡改/伪造即失效
- 🚫 **防逆向**：Go 静态编译 + 去符号 + garble 混淆（授权平台本体非 Java）

## 架构

```
┌────────────────────────────────────────────────┐
│ 厂商侧（开发机）                                 │
│  licen-tool (Go CLI)                           │
│    gen-keypair / gen-license / verify / machinecode │
│    私钥自留，公钥内置到 licen-server             │
└───────────────┬────────────────────────────────┘
                ▼ License 文件（JSON + RSA 签名，绑定机器码）
┌────────────────────────────────────────────────┐
│ 客户侧（专用 VM）                               │
│  licen-server (Go 单二进制 + SQLite)            │
│    验签 → 机器码匹配 → 有效期 → 节点并发控制      │
│    REST API：注册 / 心跳 / 状态查询 / 管理       │
└───────────────┬────────────────────────────────┘
                ▲ 注册/心跳（REST + HMAC 签名）
   ┌────────────┼───────────┬───────────┐
 licen-sdk    licen-sdk    licen-sdk   licen-sdk
 -java(已有)   -python      -go         -c
```

## 快速开始（本地全链路）

```bash
# 1. 构建
go build -o licen-server ./cmd/licen-server
go build -o licen-tool ./cmd/licen-tool

# 2. 生成密钥对（私钥厂商自留，公钥内置到服务端）
./licen-tool gen-keypair -d ./keys

# 3. 查看本机机器码（客户 VM 上执行，报给厂商）
./licen-tool machinecode

# 4. 厂商签发 License（绑定机器码、节点数、功能点、有效期）
./licen-tool gen-license -k keys/private.pem \
    -m <机器码> -p ai-engine -n 10 -f ai-inference,nlp -d 365 -c "某公司" -o license.json

# 5. 部署授权服务（客户 VM）：放置 license.json + keys/public.pem + config.yaml
./licen-server -c config.yaml
# 日志出现 "License 加载 result=VALID" 即授权生效

# 6. 验证
curl http://<host>:8090/api/v1/health
curl http://<host>:8090/api/v1/license/status
```

## 部署指南（客户 VM）

1. 上传 `licen-server` 二进制、`config.yaml`、`license.json`、`keys/public.pem`
2. 修改 `config.yaml`：`admin-token`、端口、心跳超时
3. `./licen-server -c config.yaml`（systemd 托管见 `docs/systemd.md`，可选）
4. 首次启动后：`curl /api/v1/machine-code` 获取机器码 → 发给厂商签发 License
5. 管理端 API 需请求头 `X-Admin-Token`：
   - `GET /api/v1/admin/nodes` 节点列表
   - `POST /api/v1/admin/apps` 创建应用（appKey/appSecret 给产品 SDK 用）
   - `POST /api/v1/admin/license/reload` 热加载新 License

## 产品接入（SDK）

| 语言 | 状态 | 说明 |
|---|---|---|
| Java | ✅ 可用 | `licen-sdk/`，Spring Boot Starter，配置 `licen.sdk.*` 即接入 |
| Go | ✅ 可用 | `licen-sdk-go/`，零依赖，`go get` 即用 |
| Python | ✅ 可用 | `licen-sdk-python/`，零依赖，`pip install` |
| C | ✅ 可用 | `licen-sdk-c/`，libcurl + 纯 socket 双模式，适配嵌入式 |

协议契约见 `docs/protocol.md`（唯一事实来源）。

## 目录结构

```
cmd/licen-server/      授权服务主程序
cmd/licen-tool/        厂商 CLI 工具
internal/config/       配置加载
internal/machine/      机器码采集（Linux sysfs）
internal/license/      License 模型 + RSA 签名/验签/校验
internal/store/        SQLite 存储（节点/应用/审计）
internal/node/         节点注册/心跳/名额控制/回收
internal/api/          REST API
licen-sdk/             Java 客户端 SDK（Spring Boot Starter）
licen-examples/        接入示例
docs/                  设计文档 + 协议契约 + 演示密钥
```

## 安全设计

- **RSA-2048**：License 由厂商私钥签名，服务端公钥验签；公钥无法伪造签名
- **机器码绑定**：主板+CPU+主MAC+磁盘 → SHA-256，License 无法迁移服务器
- **HMAC 心跳**：appSecret 签名 + 时间戳防重放（±5min），常量时间比较
- **节点名额**：在线节点数 ≥ maxNodes 拒绝注册；心跳超时回收防僵尸占用
- **三层鉴权**：应用凭证（注册）+ HMAC 签名（心跳）+ 管理 Token（管理 API）
- **防逆向**：Go 静态编译（musl）、去符号（-s -w）、garble 混淆（构建脚本见 `scripts/build.sh`）

## License

GPL-3.0（开源项目，欢迎共建）
