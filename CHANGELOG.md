# Changelog

All notable changes to this project will be documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
