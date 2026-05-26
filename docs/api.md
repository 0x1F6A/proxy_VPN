# proxy_VPN API 设计文档

> 版本：v0.1
> 协议：HTTPS + JSON（对外）/ gRPC（内部）
> 鉴权：JWT (Access Token 2h) + Refresh Token (7d)
> Base URL：`https://api.example.com`
> 所有时间字段使用 ISO 8601 UTC（`2026-05-26T03:00:00Z`）

---

## 1. 通用规范

### 1.1 请求规范
- `Content-Type: application/json; charset=utf-8`
- 鉴权头：`Authorization: Bearer <access_token>`
- 幂等请求附带：`Idempotency-Key: <uuid>`（POST 类支付/下单接口必带）
- 客户端标识：`X-Client: web|ios|android|cli` + `X-Client-Version: 1.0.0`

### 1.2 统一响应结构
```json
{
  "code": 0,
  "message": "ok",
  "data": { },
  "request_id": "req_01HXYZ..."
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int | 0 = 成功；非 0 见错误码表 |
| message | string | 人类可读消息（多语言 by `Accept-Language`） |
| data | object/array/null | 业务数据 |
| request_id | string | 全链路追踪 ID（同时回写到响应头 `X-Request-Id`） |

### 1.3 分页规范
请求：`?page=1&page_size=20&sort=-created_at`
响应 data：
```json
{
  "list": [],
  "pagination": { "page": 1, "page_size": 20, "total": 123, "total_pages": 7 }
}
```

### 1.4 错误码（节选）
| code | HTTP | 含义 |
|------|------|------|
| 0 | 200 | 成功 |
| 40001 | 400 | 参数错误 |
| 40101 | 401 | 未登录 / token 过期 |
| 40102 | 401 | Refresh token 无效 |
| 40301 | 403 | 权限不足 |
| 40302 | 403 | 账号被禁用 |
| 40401 | 404 | 资源不存在 |
| 40901 | 409 | 资源冲突（如重复下单） |
| 42901 | 429 | 请求频率超限 |
| 50001 | 500 | 服务内部错误 |
| 50301 | 503 | 节点不可用 |
| 60001 | 200 | 余额不足 |
| 60002 | 200 | 套餐已过期 |
| 60003 | 200 | 流量配额已用尽 |
| 60101 | 200 | 支付订单已超时 |
| 60102 | 200 | 支付金额不匹配 |

### 1.5 限流策略（默认）
| 接口类型 | 限制 |
|----------|------|
| 登录 / 注册 / 找回密码 | 10 次 / IP / 分钟 |
| 支付下单 | 20 次 / 用户 / 小时 |
| 订阅链接拉取 | 60 次 / token / 小时 |
| 其它读接口 | 600 次 / 用户 / 分钟 |

---

## 2. 认证模块（user-svc）

### 2.1 注册
`POST /api/v1/auth/register`
```json
// Request
{
  "email": "user@example.com",
  "password": "Str0ng@Pwd",
  "email_code": "123456",
  "invite_code": "ABCD12"
}
// Response
{ "code": 0, "data": { "user_id": 1024, "uuid": "9b1d…" } }
```

### 2.2 发送邮箱验证码
`POST /api/v1/auth/email-code`
```json
{ "email": "user@example.com", "scene": "register|reset_password" }
```

### 2.3 登录
`POST /api/v1/auth/login`
```json
{ "email": "...", "password": "...", "totp_code": "123456" }
```
```json
{
  "code": 0,
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "rt_...",
    "expires_in": 7200,
    "user": { "id": 1024, "email": "...", "role": "user" }
  }
}
```

### 2.4 刷新 Token
`POST /api/v1/auth/refresh`  `{ "refresh_token": "rt_..." }`

### 2.5 登出
`POST /api/v1/auth/logout`（将当前 access + refresh 加入 Redis 黑名单）

### 2.6 双因子认证
- `POST /api/v1/auth/2fa/setup` → 返回 `secret` 与 `qrcode_uri`
- `POST /api/v1/auth/2fa/enable`  `{ "code": "123456" }`
- `POST /api/v1/auth/2fa/disable` `{ "code": "123456" }`

---

## 3. 用户模块

### 3.1 获取个人信息
`GET /api/v1/user/me`
```json
{
  "code": 0,
  "data": {
    "id": 1024,
    "email": "user@example.com",
    "uuid": "9b1d...",
    "balance_cny": "12.50",
    "plan": {
      "id": 3, "name": "标准版-月付",
      "expire_at": "2026-06-25T00:00:00Z",
      "traffic_total_gb": 200, "traffic_used_gb": 37.42,
      "device_limit": 5
    },
    "invite_code": "ABCD12",
    "invited_count": 4,
    "totp_enabled": true,
    "subscription_token": "sub_8f7a..."
  }
}
```

### 3.2 修改密码
`PUT /api/v1/user/password`  `{ "old_password": "...", "new_password": "..." }`

### 3.3 重置 UUID
`POST /api/v1/user/uuid/rotate`（重置后所有客户端需重新订阅）

### 3.4 重置订阅 Token
`POST /api/v1/user/subscription/rotate`

### 3.5 在线设备
- `GET /api/v1/user/devices` 当前连接设备列表
- `DELETE /api/v1/user/devices/:id` 踢下线

### 3.6 流量明细
`GET /api/v1/user/traffic?start=2026-05-01&end=2026-05-26`
```json
{
  "data": {
    "summary": { "upload_gb": 12.4, "download_gb": 88.5 },
    "daily": [{ "date": "2026-05-25", "upload_gb": 0.8, "download_gb": 5.2 }],
    "by_node": [{ "node_id": 12, "name": "JP-Tokyo-01", "total_gb": 30.1 }]
  }
}
```

---

## 4. 套餐与流量包

### 4.1 套餐列表（公开）
`GET /api/v1/plans`
```json
{
  "data": [{
    "id": 3, "name": "标准版-月付",
    "price_cny": "29.00", "duration_days": 31,
    "traffic_gb": 200, "device_limit": 5, "speed_limit_mbps": 0,
    "node_groups": ["basic", "premium"], "tags": ["热销"]
  }]
}
```

### 4.2 流量包列表
`GET /api/v1/data-packs`

### 4.3 优惠券校验
`POST /api/v1/coupons/validate`  `{ "code": "SUMMER20", "plan_id": 3 }`
→ `{ "discount_cny": "5.80", "final_price": "23.20" }`

---

## 5. 订单与支付（order-svc + billing-svc）

### 5.1 创建订单
`POST /api/v1/orders`
```json
{
  "type": "plan",
  "target_id": 3,
  "coupon_code": "SUMMER20",
  "pay_method": "alipay"
}
```
```json
{
  "code": 0,
  "data": {
    "order_no": "20260526112233ABCD",
    "amount_cny": "23.20",
    "status": "pending",
    "expire_at": "2026-05-26T03:25:00Z",
    "pay_info": {
      "method": "alipay",
      "qr_code": "https://qr.alipay.com/...",
      "pay_url": "alipays://..."
    }
  }
}
```
USDT 订单 `pay_info` 示例：
```json
{
  "method": "usdt_trc20",
  "chain": "TRON",
  "address": "TXyz...",
  "amount_usdt": "3.21",
  "rate_cny_usdt": "7.22"
}
```

### 5.2 查询订单
`GET /api/v1/orders/:order_no`

### 5.3 我的订单列表
`GET /api/v1/orders?status=paid&page=1`

### 5.4 取消订单
`POST /api/v1/orders/:order_no/cancel`

### 5.5 余额充值
`POST /api/v1/balance/topup` → 同 5.1，`type=topup`

### 5.6 支付回调（服务端到服务端）
- 支付宝：`POST /callback/alipay`（RSA2 验签）
- 微信：`POST /callback/wechat`（v3 验签）
- USDT：内部 watcher 监听链上事件，不走 HTTP

### 5.7 邀请返佣
- `GET /api/v1/invite/stats`
- `GET /api/v1/invite/records`
- `POST /api/v1/invite/withdraw`  `{ "amount": "50.00", "method": "balance|usdt", "address": "..." }`

---

## 6. 节点与订阅（node-svc + subscription-svc）

### 6.1 节点列表（用户视角）
`GET /api/v1/nodes`
```json
{
  "data": [{
    "id": 12, "name": "🇯🇵 日本-东京-01",
    "region": "JP", "group": "premium",
    "protocol": "vless-reality",
    "rate_multiplier": 1.0,
    "online": true, "latency_ms": 45
  }]
}
```

### 6.2 获取订阅
`GET /sub/:token?type=clash|singbox|surge|v2rayn|shadowrocket|loon|stash&flag=meta`

响应 Header：
```
Content-Type: text/yaml; charset=utf-8
Subscription-Userinfo: upload=12345; download=67890; total=214748364800; expire=1782345600
Profile-Update-Interval: 24
```

### 6.3 节点测速上报
`POST /api/v1/nodes/:id/latency`  `{ "latency_ms": 42 }`

---

## 7. 工单系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/tickets` | 我的工单列表 |
| POST | `/api/v1/tickets` | 新建 `{ subject, category, content }` |
| GET | `/api/v1/tickets/:id` | 详情 + 回复列表 |
| POST | `/api/v1/tickets/:id/replies` | 追加回复 |
| POST | `/api/v1/tickets/:id/close` | 关闭 |

