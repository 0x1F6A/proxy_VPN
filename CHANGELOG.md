# Changelog

All notable changes to this project will be documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased] - Phase 6: Traffic accounting & ban list

### Added
- New bounded context `internal/traffic` with domain / ports / service / infra layers (gormrepo / redisban / chsink).
- ClickHouse sink scaffolding (`internal/traffic/infra/chsink`) with `Driver` interface so MySQL-only environments still work via `usage_event_fallback`.
- Migration `000003_traffic` — `users` gets `rate_bps_up/down`, `banned`; new `traffic_daily` and `usage_event_fallback` tables.
- HTTP endpoints: `POST /api/v1/nodes/usage`, `GET /api/v1/nodes/banlist` (bootstrap-secret auth), `GET /api/v1/me/usage`, `GET /api/v1/me/usage/daily` (JWT auth).
- Asynq tasks: `traffic:flush_ch_buffer` (15s), `traffic:recompute_bans` (1m), `traffic:rollup_daily` (01:30 daily).
- node-agent: periodic usage reports + ban-list polling (gated by `--node-id`).
- Config: `clickhouse.*` and `traffic.*` sections with sane defaults.

### Changed
- `cmd/api` and `cmd/worker` now construct & wire the traffic service.
- `internal/pkg/asynqx/tasks` registers the three new traffic task handlers.


## [Unreleased]

### Added — Phase 5 (真实支付 + Asynq 异步调度)
- 全新 `internal/payment` 限界上下文：`domain/ports/service/infra/gormrepo/transport/httpapi` + provider 子包
  （`alipay` 当面付 PreCreate、`wechat` Native v3、`usdt`-TRC20 链上扫描、`mockprov` HMAC 测试桩）。
- 数据库迁移 `000002_payment.up.sql`：`payments` / `payment_addresses` / `chain_scan_cursor`
  三张新表 + 修复 `orders` 幂等键（`uk_orders_idem` → `uk_orders_user_idem(user_id, idempotency_key)`）。
- 跨上下文回调：`payment.ports.OrderPaidNotifier` 由 billing service 直接实现
  （`GetOrderAmount` + 复用 `HandlePaid`），保持依赖方向 payment ← billing。
- 路由：
  - `POST /api/v1/orders/:no/pay`     —— 选择支付通道并返回 QR / 地址
  - `GET  /api/v1/payments/:id`       —— 轮询支付状态
  - `POST /pay/notify/:channel`       —— 根路径下的通道 webhook（短 URL）
- 装配策略：mockprov 始终注册；alipay/wechat/usdt 仅在凭证齐全时挂载，缺失通道返回 `ErrChannelUnsupported`；
  `payment.mode=mock` 模式下把 mock 注册到 alipay/wechat 槽位，方便本地联调。
- 异步任务框架 `internal/pkg/asynqx`（+ `tasks` 子包）：基于 hibiken/asynq 封装
  Client / Server / Scheduler；5 个 task type：`billing:auto_cancel_orders`、`node:mark_stale`、
  `payment:expire_pending`、`payment:reconcile_channel`、`payment:scan_usdt_block`。
- 新二进制 `cmd/worker`：消费 asynq 队列并运行 Scheduler 周期任务（1m 取消订单、30s 节点心跳过期、
  1m 支付过期、5m 对账 alipay/wechat、15s USDT 扫块）；与 `cmd/api` 共享同一 Redis。
- 配置追加 `PaymentConfig`（`mode/notify_base/return_base/mock_secret` + alipay/wechat/usdt 子段）
  和 `AsynqConfig.Concurrency`；新增 setDefaults。
- 单元测试：`payment/service`（CreatePayment 复用、HandleNotify 幂等、AmountMismatch、签名拒绝、ExpireOldPending）
  + `mockprov` 签名往返 + `usdtprov.Scanner.Step`（fake TronClient + cursor 推进）。

### Notes
- `cmd/api` 保留 `RunAutoCancelLoop` / `RunStaleMarker` 两个内嵌 goroutine 作为单机部署的回退；
  与 `cmd/worker` 同时运行也安全（操作均幂等）。
- USDT 汇率字段 `payment.usdt.cny_per_usdt` 当前手工配置（默认 7.30），后续可接 oracle。
- Wechat provider 启动时会调用 `RegisterDownloaderWithPrivateKey` 注册证书下载器，凭证错误时返回 error；
  此时该通道不挂载，但不会让 api 启动失败。


