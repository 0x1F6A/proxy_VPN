# Changelog

All notable changes to this project will be documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added — Phase 15-A: 风控反滥用
- `internal/pkg/geoip`：封装离线 GeoLite2 mmdb；找不到文件时降级为 `NoopLookup`，业务路径零阻断。
- `internal/risk`：全新 bounded context。`service.Service` 暴露 `PreLogin` / `RegisterLoginFailure` / `RegisterLoginSuccess` / `RotateSubscriptionToken` / `SubGuard` / `ListDevices` / `RevokeDevice`，所有 deps 可 nil 降级；`Fingerprint = sha256(ua + accept-language + ip/24)` 让同 NAT 反复登录不算新设备；失败计数与锁定走 Redis 两个 key (`risk:fail:*` / `risk:lock:*`)，达到 `LoginLockThreshold` 后锁 `LoginLockDuration`；订阅 IP 治理用 ZSET 滚动窗口，超 `SubRevokeThreshold` 自动 rotate token + 邮件告警。
- `internal/risk/infra`：gormrepo（device upsert + user lookup over `users` 表）+ rediskv（lockout / sub-IP）+ smtp mailer（subject 走 i18n bundle）。
- `internal/risk/transport/httpapi`：admin `GET /api/v1/admin/users/:id/devices` / `DELETE /api/v1/admin/users/:id/devices/:fp` / `POST /api/v1/admin/users/:id/subscribe-token/rotate`；user 自助 `POST /api/v1/user/subscribe-token/rotate`。
- `internal/user/service`: `Login` 签名追加可选 `acceptLang`；新增 `PreLogin/RegisterLoginFailure/RegisterLoginSuccess` 三个注入点；`Service.SetRisk` 允许 cmd 后置注入 risk 钩子。
- `internal/migrations/000007_risk`：`login_devices`（user_id + fp_hash + first/last_seen + ip + ua + country）+ `users.subscribe_token_rotated_at` + `users.last_login_country` + `orders.requires_manual_review` + `orders.risk_score`。
- `internal/pkg/config`: `RiskConfig{Enabled,LoginLockThreshold,LoginLockDuration,SubMaxIPs,SubRevokeThreshold,SubWindow,GeoIPDBPath}` + 默认值。
- `cmd/api` / `cmd/admin`: 启用 Risk 时构造完整依赖图，挂 risk 路由，并把 risk 注入 user service 让 Login 走风控。
- 测试：`internal/risk/service` 9 case + `internal/pkg/geoip` 3 case + `internal/pkg/i18n` 4 case 全绿。

### Added — Phase 15-C: 国际化 + 工单系统
- `internal/pkg/i18n`：自实现轻量 bundle，`embed messages/{en,zh-CN,zh-TW,ja}.toml`；`MatchLocale(Accept-Language)` 先全等再 lang 前缀回退；gin `Middleware` 把命中的 locale 写入 ctx。
- `internal/pkg/i18n/messages/*.toml`：4 国语言基础消息（`err.*` / `email.*` / `ticket.*`）。
- `internal/ticket`：全新 bounded context（domain/ports/service/infra-gormrepo/transport-httpapi）。状态机 `open → pending → resolved → closed`，admin 回复→pending、用户对 resolved 回复→重开 open。
- `internal/ticket/transport/httpapi`：用户 `POST/GET /api/v1/tickets` + `GET /api/v1/tickets/:id` + `POST /:id/messages` + `POST /:id/close`；admin `GET /api/v1/admin/tickets`（status/priority/assignee/keyword 过滤+分页）+ `POST /:id/assign` + `POST /:id/reply` + `PATCH /:id`。
- `internal/migrations/000008_tickets`：`tickets`（含 assignee_id + priority + status idx）+ `ticket_messages`（attachments JSON）+ `users.locale VARCHAR(16)`。
- `internal/pkg/config`: `I18nConfig{DefaultLocale,SupportedLocales}` + 默认值。
- `cmd/api` / `cmd/admin`: router 级挂 i18n middleware；构造 ticket service + 路由。