---

## 8. 公告与系统

- `GET /api/v1/notices` 公告列表
- `GET /api/v1/config/public` 公开配置（注册开关、支付开关、客户端下载、教程）
- `GET /healthz` 健康检查（不鉴权）
- `GET /metrics` Prometheus 指标（仅内网）

---

## 9. 管理后台 API（admin-svc，前缀 `/admin`）

> 角色：`admin | ops | finance`，必须开启 2FA。

### 9.1 用户管理
- `GET /admin/users?keyword=&status=`
- `GET /admin/users/:id`
- `PUT /admin/users/:id`（套餐、流量、状态、余额）
- `POST /admin/users/:id/reset-password`
- `POST /admin/users/:id/ban` / `/unban`

### 9.2 套餐 / 流量包 / 优惠券
标准 CRUD：`/admin/plans`、`/admin/data-packs`、`/admin/coupons`

### 9.3 订单与财务
- `GET /admin/orders?status=&pay_method=&start=&end=`
- `POST /admin/orders/:order_no/refund`  `{ "reason": "..." }`
- `GET /admin/finance/daily?start=&end=`
- `GET /admin/finance/export?start=&end=`

### 9.4 节点管理
- `GET /admin/nodes`
- `POST /admin/nodes`
- `PUT /admin/nodes/:id`
- `DELETE /admin/nodes/:id`
- `POST /admin/nodes/:id/reload` 触发 Xray 热重载
- `GET /admin/nodes/:id/stats` 流量/在线人数

