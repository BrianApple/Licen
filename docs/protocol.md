# Licen 授权协议契约 v1（唯一事实来源）

> 所有 SDK（Java/Python/Go/C）与 licen-server 的通信契约。
> 服务端字段只增不改，向后兼容；SDK 必须忽略未知字段。

## 1. 基础信息

- Base URL：`http://<licen-server-host>:8090`
- 数据格式：JSON（UTF-8），`Content-Type: application/json`
- 时间字段：ISO-8601（如 `2027-08-11T15:32:19+08:00`）
- 时间戳：Unix 毫秒（签名用）

## 2. 端点

### 2.1 注册 POST /api/v1/nodes/register

客户端启动时调用（首次注册或重启续约）。同 nodeId 重复注册视为续约，不占新名额。

**请求**：
```json
{
  "appKey": "hxapigate",
  "appSecret": "xxx",
  "nodeId": "uuid-optional（客户端生成，可空则由服务端分配）",
  "nodeName": "hxapigate-1（可选）",
  "ip": "10.0.0.5（可选）",
  "version": "1.0.0（可选）"
}
```

**响应 200**：
```json
{
  "success": true,
  "message": "registered",
  "nodeId": "uuid",
  "licenseId": "LIC-XXXX",
  "expiresAt": "2027-08-11T15:32:19+08:00",
  "onlineNodes": 1,
  "maxNodes": 10
}
```

**失败响应**（HTTP 200 + success=false，或 4xx）：
```json
{ "success": false, "message": "NODE_LIMIT_REACHED", "onlineNodes": 1, "maxNodes": 1 }
```

### 2.2 心跳 POST /api/v1/nodes/heartbeat

定时保活（默认 30s）。**必须 HMAC 签名**。

**签名算法**：
```
sign = hex( HMAC-SHA256(appSecret, nodeId + ":" + timestamp) )
timestamp = 当前 Unix 毫秒
服务端校验：常量时间比较 + |now - timestamp| ≤ 5 分钟（防重放）
```

**请求**：
```json
{
  "appKey": "hxapigate",
  "nodeId": "uuid",
  "timestamp": "1786433000000",
  "sign": "hex-hmac"
}
```

**响应**：同 2.1（success=true 表示心跳接受，节点保活）

### 2.3 授权状态 GET /api/v1/license/status

查询当前授权状态（SDK 启动/刷新时调用）。

**响应 200**：
```json
{
  "valid": true,
  "result": "VALID",
  "machineCode": "sha256-hex",
  "licenseId": "LIC-XXXX",
  "product": "hxapigate",
  "edition": "enterprise",
  "customer": "公司一",
  "maxNodes": 10,
  "features": ["ai-inference", "nlp"],
  "issuedAt": "2026-08-11T00:00:00+08:00",
  "expiresAt": "2027-08-11T00:00:00+08:00"
}
```

### 2.4 健康检查 GET /api/v1/health

```json
{ "status": "UP", "time": 1786433000000, "licenseValid": true }
```

### 2.5 机器码查询 GET /api/v1/machine-code

部署后查询本机机器码（报给厂商签发 License）。

```json
{ "machineCode": "sha256-hex", "hint": "请将此机器码发送给厂商用于签发 License" }
```

### 2.6 激活 POST /api/v1/activate（先部署后激活）

**License 未激活时**：除 health / machine-code / license/status / activate 外，其余接口一律返回 HTTP 403：

```json
{ "success": false, "code": "LICENSE_NOT_ACTIVATED", "message": "License 未激活..." }
```

**激活请求**（两种格式，均无需管理 Token）：
```json
// 方式一：请求体直接为 license.json 内容
{ "licenseId": "...", "product": "licen-server", ..., "sign": "..." }

// 方式二：包装格式
{ "licenseContent": "{ \"licenseId\": \"...\", ... }" }
```

**成功响应 200**：
```json
{ "success": true, "message": "License 激活成功，全部功能已启用",
  "licenseId": "LIC-XXXX", "product": "licen-server", "edition": "enterprise",
  "customer": "公司二", "maxNodes": 50, "features": ["server-core"], "expiresAt": "..." }
```