### Added — Phase 14-B: 客户端 SDK + 参考 CLI
- `sdk/go/proxyvpn/`：独立 Go module（`github.com/0x1F6A/proxy_VPN/sdk/go`），零非 stdlib 依赖。类型化 client 覆盖 auth / user / billing / payment / subscription / nodes 全部公开端点；统一响应壳解析（`{code,message,data,request_id}`）；401 自动用 refresh_token 换新并重放一次，并发触发时通过 `tokenVersion` double-check 只发一次刷新；错误码常量支持 `errors.Is(err, proxyvpn.ErrInsufficientBalance)`。
- `sdk/cli/proxyvpnctl/`：独立 module + cobra CLI 参考实现。子命令：`login` / `logout` / `me` / `plans` / `buy` / `pay --watch` / `orders` / `sub` / `nodes`。凭据持久化到 `~/.proxyvpn/credentials.json`（0600），通过 `PROXYVPN_BASE_URL` / `PROXYVPN_CRED_FILE` 环境变量配置。
- `go.work`：把根 module + `sdk/go` + `sdk/cli` 串成 workspace，本地开发不需要 replace 跳转。
- `sdk/go/README.md`：5 行 hello world + 错误处理示例。
- `docs/sdk.md`：SDK 概览、CLI 用法、非 Go 客户端对接协议（统一响应壳 / token 刷新 / 幂等 / 客户端标识 / 订阅端点）、错误码 UX 映射表、未来路线。
- 测试：`sdk/go/proxyvpn/client_test.go` 用 `httptest` 覆盖 login + Me、401 自动刷新（验证只发一次 refresh + token rotate）、错误码映射（`errors.Is`）、subscription 原文透传、ListPlans 解析。

### Added — Phase 14-C: 企业级 SSO + SLA 自探活报表
- `internal/pkg/config`: 新增 `OIDCConfig`（issuer/client_id/secret/redirect_url/scopes/allowed_domains/allowed_emails/admin_emails/state_ttl）+ `SLAConfig`（enabled/region/probe_interval/timeout/targets），全部带零值默认，老配置零破坏。
- `internal/user`: domain.User 增加 `OIDCSubject`；ports.UserRepo 增加 `FindByOIDCSubject` / `LinkOIDCSubject`；service 新增 `oidc.go`（`BeginAuth` + `CompleteAuth` + `findOrLinkOIDCUser` 三路：subject 命中 / email 命中后挂 subject / 全新自动注册），强制 `email_verified=true`，按 `AdminEmails` 映射 admin 角色，命中白名单/域名才放行（`ErrForbidden`）。
- `internal/user/infra/oidcprov`: 基于 `github.com/coreos/go-oidc/v3` + `golang.org/x/oauth2` 的 verifier 实现，封装 `Exchange` + `AuthCodeURL`。
- `internal/user/infra/oidcstate`: Redis state 存储，写入时 TTL，回调时 `GetDel` 单次原子消费防重放。
- `internal/user/transport/httpapi/oidc.go`: `GET /api/v1/auth/oidc/login?next=` 302 跳 IDP；`GET /api/v1/auth/oidc/callback` 默认返回 JSON token，若 `next` 带 `text/html` 则 302 回 `next#access_token=...&refresh_token=...`（SPA popup 与重定向两种 UX 均覆盖）。
- `internal/migrations/000005_user_oidc.up.sql`: users 表加 `oidc_subject VARCHAR(191) NULL UNIQUE`，允许已有 email/password 用户与 IDP 双向并存。
- `internal/sla`: 全新模块（domain/ports/service/infra/transport）。`Service` 暴露 `Record`（asynq prober 调用）/ `RollupDay`（按 region+target 分组，sort+索引法算 p50/p95/p99）/ `Summary`（按 target 聚合 uptime% + p95），日表幂等 upsert 用原生 `ON DUPLICATE KEY UPDATE`。
- `internal/sla/infra/prober`: HTTP `GET` 探活，2xx-3xx 视为 success，记录 `latency_ms` + 错误信息（255 字符截断）。
- `internal/sla/transport/httpapi`: `GET /api/v1/admin/reports/sla?from=YYYY-MM-DD&to=YYYY-MM-DD[&target=]`，复用 admin 鉴权。
- `internal/migrations/000006_sla.up.sql`: `sla_probes`（明细，按 ts 索引）+ `sla_daily`（聚合，`UNIQUE(day,region,target)`）。
- `internal/pkg/asynqx/tasks`: 新增 `TypeSLAProbe` + `TypeSLARollupDaily` + 构造器 + Deps 字段 + 任务处理器。
- `cmd/admin`: OIDC 启用时初始化 verifier + state store 并注入 user handler；reportH 之后挂 SLA report 路由（复用 `AuthRequired` + admin roleOf）。
- `cmd/worker`: 构造 sla service + prober，按 `cfg.SLA.ProbeInterval`（默认 1m）跑 `SLAProbe`，每天 00:05 跑 `SLARollupDaily` 滚动前一天。
- `docs/architecture.md`: 新增 §11.2 SLA 数据流（probe→daily→report）+ §12.1 OIDC SSO 流程（authorize→callback→token→自动注册/链接）。
- `docs/ops.md`: 新增「OIDC 配置示例」（Google / GitHub / Okta）+「SLA 大盘解读」章节。
- 测试：`internal/user/service/oidc_test.go`（白名单/admin 映射/state 缺失/email 未验证 4 个场景）；`internal/sla/service/service_test.go`（percentiles 边界、RollupDay 幂等、Record 必填校验、Summary 区间校验）。

