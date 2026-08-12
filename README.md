# 🔑 Licen —— 私有化部署产品授权服务

> 为交付到客户现场的私有化软件（AI 引擎、应用服务、边缘网关等）提供**开箱即用的授权管理方案**：
> **先部署后激活**、硬件绑定、节点并发控制、功能点授权、防篡改防逆向，一条龙搞定。

[![文档站](https://img.shields.io/badge/📚%20文档站-BrianApple.github.io-38bdf8)](https://BrianApple.github.io/docs/licen/intro)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)
[![SDK](https://img.shields.io/badge/SDK-Java%20%7C%20Go%20%7C%20Python%20%7C%20C-orange)](README.md)

---

## ✨ 核心功能

| 能力 | 说明 |
|---|---|
| **先部署后激活** | 客户先装好系统 → 机器码发给厂商 → 一键签发 → 上传即激活，无需等待 |
| **硬件绑定防拷贝** | License 绑定硬件指纹锚点（系统UUID→磁盘→MAC→主板→CPU）SHA-256 机器码，整套拷走也跑不起来 |
| **防篡改防逆向** | RSA-2048 私钥签名（改一个字段即失效）+ Go 静态编译去符号 + garble 混淆 |
| **节点并发控制** | License 限定最大并发节点数，心跳保活、超时自动回收名额，防僵尸占用 |
| **多语言 SDK** | Java / Go / Python / C 四语言官方 SDK，统一协议契约，接入 5 分钟 |
| **Web 签发中心** | 输入机器码一键签发；已签发授权全量台账（搜索/吊销/重新签发/授权时序） |
| **客户维度管理** | 客户-产品对应绑定、客户档案归档（证书+SDK 按客户落盘）、到期提醒（30 天标临期） |
| **产品库 & SDK 分发** | 管理可授权产品，按 语言×产品 下载定制版 SDK（内嵌产品标识） |
| **零外部依赖** | `licen-server` 单二进制 + SQLite，不依赖 MySQL/Redis/Nginx，拷到客户 VM 即跑 |

## 🎯 项目价值

| 维度 | Licen | 传统自研/商业方案 |
|---|---|---|
| **交付体验** | 先部署后激活，无需现场授权或邮寄加密狗 | 周期长，实施成本高 |
| **部署形态** | 单二进制 + SQLite，零外部依赖 | 通常要配套数据库、中间件 |
| **防拷贝** | 硬件指纹锚点，加盘/换网卡/VM 迁移不变码 | 单纯 License 文件可随意复制 |
| **成本** | 纯 Go 开源，无按量计费、可完全离线运行 | 按节点/按年收费的授权网关 |

## 📸 截图

| 厂商签发中心（输入机器码一键签发） | 客户侧管理平台（授权状态/节点/应用/审计） |
|---|---|
| ![签发界面](docs/screenshots/01-issuer-webui.png) | ![管理平台](docs/screenshots/06-server-admin-platform.png) |

## 🎯 应用场景

1. **私有化软件交付**（最典型）：AI 推理引擎、NLP 平台等交付客户内网，控制"谁能用、用几台、用多久、用哪些功能"
2. **边缘网关 / 嵌入式设备**：C SDK 纯 socket 零依赖模式，可嵌入 ARM 环境
3. **SaaS 转私有化**：按年授权、按并发节点计费、按功能点区分版本
4. **项目型软件（集成商/代理）**：统一管理所有客户授权，Web 签发中心一目了然
5. **试用版 / PoC 引流**：签发 30 天试用 License，到期自动停，满意再续费

## 🚀 快速开始（本地全链路）

```bash
# 构建
go build -o licen-server ./cmd/licen-server
go build -o licen-tool ./cmd/licen-tool

# 生成密钥对（私钥厂商自留，公钥内置到服务端）
./licen-tool gen-keypair -d ./keys

# 查看本机机器码（客户 VM 上执行，报给厂商）
./licen-tool machinecode

# 厂商签发 License（绑定机器码、节点数、功能点、有效期）
./licen-tool gen-license -k keys/private.pem \
    -m <机器码> -p hxapigate -n 10 -f ai-inference,nlp -d 365 -c "公司一" -o license.json

# 部署授权服务（客户 VM）：放置 license.json + keys/public.pem + config.yaml
./licen-server -c config.yaml
# 日志出现 "License 加载 result=VALID" 即授权生效
```

## 📚 详细文档（文档站）

完整教程已迁移至文档站，**后续文档更新以文档站为核心**：

| 文档 | 链接 |
|---|---|
| 产品介绍 | https://BrianApple.github.io/docs/licen/intro |
| 快速开始 | https://BrianApple.github.io/docs/licen/quickstart |
| 架构设计 | https://BrianApple.github.io/docs/licen/architecture |
| 证书协议 | https://BrianApple.github.io/docs/licen/protocol |
| 多语言 SDK 接入 | https://BrianApple.github.io/docs/licen/sdk |
| 厂商签发中心 | https://BrianApple.github.io/docs/licen/issuer |

## 🛠 目录结构

```
cmd/licen-server/      授权服务主程序（先部署后激活）
cmd/licen-tool/        厂商 CLI 工具
cmd/licen-issuer/      厂商 Web 签发服务（机器码 → license）
licen-sdk/             Java 客户端 SDK（Spring Boot Starter）
licen-sdk-go/          Go SDK（零依赖）
licen-sdk-python/      Python SDK（零依赖）
licen-sdk-c/           C SDK（libcurl + 纯 socket 双模式）
licen-examples/        接入示例
scripts/               加固构建脚本（build-release.sh：静态+去符号+garble）
docs/                  设计文档 + 协议契约 + 演示密钥 + 截图
```

## 🔗 链接

- **GitHub 镜像**：https://github.com/BrianApple/Licen
- **开源文档站**：https://BrianApple.github.io （全部产品教程）

## License

GPL-3.0（开源项目，欢迎共建）
