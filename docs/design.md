# Licen 授权平台总体设计（v1.0 定稿）

> 状态：方案已评审（2026-08-11）
> 决策：授权平台本体用 **Go** 实现（开发者可控、防逆向门槛高于 Java、单二进制部署）
> Java 版 licen-server 作为协议参考实现保留于 `licen-server-java-legacy/`（deprecated）

---

## 1. 项目定位

为私有化交付的产品（AI 引擎、应用服务、边缘网关等，Java/Python/Go/C 多技术栈）提供**统一授权管理**：

- 授权服务仅允许部署在**指定物理机/VM**（硬件指纹绑定）
- 控制客户产品**并发节点数**、**功能点**、**有效期**
- 防篡改（RSA 签名）、防伪造（私钥仅厂商持有）、防迁移（机器码绑定）、防逆向（Go + 混淆）

## 2. 总体架构

```
┌────────────────────────────────────────────────────────────┐
│ 厂商侧（开发机）                                              │
│  licen-tool (Go CLI)  ── 生成密钥对 / 签发License / 验证      │
│         │ 私钥自留，公钥内置到 licen-server 二进制             │
└────────┼───────────────────────────────────────────────────┘
         ▼ License 文件（JSON + RSA 签名，绑定机器码）
┌────────────────────────────────────────────────────────────┐
│ 客户侧（专用 VM）                                            │
│  licen-server (Go 单二进制)                                  │
│   ├─ 启动采集本机机器码 → 验签 → 匹配 → 有效期                 │
│   ├─ REST API：注册 / 心跳 / 状态查询 / 管理                  │
│   ├─ 节点并发控制（maxNodes）+ 心跳超时回收                    │
│   └─ SQLite 存储（节点/应用/审计，零部署成本）                 │
└────────────┬───────────────────────────────────────────────┘
             ▲ 注册/心跳/状态（REST + HMAC 签名）
   ┌─────────┼─────────┬─────────┐
   │         │         │         │
 licen-sdk  licen-sdk  licen-sdk  licen-sdk
 -java      -python    -go       -c
 (已有)     (pip)     (module)  (lib)
```

## 3. 授权平台技术选型（Go）

| 组件 | 选型 | 理由 |
|---|---|---|
| 语言 | **Go 1.22+** | 开发者可维护；静态编译单二进制；逆向门槛高于 Java |
| HTTP 框架 | **标准库 net/http**（Go 1.22 路由增强） | 接口少，零框架依赖，减少攻击面 |
| 存储 | **SQLite**（modernc.org/sqlite 纯 Go 驱动） | 单文件，客户 VM 零部署；无 CGO 可交叉编译 |
| 配置 | YAML（gopkg.in/yaml.v3）或环境变量 | 简单 |
| 机器码采集 | sysfs 直接读取（Linux）+ WMI（Windows） | 主板/CPU/MAC/磁盘序列号，无需第三方 OSHI |
| 密码学 | 标准库 crypto/rsa、crypto/hmac、crypto/sha256 | RSA-2048 验签、HMAC 签名 |
| 日志 | 标准库 log/slog | 零依赖 |
| 防逆向加固 | L1 去符号+静态链接(musl) + L2 garble 混淆 | 见 §5 |

### Go 模块结构（licen-server）

```
licen-server/
├── cmd/licen-server/main.go        # 入口
├── internal/
│   ├── config/                     # 配置加载
│   ├── machine/                    # 机器码采集（Linux sysfs / Windows WMI）
│   ├── license/                    # License 模型 + 验签 + 有效期校验
│   ├── store/                      # SQLite：节点/应用/审计
│   ├── node/                       # 节点注册/心跳/名额控制/超时回收
│   └── api/                        # REST handlers（/api/v1/*）
├── keys/public.pem                 # 内置公钥（构建时替换）
├── config.yaml                     # 部署配置
└── go.mod
```

### licen-tool（Go CLI，同样重写）

```
licen-tool/
├── cmd/licen-tool/main.go
├── internal/
│   ├── keypair/   # gen-keypair
│   ├── license/   # gen-license / verify
│   └── machine/   # machinecode（复用 licen-server 的采集逻辑）
```

## 4. 多语言 SDK 方案（客户产品接入）

完整方案见 `docs/multi-language-sdk-design.md`，要点：

- **协议为中心**：REST + HMAC 契约是唯一事实来源，server 换 Go 不影响 SDK
- **SDK 矩阵**：Java（已有）/ Python（零依赖）/ Go（零依赖）/ C（libcurl + 纯 socket 双模式）
- SDK 为纯客户端：注册、心跳、宽限期、自愈、能力校验；**不含验签核心**（验签只在 server）
- 各语言行为一致：状态机（REGISTERED→HEALTHY→GRACE→DEGRADED）+ 统一错误码

## 5. 安全加固方案

| 层次 | 手段 | 状态 |
|---|---|---|
| L1 | musl 静态链接、`-ldflags "-s -w"` 去符号、release 优化 | ✅ 必做 |
| L2 | **garble 混淆**（开源，Go 专用） | ✅ 必做 |
| L3 | 防调试（ptrace 检测）、二进制自校验、关键逻辑内联 | ⏸️ 二期按需 |
| L4 | RSA-2048 签名防伪造（架构保证） | ✅ 已具备 |

> 认知：客户侧软件无绝对防逆向，L1+L2 已将逆向成本提升至"专业级"，对绝大多数客户足够。

## 6. 仓库结构（最终形态）

```
Licen/
├── licen-server/          # Go 授权服务（客户侧，开发中）
├── licen-tool/            # Go 厂商 CLI（开发中）
├── licen-sdk/             # Java SDK（已有，客户 Java 产品接入）
├── licen-sdk-python/      # Python SDK（待开发）
├── licen-sdk-go/          # Go SDK（待开发）
├── licen-sdk-c/           # C SDK（待开发）
├── licen-examples/        # 接入示例（example-app 为 Java）
└── docs/                  # 设计文档 + 协议契约 + OpenAPI
```

> 注：Java 版授权平台本体（licen-server/licen-tool/licen-core）已按决策移除，不保留 legacy。
> 授权平台本体统一为 Go；Java 仅在客户产品接入侧（licen-sdk）保留。

## 7. 开发路线图

| 阶段 | 内容 | 依赖 |
|---|---|---|
| P0 | 协议契约文档 docs/protocol.md + OpenAPI 固化 | 无 |
| P1 | **licen-server Go 重写**（机器码/验签/节点/心跳/管理 API + SQLite） | P0 |
| P2 | **licen-tool Go 重写**（密钥对/签发/验证/机器码） | P1 的 machine/license 复用 |
| P3 | Go SDK（licen-sdk-go） | P1 完成（联调） |
| P4 | Python SDK（licen-sdk-python） | 可与 P3 并行 |
| P5 | C SDK（licen-sdk-c） | 可与 P3/P4 并行 |
| P6 | 集成测试矩阵 + CI（GitHub Actions） | P1-P5 |
| P7 | 安全加固 L1+L2（garble 构建脚本） | P1/P2 |

## 8. 已确认决策

| # | 事项 | 决策 |
|---|---|---|
| 1 | 授权平台语言 | **Go**（licen-server + licen-tool 统一，开发者可控） |
| 2 | Java 版平台本体 | **移除，不保留 legacy**（licen-core/licen-tool/licen-server 已删） |
| 3 | 加固级别 | L1 去符号+静态链接 + L2 garble 混淆；L3 二期按需 |
| 4 | Java SDK | 保留（licen-sdk，客户 Java 产品接入用，已从 licen-core 解耦） |