### Added — Phase 4 (节点与协议：Xray / Sing-box / Hysteria2 + 订阅)
- 六边形分层 `internal/node/{domain,ports,service,service/subgen,infra/gormrepo,transport/httpapi}`。
- 节点协议支持：`vless-reality` / `trojan` / `hysteria2` / `ss-2022`（受 `tls_config` + `transport_config` JSON 驱动，pass-through）。
- 节点 Agent 接入：`POST /api/v1/node-agent/register`（bootstrap secret + 单节点 token）、`POST /api/v1/node-agent/heartbeat`（CPU/MEM/带宽/在线数）；30s 后台 worker `RunStaleMarker` 把超过 `node.heartbeat_timeout` 的节点置 offline。
- 管理员路由：`/api/v1/admin/nodes`（POST 一次性吐出 bootstrap token）、`/api/v1/admin/node-groups` CRUD，统一走 `RequireRole("admin")`。
- 用户路由：`GET /api/v1/nodes` 仅返回当前用户套餐 `plan_node_groups` 授权 + serviceable 的节点（隐藏 token / TLS 细节）。
- 公共订阅 `GET /sub/:token?format=v2ray|clash|sing-box`：
  - v2ray：share-link 列表 base64（vless-reality/trojan/hysteria2/ss-2022）；
  - Clash Meta YAML：proxies + PROXY selector group；
  - Sing-box 1.8 JSON：outbounds + selector + final route。
  - 响应带 `Subscription-Userinfo` / `Profile-Update-Interval` 头部，兼容 Clash Verge / Shadowrocket。
- `cmd/node-agent`：注册 → 30s 心跳循环；从 flag 或环境变量读取 bootstrap / token。
- 配置追加 `node` 段：`bootstrap_secret`、`heartbeat_timeout`、`subscription_base`。
- `internal/user/infra/gormrepo/SubscriberLookupRepo` 实现 `node/ports.SubscriberPort`，反向依赖避免 node 直接耦合 user。
- 单测：3 种订阅格式渲染、AgentRegister（bootstrap/token 校验）、Heartbeat → MarkStale、Subscription 各种边界（无 plan / 过期 / 错 format / 错 token）。

### Added — Phase 3 (套餐、流量包、订单与 Mock 支付)
- 六边形分层 `internal/billing/{domain,ports,service,infra/gormrepo,transport/httpapi}`。
- 套餐 / 流量包目录：公开浏览 `GET /api/v1/plans|/data-packs`，管理员 CRUD `POST|PUT|DELETE /api/v1/admin/plans|/admin/data-packs`。
- 优惠券预估 `POST /api/v1/coupons/quote`：支持 fixed / percent 折扣、scope（plan|pack|all）、最低金额、总配额与人均次数。
- 订单全生命周期：创建（含 `Idempotency-Key` 幂等）、查询（单笔 / 我的列表）、取消、Mock 支付 `POST /api/v1/orders/:no/mock-pay`。
- 支付成功后通过 `UserBillingPort`（user 模块实现）将套餐 / 流量包 / 充值原子应用到用户：plan_id / plan_expire_at / traffic_total / balance_cny。
- 后台 worker `RunAutoCancelLoop`：每分钟将超过 15 分钟未支付的订单批量置为 expired。
- 金额一律使用 `big.Rat` 处理（避免浮点漂移），CNY 以字符串形式贯穿 API 与持久层 `DECIMAL(12,2)`。
- 管理员路由通过 `RequireRole("admin")` 中间件保护，复用 user 模块的 JWT AuthRequired。
- 单测：QuoteCoupon（fixed / percent / 配额耗尽 / 最低金额未达）、CreateOrder 并发幂等、MockPay 触发 ApplyPlan、AutoCancel worker。

### Added — Phase 2 (用户体系)
- 六边形分层 `internal/user/{domain,ports,service,infra/{gormrepo,rediskv,smtpmail},transport/httpapi}`。
- `internal/pkg/auth`：bcrypt 密码哈希、HS256 JWT 签发/解析、TOTP（pquerna/otp）、SHA256/RandomToken 工具。
- `internal/pkg/idgen`：UUID / InviteCode / SubscriptionToken。
- 用例：注册、登录（含 2FA）、Refresh 轮换、Logout（撤销 access+refresh）、修改密码、Me、2FA enroll/verify/disable。
- HTTP 路由 `/api/v1/auth/*` + `/api/v1/user/*`，带 AuthRequired 中间件（Bearer + JWT 校验 + Redis 黑名单）。
- 配置追加 `smtp` / `rate` 段；dev 默认对接 MailHog 1025。
- 失败登录 / 验证码节流（Redis 固定窗口计数器）。
- 单测覆盖 auth 包以及 user/service 全部主路径（fake repos / blacklist / limiter）。