### Added — Phase 14-A: 多控制面跨区灾备
- `internal/pkg/config`: `MySQLConfig` 新增 `ReadReplicas []string` + `ResolverPolicy`（`random` 默认 / `round_robin`）；`RedisConfig` 新增 `Mode`（`standalone` 默认 / `sentinel`）+ `MasterName` + `SentinelAddrs`，全部带零值默认确保老配置零破坏。
- `internal/pkg/storage`: `NewMySQL` 在 `ReadReplicas` 非空时挂 `gorm.io/plugin/dbresolver`，SELECT 自动负载到只读池，写流量留在主库；新增 `HasReplicas()` / `ReadPing(ctx)` 与本地 `roundRobinPolicy`。`NewRedis` 在 `Mode=sentinel` 走 `redis.NewFailoverClient`，自动跟随主库重选举。
- `cmd/api` / `cmd/admin` / `cmd/user-web`: `/readyz` 拆 `mysql.write` + `mysql.read`（仅当配置了 read replica 时注册），任一失败即返回 503，方便 L7 LB 精细摘除。
- `deploy/helm/proxy-vpn/values.yaml`: 新增顶层 `region`、`topologySpread{enabled,maxSkew,topologyKey,whenUnsatisfiable}`、`pdb{enabled,minAvailable}`；`api-deployment.yaml` 与 `worker-deployment.yaml` 同步加 region label + 可选 `topologySpreadConstraints`。
- `deploy/helm/proxy-vpn/templates/pdb.yaml`（新）：可选 PodDisruptionBudget，避免节点 drain 时单实例 API 被同时干掉。
- `deploy/scripts/failover.sh`（新）：手动 promote read replica 到 primary 的 runbook 脚本，含 check / promote 双子命令 + 善后步骤清单。
- `deploy/scripts/verify-region.sh`（新）：单 URL 汇聚 `/healthz` + `/readyz` 探测结果，给 Cloudflare / Route53 health check 用。
- `docs/ops.md`: 新增「跨区灾备」章节（拓扑、DNS HC、RPO/RTO 目标、failover runbook）。
- `docs/architecture.md` §11: 加跨区拓扑说明与 SLO 对齐。

### Added — Phase 13: 节点出口 sing-box + 多协议混合节点
- `internal/node/service/nodecfg/singbox.go`: 新增 `RenderSingBox`，与现有 `RenderXray` 并列，输出完整的 sing-box 1.8+ 服务端配置（log/dns/inbounds/outbounds + v2ray_api stats）。4 协议全覆盖（VLESS-Reality / Trojan / Hysteria2 / SS-2022），Version 哈希算法与 xray 渲染器一致，node-agent 可继续按 hash 跳过未变更的写盘与 reload。
- `internal/node/domain/node.go`: `Node` 新增 `Engine`（`"xray"` | `"sing-box"`，默认 xray）与 `Inbounds`（`json.RawMessage`，可选 `[]NodeInbound`）字段；新增 `NodeInbound` 类型与 `AllInbounds()` / `EffectiveEngine()` / `IsValidEngine()` 辅助。一个节点现在可声明 N 个不同协议 / 不同端口的 inbound 共享同一台 VPS（多协议混合节点）。
- `internal/node/service/nodecfg/render.go`: `RenderXray` 重写为遍历 `Node.AllInbounds()` 产出多个 inbound 段，单 inbound 节点表现与之前完全一致，确保向后兼容。
- `internal/node/service/service.go`: `AgentConfig` 按 `Node.EffectiveEngine()` 分发到 `RenderSingBox` 或 `RenderXray`，节点全量可热切换引擎。
- `internal/node/service/subgen/expand.go`: 新增内部 `expand()` 辅助，把多 inbound 节点在 v2ray / Clash / Sing-box 订阅产出中展开为 N 个虚拟节点条目，名称加 `[protocol]` 后缀防重；clash/singbox/v2ray 三个 generator 入口前都调用一次。
- `internal/node/transport/httpapi/handler.go`: admin `POST /admin/nodes` 与 `PUT /admin/nodes/:id` 新增 `engine`、`inbounds` 字段透传；非法 engine 值返回 400。
- `internal/migrations/000004_node_engine_inbounds.up.sql`: `ALTER TABLE nodes ADD engine VARCHAR(16) NOT NULL DEFAULT 'xray', ADD inbounds JSON NULL`。