### 9.5 节点分组
`/admin/node-groups` CRUD（控制套餐对节点的访问权限）

### 9.6 工单
`/admin/tickets` 列表 / 回复 / 关闭

### 9.7 系统设置
- `GET/PUT /admin/settings/site`
- `GET/PUT /admin/settings/payment`（支付宝、微信、USDT 钱包）
- `GET/PUT /admin/settings/email`
- `GET/PUT /admin/settings/security`

### 9.8 运维监控
- `GET /admin/metrics/overview` 在线人数 / 带宽 / 今日收入
- `GET /admin/logs/admin-audit` 后台操作审计

---

## 10. 内部 gRPC 接口（node-agent ↔ control-plane）

`api/proto/node_agent.proto`

```protobuf
syntax = "proto3";
package proxy_vpn.node.v1;

service NodeAgentService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc ReportTraffic(ReportTrafficRequest) returns (ReportTrafficResponse);
  rpc WatchConfig(WatchConfigRequest) returns (stream ConfigEvent);
}

message HeartbeatRequest {
  string node_id = 1;
  uint64 online_users = 2;
  double cpu_percent = 3;
  double mem_percent = 4;
  uint64 bandwidth_in_bps = 5;
  uint64 bandwidth_out_bps = 6;
  int64  timestamp = 7;
}

message TrafficItem {
  string user_uuid = 1;
  uint64 upload_bytes = 2;
  uint64 download_bytes = 3;
}

message ReportTrafficRequest {
  string node_id = 1;
  repeated TrafficItem items = 2;
  int64 window_start = 3;
  int64 window_end = 4;
}
```

鉴权：mTLS（节点客户端证书）+ Metadata 头 `node_token`。

---

## 11. Webhook（对外通知）

供企业版用户 / 管理员订阅事件：
- 事件：`user.created`、`order.paid`、`order.refunded`、`traffic.exceeded`、`node.offline`
- 推送格式：
```http
POST <user_webhook_url>
X-Signature: hmac-sha256=...

{ "event": "order.paid", "data": { ... }, "timestamp": 1782345600 }
```

---

## 12. 接口版本与演进

- URL 前缀版本化：`/api/v1`、`/api/v2`
- 废弃接口响应头：`Deprecation: true` + `Sunset: <date>`
- 字段新增向后兼容；字段移除需走 `v2`

---

**变更日志**
- v0.1 (2026-05-26)：初版 API 设计，覆盖用户/订单/支付/节点/订阅/管理 6 大模块。
