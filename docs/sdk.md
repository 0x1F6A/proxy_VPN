# 客户端 SDK 与对接指引（Phase 14-B）

> 目标读者：第三方集成方、自家 Web/Mobile 客户端开发、CI 自动化脚本

## 1. 总览

```
┌────────────────────────────────────────┐
│ proxy_VPN API (HTTPS + JSON envelope)  │
└────────────┬───────────────────────────┘
             │
   ┌─────────┼─────────┬──────────────────┐
   ▼         ▼         ▼                  ▼
sdk/go    sdk/cli   Mobile binding     Web frontend
(Go lib) (CLI ref)  (按本文协议手写)    (axios + types)
```

本仓内置：
- **`sdk/go/proxyvpn/`**：官方 Go SDK，类型化、自动 token 刷新、错误码常量
- **`sdk/cli/proxyvpnctl/`**：基于 Go SDK 的参考 CLI（cobra），覆盖完整购买/订阅流程
- **本文**：未提供原生 SDK 的语言（iOS/Android/Web/Dart）按协议手工对接的契约

## 2. Go SDK 用法

参考 `sdk/go/README.md`，5 行可跑：

```go
c := proxyvpn.New(proxyvpn.Config{BaseURL: "https://api.example.com"})
_, _ = c.LoginPassword(ctx, email, password, "")
me, _ := c.Me(ctx)
```

特性：
- 401 自动用 refresh_token 换一次新 access_token 后重放原请求
- 多 goroutine 并发触发 refresh 时通过 `tokenVersion` double-check 只发一次
- `errors.Is(err, proxyvpn.ErrInsufficientBalance)` 走业务码分支

## 3. CLI 用法

```bash
# 编译
go build -o proxyvpnctl ./sdk/cli/proxyvpnctl

# 凭据落盘到 ~/.proxyvpn/credentials.json（0600）
PROXYVPN_BASE_URL=https://api.example.com ./proxyvpnctl login --email a@b.com --password ***

./proxyvpnctl me               # 当前账号
./proxyvpnctl plans            # 套餐列表
./proxyvpnctl buy --plan 1     # 下单
./proxyvpnctl pay <no> --channel alipay --watch  # 付款并轮询状态
./proxyvpnctl orders --status paid
./proxyvpnctl sub --format clash -o clash.yaml   # 拉订阅
./proxyvpnctl nodes
./proxyvpnctl logout
```

环境变量：
- `PROXYVPN_BASE_URL`（默认 `http://localhost:8082`）
- `PROXYVPN_CRED_FILE`（默认 `$HOME/.proxyvpn/credentials.json`）

## 4. 非 Go 客户端对接协议

### 4.1 统一响应壳

所有 `Content-Type: application/json` 的接口（订阅端点除外）必须按下列结构反序列化：

```json
{ "code": 0, "message": "ok", "data": {...|[...]|null}, "request_id": "req_..." }
```

- `code == 0` 视为成功，`data` 字段才是业务载荷
- `code != 0` 时取 `message` 展示，按 `code` 决定 UX 分支（见错误码表）
- 始终记录 `request_id`（同时回写到响应头 `X-Request-Id`），上报错误时附带

### 4.2 鉴权头

```
Authorization: Bearer <access_token>
```

### 4.3 Token 刷新协议（关键）

客户端必须实现：

1. 收到任意请求返回 `code=40101`（HTTP 200/401 皆有可能）时：
2. 调 `POST /api/v1/auth/refresh { "refresh_token": "..." }`
3. 成功 → 用新 `access_token` 重放原请求一次
4. 失败 → 视为登出，回到登录页
5. 并发请求同时触发刷新时，必须只发一次刷新请求（mutex / single-flight）

错误的实现会导致：refresh_token 被一次性 burn（服务端 rotate）→ 多个请求互相 invalidate → 用户被迫重登。

### 4.4 幂等请求

POST 类下单 / 支付接口必须携带：

```
Idempotency-Key: <uuid v4>
```

服务端用此 key 在 24h 内做幂等去重。

### 4.5 客户端标识

```
X-Client: web|ios|android|cli
X-Client-Version: 1.0.0
```

便于服务端按客户端类型做限流 / 灰度 / 强制升级。

### 4.6 订阅端点

`GET /sub/:token?format=clash|sing-box|v2ray` 不走 envelope，直接返回：
- `clash` → `application/yaml`
- `sing-box` → `application/json`
- `v2ray` → `text/plain`（base64 节点列表）

`:token` 即 `User.subscribe_token`，需视为 secret（与密码同级）。

## 5. 错误码映射（与 `docs/api.md` 一致）

| code | 含义 | 建议 UX |
|------|------|--------|
| 40001 | 参数错误 | toast 错误文案，停留在表单 |
| 40101 | access 过期 | 触发 refresh 流程 |
| 40102 | refresh 无效 | 强制重新登录 |
| 40301 | 权限不足 | 隐藏/禁用入口 |
| 40302 | 账号被禁用 | 回到登录页并提示联系客服 |
| 40401 | 资源不存在 | 404 占位页 |
| 40901 | 资源冲突 | 提示已存在 / 重复下单 |
| 42901 | 限流 | 退避重试，UI 显示「请稍后」 |
| 50001 | 服务端错误 | 上报 + 通用错误页 |
| 50301 | 节点不可用 | 在订阅/节点页标灰节点 |
| 60001 | 余额不足 | 跳转充值 / 修改支付方式 |
| 60002 | 套餐已过期 | 跳转续费 |
| 60003 | 流量已耗尽 | 跳转购买流量包 |
| 60101 | 支付订单超时 | 提示重新下单 |
| 60102 | 支付金额不匹配 | 联系客服（USDT 常见） |

## 6. 未来路线

- TypeScript SDK（基于 fetch + zod schema 校验）
- Dart SDK（Flutter 客户端）
- iOS / Android 原生 SDK（暂建议直接基于本文档手写 thin client）
- gRPC 内部 API 文档（仅内部服务消费）

## 7. CHANGELOG 同步

每次 API 变动（路径 / 字段增删 / 错误码新增）必须：

1. `docs/api.md` 更新
2. `sdk/go/proxyvpn/` 同步类型与方法
3. `CHANGELOG.md [Unreleased]` 写明 BREAKING（如果是）
4. 非破坏性新增不需要 bump 主仓 major