### Added — Phase 1 (基础设施 / 依赖装配)
- `deploy/docker-compose.dev.yml`：MySQL 8 / Redis 7 / MailHog / Prometheus / Grafana / Loki 一键本地起栈。
- `deploy/prometheus/prometheus.yml`、`deploy/grafana/provisioning/datasources/datasources.yml`。
- `internal/migrations/000001_init_schema.{up,down}.sql` + 迁移规范 README（覆盖用户/订单/支付/节点/工单等全部一期业务表）。
- `internal/pkg/storage`：GORM (MySQL) + go-redis 客户端封装，含 `Ping` / `Close`。
- `/readyz` 真实健康检查：API 启动时探测 MySQL / Redis，运行期按依赖逐项返回 ok / fail。
- `docs/architecture.md`：13 章详细系统架构（分层、六边形、数据流、节点、支付、部署拓扑、安全、容量）。

### Added — Phase 0 (project skeleton)
- Go module skeleton: `cmd/api` HTTP server with graceful shutdown, version stamping.
- `internal/pkg/config` (Viper + env override), `logger` (slog), `httpx` (Gin + healthz/readyz/metrics).
- Makefile with `tidy / build / run-api / test / cover / lint / ci / docker` targets.
- golangci-lint configuration.
- GitHub Actions: CI (vet + lint + test + build), security (gosec + trivy), govulncheck.
- Distroless Dockerfile.
- PR template, Issue templates, CODEOWNERS, EditorConfig.
- Example configuration `config.example.yaml`.
- README quick-start.

### Documentation
- 整体服务框架.md — 系统总体架构与模块拆分。
- docs/api.md — 对外 / 管理 / 内部 gRPC API 设计 v0.1。
- docs/开发顺序与稳定性检查.md — 10 阶段交付路线与稳定性 checklist。

## [Unreleased] - Phase 7: Node config render + reload loop

### Added
- `internal/node/service/nodecfg`: deterministic xray server-side config renderer (VLESS-Reality / Trojan / Hysteria2 / SS-2022) with stable version hash; per-user `email=u<id>` so xray Stats API queries are trivial.
- `POST /api/v1/node-agent/config` endpoint (node_token auth, supports `known_hash` short-circuit).
- node-agent `configloop.go`: periodic config pull, atomic write to `--config-out`, optional `--reload-cmd` exec.
- `SubscriberPort.ListActive` (excludes banned + expired plans) with gormrepo impl.

## [Unreleased] - Phase 8: Admin user management + RBAC

### Added
- `ports.AdminUserRepo` + `gormrepo.AdminUserRepo`: list/search users (paged), set-banned, adjust traffic (signed delta with floor 0), set per-user rate limits, overall counts.
- `user/service` admin methods: `AdminListUsers`, `AdminSetBanned`, `AdminAdjustTraffic`, `AdminSetRateLimits`, `AdminOverallCounts`.
- HTTP admin routes (admin/ops role):
  - `GET  /api/v1/admin/users` (q, limit, offset)
  - `GET  /api/v1/admin/users/summary`
  - `POST /api/v1/admin/users/:id/ban` / `unban`
  - `POST /api/v1/admin/users/:id/traffic` (`delta_bytes`)
  - `POST /api/v1/admin/users/:id/rate`   (`up_bps`, `down_bps`)
- Reusable `requireRole(...)` middleware in user/transport/httpapi.

## [Unreleased] - Phase 9: Admin reports + ClickHouse rollup MV

### Added
- New `report` bounded context (ports/service/infra/transport):
  - `GET /api/v1/admin/reports/revenue?from=YYYY-MM-DD&to=YYYY-MM-DD`
  - `GET /api/v1/admin/reports/traffic?from&to`
  - `GET /api/v1/admin/reports/orders?from&to`
  - `GET /api/v1/admin/dashboard` (KPI snapshot: users/orders/revenue/traffic today)
- Range validation: max 366 days; from must precede to.
- Admin/ops/finance role required (reuses JWT claims).

### Changed
- `chsink.Bootstrap` now also creates `traffic_user_daily` SummingMergeTree + `mv_traffic_user_daily` materialised view, so per-user daily totals stream automatically from the raw events table.

## [Unreleased] - Phase 10: Observability + blue/green ops

### Added
- `httpx.Metrics()` Gin middleware: `http_requests_total{method,route,status}` counter, `http_request_duration_seconds{method,route}` histogram (5ms..20s exp×2 buckets), `http_requests_in_flight` gauge. Uses `c.FullPath()` for low-cardinality route labels.
- `docs/ops.md`: Prometheus scrape targets, recommended Grafana panels, starter alerting rules, blue-green & canary deployment runbook, pre-flight checklist & rollback procedure.
- `internal/pkg/httpx/metrics_test.go`: integration test asserting `/metrics` exposes counters for served routes.

Note: `/healthz`, `/readyz`, `/metrics` endpoints and MySQL/Redis readiness checks were already in place; this phase adds per-request metrics and the operations runbook on top.