### Added — Phase 12: User Web Portal
- `cmd/user-web`: 独立的用户端 Web 二进制，默认监听 `:8082`。复用 `cmd/api` 的全部 bounded contexts（user/billing/payment/node/report），但不挂载 admin 审计中间件——专供普通用户走浏览器登录、买套餐、付款、拉订阅。
- `web/user`: React 18 + TypeScript + Vite + Ant Design 5 + TanStack Query + Zustand + react-router 的用户面 SPA，9 个页面：登录 / 注册（带 60 秒验证码倒计时）/ 仪表盘（余额、流量、套餐到期、邀请码、快速入口）/ 套餐与流量包 / 订单（取消 / 去支付）/ 支付页（多渠道二维码 + USDT 地址金额 + mock-pay + 3s 轮询订单状态）/ 订阅（clash / sing-box / v2ray-base64 + 二维码 + Token 复制）/ 节点列表（30s 轮询）/ 邀请 / 账号（改密 + 2FA enroll/disable）/ 帮助（多平台客户端下载 + FAQ）。
- `cmd/user-web/web.go`: `go:embed all:dist` 嵌入前端产物，SPA fallback 跳过 `/api/`、`/sub/`、`/pay/notify/`、`/healthz`、`/readyz` 等后端路径。
- `deploy/Dockerfile.user-web`: 三段式镜像（node:20 build SPA → golang:1.25 build embed → distroless）；`release.yml` matrix 加 `user-web`，走该 Dockerfile。
- Makefile 新增 `web-user-install` / `web-user-build` / `web-user-dev` / `run-user-web` 目标；后者构建 SPA 后以 `PROXYVPN_HTTP__ADDR=:8082` 启动 `cmd/user-web`。

### Added — Phase 11: Admin Console
- `cmd/admin`: 独立的管理后台二进制，默认监听 `:8081`。复用 `cmd/api` 的 Bootstrap（MySQL/Redis/JWT/各 service 装配），全量挂载 `/api/v1` 路由，登录走 `POST /api/v1/auth/login`，由 `auth.Claims.Role` 限制 admin/ops/finance 才能访问 `/api/v1/admin/*`。
- `internal/pkg/audit`: 新增审计中间件（Record/Writer/ClaimsExtractor + GORM 实现），自动落 `admin_audit_logs` 表。POST/PUT/DELETE/PATCH 异步写入（4 KB payload 截断），GET/4xx/5xx 跳过。`cmd/admin` 通过 path-prefix 过滤只对 `/api/v1/admin/*` 生效。
- `internal/billing` admin API 补齐：`CouponRepo` 加 `List/Get/Create/Update/Delete`，`OrderRepo` 加 `AdminList` + `OrderFilter`，新 `service/admin.go` + `transport/httpapi/admin_extra.go` 暴露 `/api/v1/admin/coupons[/:id]` 和 `/api/v1/admin/orders[/:no]`。
- `internal/payment` admin API：`PaymentRepo.AdminList` + `PaymentFilter` + `service/admin.go` + `transport/httpapi/admin.go`；`requireAdmin` 接受 admin/ops/finance 三种角色（兼顾财务对账场景）。
- `cmd/admin/e2e_test.go` (`//go:build integration && e2e`): admin 登录 → 列用户 → 封禁目标用户 → 验证 audit_logs 行写入的端到端测试。Makefile `test-e2e` 已加 `./cmd/admin/...`。
- `web/admin`: React 18 + TypeScript + Vite + Ant Design 5 + TanStack Query + Zustand + react-router 的完整后台 SPA，包含 Login + Dashboard + Users + Orders + Payments + Plans + Coupons + DataPacks + Nodes + NodeGroups + Reports 11 个页面，axios 拦截器统一处理鉴权与 401 跳转。
- `cmd/admin/web.go`: `go:embed all:dist` 嵌入前端产物，未命中静态资源时回落到 `index.html`（SPA 路由）。
- `deploy/Dockerfile.admin`: 三段式镜像（node:20 build SPA → golang:1.25 build embed → distroless）；release.yml 对 `admin` matrix 走该 Dockerfile，其它 binary 仍走 `deploy/Dockerfile`。
- Makefile 新增 `web-admin-install` / `web-admin-build` / `web-admin-dev` 目标；后者会把 `web/admin/dist/*` 拷到 `cmd/admin/dist/` 供 go:embed 使用。