**失败响应 400**（伪造/篡改/换机器均无法激活）：
```json
{ "success": false, "result": "MACHINE_MISMATCH", "message": "License 激活失败：MACHINE_MISMATCH（...）" }
```

### 2.7 管理接口 /api/v1/admin/**（需 X-Admin-Token 请求头）

| 端点 | 方法 | 说明 |
|---|---|---|
| /api/v1/admin/license/status | GET | 授权详情（含 License 全文） |
| /api/v1/admin/license/reload | POST | 重新加载 license.json |
| /api/v1/admin/nodes | GET | 节点列表（page/size 参数） |
| /api/v1/admin/nodes/{id} | DELETE | 强制下线（释放名额） |
| /api/v1/admin/apps | GET | 应用列表 |
| /api/v1/admin/apps | POST | 创建应用 {name,product,appKey,appSecret} |
| /api/v1/admin/apps/{id} | DELETE | 删除应用 |
| /api/v1/admin/audits | GET | 审计日志（page/size 参数） |

## 3. 错误码

| 错误码 | 含义 | SDK 行为 |
|---|---|---|
| LICENSE_NOT_ACTIVATED | License 未激活（先部署后激活模式） | 服务端 403，业务接口不可用；上传厂商 License 后自动恢复 |
| APP_NOT_FOUND | appKey 不存在 | 检查配置，终止注册（持续重试） |
| APP_AUTH_FAILED | appSecret 错误 | 检查配置，终止注册 |
| LICENSE_INVALID:<原因> | License 无效（EXPIRED/MACHINE_MISMATCH/INVALID_SIGNATURE） | 进入 DEGRADED，产品降级 |
| PRODUCT_MISMATCH | 应用与 License 产品不匹配 | 检查配置 |
| NODE_LIMIT_REACHED | 并发节点数已满 | 周期性重试注册 |
| NODE_NOT_FOUND | 心跳节点不存在（已被清理/服务重启） | **自动重新注册（自愈）** |
| SIGN_INVALID | 心跳签名错误 | 检查 appSecret/时钟 |
| TIMESTAMP_REJECTED | 时间戳偏差 > 5 分钟 | 同步时钟（NTP） |
| TIMESTAMP_INVALID | 时间戳格式错误 | SDK bug |

## 4. 统一状态机（SDK 必须实现）

```
        启动
         │
         ▼
   ┌────────────┐ register 成功   ┌─────────┐  心跳成功   ┌─────────┐
   │ UNREGISTERED │──────────────→│ HEALTHY │←───────────│(持续心跳)│
   └────────────┘                 └─────────┘            └─────────┘
         │ 失败(不阻塞启动)             │ 心跳连续失败
         ▼                            ▼
   ┌─────────┐   超过宽限期(默认300s)  ┌─────────┐
   │  GRACE  │──────────────────────→│ DEGRADED │
   └─────────┘                       └─────────┘
         ▲                               │
         └────── 心跳恢复 ────────────────┘
  心跳返回 NODE_NOT_FOUND → 自动重新注册（自愈）
```

- **宽限期（GRACE）**：授权中心不可达 grace 秒内（默认 300s），SDK 返回最近一次有效状态（防网络抖动误杀）
- **降级（DEGRADED）**：超过宽限期，`isValid()` 返回 false，产品自行决定行为（SDK 不强制停止）
- **自愈**：心跳 NODE_NOT_FOUND → 立即重新注册，成功即恢复 HEALTHY

## 5. 配置项（各语言 SDK 统一）

| 配置 | 默认 | 说明 |
|---|---|---|
| server-url | http://127.0.0.1:8090 | 授权服务地址 |
| app-key | - | 应用标识（必填） |
| app-secret | - | 应用密钥（必填） |
| node-name | hostname | 节点名称 |
| heartbeat-interval-seconds | 30 | 心跳间隔 |
| grace-period-seconds | 300 | 离线宽限期 |
| connect-timeout-ms | 3000 | 连接超时 |