## [v0.2.0] - 2026-05-26

### Added
- `cmd/api/e2e_test.go` (`//go:build e2e`): end-to-end HTTP test that boots the full user API in-process against testcontainers MySQL+Redis and walks the complete buyer journey — `POST /auth/email/send-code` → `POST /auth/email/register` → `POST /auth/login` → `GET /user/me` → `GET /api/v1/plans` → `POST /api/v1/orders` (plan / mock) → `POST /api/v1/orders/:no/mock-pay` → `GET /sub/:token`. A `fakeMailer` is injected via the new `newMailer` factory hook in `cmd/api/main.go` to capture verification codes without touching SMTP. Wired into the nightly `integration.yml` workflow as a second job step (`go test -tags="integration e2e" ./cmd/api/...`) and exposed locally via `make test-e2e`.
- `cmd/api/main.go`: `newMailer` package-level factory variable so tests can swap the `ports.Mailer` implementation without rebuilding the binary.
- `.github/workflows/integration.yml`: nightly (03:00 UTC) + manual-dispatch workflow that runs the `//go:build integration` suite against real MySQL/Redis/ClickHouse containers on the runner's Docker daemon. Keeps the standard CI fast while still exercising the testcontainer paths regularly.
- `internal/pkg/testsupport`: helpers (`StartMySQL`, `StartRedis`) that spin up disposable MySQL/Redis containers via testcontainers-go for integration tests. Migrations are loaded automatically.
- Integration tests (`//go:build integration` tag) for `traffic/infra/gormrepo` (QuotaRepo + UsageFallbackSink), `traffic/infra/redisban` (BanCache), `user/infra/gormrepo` (UserRepo + RefreshRepo), and `traffic/infra/chsink` end-to-end against a real ClickHouse container (Bootstrap idempotency + Write + materialised-view rollup readback).
- Integration tests for `payment/infra/gormrepo` (PaymentRepo CRUD + MarkPaid + ExpirePending, AddressPoolRepo Seed/Allocate/Release/MarkUsed, ChainScanCursor upsert), `billing/infra/gormrepo` (PlanRepo / DataPackRepo CRUD, CouponRepo quota exhaustion + CountUsedByUser, OrderRepo idempotency + status transitions + ExpirePending), and `node/infra/gormrepo` (GroupRepo CRUD, NodeRepo CRUD + FindByTokenHash + ListByGroups serviceable filter + UpsertHeartbeat + MarkStale).
- `internal/traffic/infra/chsink/chgo`: real ClickHouse driver adapter built on `github.com/ClickHouse/clickhouse-go/v2`. Implements `chsink.Driver` plus an `EnsureDatabase` helper. The CH dependency is isolated to this subpackage so the rest of the tree builds without it.
- `cmd/api` and `cmd/worker` now open the ClickHouse driver when `clickhouse.enabled=true` and call `Sink.Bootstrap` on startup; previously the `Driver` was always nil so enabling CH would have panicked at construction.
- `make test-integration` Makefile target.

### Fixed
- Migration `000003_traffic.up.sql`: dropped the legacy `traffic_daily` table (created by `000001_init_schema.up.sql` with the pre-Phase-6 schema) before recreating it with the new per-user/per-day shape. Previously `CREATE TABLE IF NOT EXISTS` was a no-op against the legacy schema, leaving the `day`/`up_bytes`/`down_bytes` columns missing and breaking `QuotaRepo.UpsertDaily`/`SumDaily` at runtime.

## [v0.1.1] - 2026-05-26

### Fixed
- release workflow: helm OCI push now lowercases the repository owner before pushing to `ghcr.io/<owner>/charts`. v0.1.0 OCI push failed with "invalid_reference: invalid repository" because the owner name contains uppercase letters; the chart was still published as a GH Release asset.

## [v0.1.0] - 2026-05-26

First tagged release. Includes Phases 1-10 plus deployment & CI/CD pipeline. See per-section entries below.

## [v0.1.0] - 2026-05-26 - Phase 6: Traffic accounting & ban list

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

## [v0.1.0] - 2026-05-26 - Phase 7: Node config render + reload loop

### Added
- `internal/node/service/nodecfg`: deterministic xray server-side config renderer (VLESS-Reality / Trojan / Hysteria2 / SS-2022) with stable version hash; per-user `email=u<id>` so xray Stats API queries are trivial.
- `POST /api/v1/node-agent/config` endpoint (node_token auth, supports `known_hash` short-circuit).
- node-agent `configloop.go`: periodic config pull, atomic write to `--config-out`, optional `--reload-cmd` exec.
- `SubscriberPort.ListActive` (excludes banned + expired plans) with gormrepo impl.

## [v0.1.0] - 2026-05-26 - Phase 8: Admin user management + RBAC

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

## [v0.1.0] - 2026-05-26 - Phase 9: Admin reports + ClickHouse rollup MV

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

## [v0.1.0] - 2026-05-26 - Phase 10: Observability + blue/green ops

### Added
- `httpx.Metrics()` Gin middleware: `http_requests_total{method,route,status}` counter, `http_request_duration_seconds{method,route}` histogram (5ms..20s exp×2 buckets), `http_requests_in_flight` gauge. Uses `c.FullPath()` for low-cardinality route labels.
- `docs/ops.md`: Prometheus scrape targets, recommended Grafana panels, starter alerting rules, blue-green & canary deployment runbook, pre-flight checklist & rollback procedure.
- `internal/pkg/httpx/metrics_test.go`: integration test asserting `/metrics` exposes counters for served routes.

Note: `/healthz`, `/readyz`, `/metrics` endpoints and MySQL/Redis readiness checks were already in place; this phase adds per-request metrics and the operations runbook on top.

## [v0.1.0] - 2026-05-26 - Deployment: Dockerfile (multi-bin) + docker-compose + Helm

### Added
- `deploy/docker-compose.yml`: production-style stack (api+worker+mysql+redis), config via PROXYVPN_* env.
- `deploy/helm/proxy-vpn/`: Helm chart v0.1.0
  - api Deployment (rolling, maxSurge=1/maxUnavailable=0) + Service
  - worker Deployment
  - Secret (envFrom) + optional externalSecret bind
  - optional Ingress + ServiceMonitor (Prometheus Operator CRD)
  - blue/green pod label (`colour: blue|green`) for Service-selector flipping
  - liveness/readiness probes wired to /healthz, /readyz
  - non-root, read-only rootfs, dropped caps
- `deploy/helm/proxy-vpn/README.md`: install / blue-green / canary / external-secrets guide

### Changed
- `deploy/Dockerfile`: Go 1.24 → 1.25; added `BIN` build-arg so the same Dockerfile produces api / worker / node-agent / usdt-watcher / admin images.

Verified: `helm lint` + `helm template` clean; `docker compose -f deploy/docker-compose.yml config` valid.

## [v0.1.0] - 2026-05-26 - CI/CD: GitHub Actions release pipeline

### Added
- `.github/workflows/release.yml`: tag-triggered (v*.*.*) pipeline
  - matrix-builds & pushes one multi-arch (amd64+arm64) image per cmd/ entrypoint to GHCR
  - packages the Helm chart with the tag's semver, uploads as GH release asset, and pushes as OCI artifact to `ghcr.io/<owner>/charts`
  - GHA cache scoped per binary
- `helm-lint` job added to `ci.yml` so chart regressions are caught on PR

### Changed
- `ci.yml`: bumped setup-go to `1.25` to match the repo's go.mod; dropped the version-pinned `golangci-lint` step (the pin lagged behind Go 1.25 support).
