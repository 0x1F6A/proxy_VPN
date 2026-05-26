# proxy_VPN 详细架构设计文档

> 版本：v0.1
> 维护者：@0x1F6A
> 最后更新：2026-05-26
>
> 本文是 [`整体服务框架.md`](../整体服务框架.md) 的工程化展开，面向开发与运维。
> 阅读顺序建议：**整体服务框架 → 本文 → docs/api.md → docs/开发顺序与稳定性检查.md**。

---

## 1. 设计目标与约束

| 目标 | 指标 |
|------|------|
| 可用性 | 控制面 ≥ 99.9% / 月；节点平均在线率 ≥ 99.5% |
| 吞吐 | 单 API 实例 ≥ 1500 RPS，p99 < 300ms |
| 并发用户 | MVP 支持 5 万注册用户、1 万 DAU、5000 同时在线 |
| 横向扩展 | API、Worker、节点均可水平扩容，控制面无单点 |
| 数据一致性 | 计费/订单强一致；流量统计准实时（≤ 60s 误差） |
| 隐私 | 不记录用户访问目标，仅聚合流量字节 |
| 抗审查 | 多协议、CDN 中转、域名池、TLS 指纹伪装 |
| 部署成本 | MVP 阶段单台 4C8G + N 台 1-2C2G 节点即可承载 |

---

## 2. 系统分层

```
┌──────────────────────────────────────────────────────────────┐
│  L1 接入层  Cloudflare CDN / WAF / DDoS  +  备用域名池       │
├──────────────────────────────────────────────────────────────┤
│  L2 网关层  Caddy (TLS, mTLS 后台, HTTP/3)  +  限流/鉴权     │
├──────────────────────────────────────────────────────────────┤
│  L3 应用层  api-svc | admin-svc | sub-svc | webhook-svc      │
│             (Go, 无状态, 可横向扩展, gRPC 互通)              │
├──────────────────────────────────────────────────────────────┤
│  L4 异步层  Asynq Worker  +  Cron Scheduler  +  usdt-watcher │
├──────────────────────────────────────────────────────────────┤
│  L5 存储层  MySQL 主从 │ Redis 哨兵 │ ClickHouse │ S3        │
├──────────────────────────────────────────────────────────────┤
│  L6 数据面  Node-Agent  ↔  Xray / Sing-box / Hysteria2       │
│             (gRPC 长连接到控制面，独立部署在各 VPS)          │
├──────────────────────────────────────────────────────────────┤
│  L7 观测面  Prometheus + Grafana + Loki + Alertmanager + OTel│
└──────────────────────────────────────────────────────────────┘
```

### 2.1 边界与隔离
- **控制面 / 数据面物理隔离**：控制面用独立域名（如 `api.example.com`），节点用独立域名（如 `jp01.cdn.example.com`），互不暴露。
- **管理后台 / 用户前端隔离**：`admin.example.com` 走独立 Caddy 站点 + IP 白名单 + 强制 2FA。
- **支付回调单独子域**：`pay.example.com/callback/*`，只允许支付平台 IP 段访问。

---

## 3. 服务清单（控制面）

| 服务 | 类型 | 端口 | 状态 | 关键依赖 |
|------|------|------|------|----------|
| `api`            | HTTP (Gin)         | 8080 | 无状态 | MySQL, Redis |
| `admin`          | HTTP (Gin)         | 8081 | 无状态 | MySQL, Redis |
| `subscription`   | HTTP (Gin)         | 8082 | 无状态 | MySQL, Redis |
| `webhook`        | HTTP (Gin)         | 8083 | 无状态 | Redis (queue) |
| `node-rpc`       | gRPC               | 9090 | 长连接 | MySQL, Redis, Asynq |
| `worker`         | Asynq consumer     | -    | 无状态 | Redis, MySQL |
| `scheduler`      | Cron               | -    | **单实例**（leader-election by Redis） | Redis |
| `usdt-watcher`   | 独立进程           | -    | **单实例** | 链 RPC, Redis |
| `node-agent`     | gRPC client + 本地 sidecar | - | 每节点 1 实例 | 控制面 gRPC |

**Phase 0/1 可合并部署**：所有 Go 服务用同一镜像，按 `cmd/*` 子命令启动，简化运维。

---

## 4. 模块边界（六边形/Clean Architecture 思想）

每个业务模块（user / order / billing / node / subscription / traffic）遵循同样的内部结构：

```
internal/<module>/
├── domain/        # 实体、值对象、领域错误（无外部依赖）
├── service/       # 用例编排（依赖 repository 接口）
├── repository/    # 接口定义（由 infra 实现）
├── infra/
│   ├── mysql/     # GORM 实现
│   └── redis/     # 缓存/分布式锁实现
├── transport/
│   ├── http/      # Gin handlers（DTO ↔ domain）
│   └── grpc/      # gRPC handlers
└── module.go      # Wire 装配
```

依赖方向：`transport → service → domain ← repository ← infra`。
**领域层永远不引用 GORM / Gin / Redis 等基础设施。**

---

## 5. 关键技术决策

### 5.1 鉴权
- **Access Token**：`HS256 JWT`，TTL 2h，无状态校验
- **Refresh Token**：随机字节 `base64url(32B)`，存 MySQL `refresh_tokens` 表 + Redis 索引
- **Token 撤销**：登出时把 access token 的 `jti` 写 Redis 黑名单（TTL = 剩余时长）；refresh 行更新 `revoked_at`
- **节点鉴权**：mTLS 客户端证书 + `node_token`，gRPC Metadata 双重校验
- **后台**：admin role + 强制 TOTP + IP 白名单

### 5.2 幂等
- 客户端必带 `Idempotency-Key`（UUID v4）
- 服务端：`SETNX idemp:<key> <order_no> EX 86400`，命中返回缓存订单号
- 表层兜底：`orders.idempotency_key` 唯一索引

### 5.3 分布式锁
- 单 Redis：`SETNX key val PX ttl`，释放用 Lua 校验 owner
- 关键场景：订单回调处理、流量上报聚合、scheduler leader-election

### 5.4 限流
- **接入层**：Cloudflare Rate Limit（粗粒度，按 IP）
- **网关**：Caddy `rate_limit` 插件
- **应用层**：Redis 令牌桶（细粒度，按用户 / 接口）
- **代码示例键名**：`rl:user:<uid>:<bucket>`，原子 `INCR + EXPIRE`

### 5.5 配置与密钥
- 配置：YAML + 环境变量（前缀 `PROXYVPN_`，双下划线表层级）
- 密钥：开发用 `.env`（不入库），生产用 Doppler / AWS SSM / Vault
- 仓库内 CI 强制扫描：`gitleaks` + `trivy fs --scanners secret`

### 5.6 数据库读写
- Phase 1-3：单实例 MySQL 即可
- Phase 6+：主从分离，读走从库，写走主库
- ORM：GORM v2；复杂查询用原生 SQL（`gorm.Raw`）
- 迁移：`golang-migrate`，所有线上变更必须走 migration

### 5.7 缓存策略
- **热点表读缓存**：`plans`、`nodes`、`node_groups` 全量缓存到 Redis，写时失效
- **用户态缓存**：`user:<id>:profile`，TTL 5min，关键字段更新时主动失效
- **订阅缓存**：`sub:<token>:<client_type>`，TTL 5min（兼顾客户端 24h 更新与配置变更）
- **防穿透**：空值缓存（短 TTL）；防雪崩：TTL 加随机偏移；防击穿：单飞 (singleflight)

### 5.8 异步与定时
- 队列：Asynq（基于 Redis Stream），分队列 `critical / default / low`
- 任务：邮件发送、Webhook 推送、对账、流量聚合刷盘、订阅链接生成（重逻辑）
- 定时：cron 表达式集中在 `internal/scheduler/jobs.go`，leader-election 避免多实例重复执行

### 5.9 可观测性
- **日志**：zap → Loki（结构化 JSON，含 `request_id`、`user_id`、`trace_id`）
- **指标**：Prometheus（HTTP RED、DB pool、Redis 命中率、节点在线数、订单成功率、支付通道成功率）
- **追踪**：OpenTelemetry SDK → OTLP → Jaeger / Tempo
- **告警**：Alertmanager → Telegram / 邮件 / 电话（PagerDuty）

---

## 6. 关键数据流

### 6.1 用户登录
```
Client → Caddy → api(POST /auth/login)
   → user.Service.Login()
      → MySQL: 查 users + login_logs
      → Argon2id: 校验密码
      → (可选) TOTP 校验
      → 生成 access JWT (内存) + refresh token (写 MySQL + Redis)
   ← 返回 tokens
   异步：Asynq → 发送登录通知邮件 + 写审计日志
```

### 6.2 创建订单 + 支付
```
Client → api(POST /orders, Idempotency-Key) 
   → order.Service.Create()
      → Redis: SETNX idemp key
      → MySQL TX: insert orders + lock plan stock
      → billing.Provider.CreatePay() (alipay/wechat/usdt)
      → 返回 pay_info
Client → 支付平台扫码
支付平台 → /callback/alipay (异步)
   → billing.Service.HandleCallback()
      → 验签 → 幂等检查 → MySQL TX: 更新订单 + 用户套餐/流量
      → Asynq: 发送购买成功邮件、累加邀请返佣
   ← "success"
```

### 6.3 订阅拉取
```
客户端 → CDN → sub-svc(GET /sub/:token?type=clash)
   → 鉴权 token → 查 user (Redis 缓存)
   → 按 user.plan.node_group 拉节点列表 (Redis 缓存)
   → 模板渲染 → 返回配置文件 + Subscription-Userinfo 头
```

### 6.4 节点连接与流量上报
```
用户客户端 → Xray (节点) — 用 UUID 鉴权
node-agent (定时):
   每 30s: Heartbeat → 控制面更新 node.last_heartbeat
   每 30s: ReportTraffic → traffic-svc 累加 Redis 流量
   监听:  WatchConfig (server-stream) ← 控制面推送配置变更
   收到配置变更 → 写 Xray 配置 → 调 Xray API 热重载（无中断）

控制面 traffic-svc:
   每 1min: 扫描 Redis 已用量 → 超额用户 → 推送到所有节点的"封禁列表"
   每 5min: Redis → MySQL 聚合刷盘
   每 1h:  MySQL → ClickHouse 落档
```

### 6.5 USDT 支付链路
```
usdt-watcher (独立进程):
   每 10s: 调用 TronGrid API 拉收款地址最近交易
        → 匹配 (address + amount_usdt + 时间窗口)
        → 命中 → 内部调 billing-svc 内部 RPC: MarkUSDTPaid(order_no, tx_hash)
        → 同 6.2 后段流程
   15min 未匹配 → 订单 expired
```

---

## 7. 节点 (Data Plane) 详细设计

### 7.1 节点角色
- **接入节点**：直接面向客户端的 Xray/Hysteria2 实例
- **中转节点**（可选）：链式代理（落地中转）
- **CDN 中转节点**：套 Cloudflare/Fastly，伪装为正常 HTTPS

### 7.2 node-agent 职责
1. 启动时向控制面 `Register(node_token)`，拿到 node_id + 初始配置
2. 维护一条 gRPC 长连接（`WatchConfig` 双向流），接收用户列表/路由策略推送
3. 每 30s `Heartbeat`：CPU、内存、带宽、在线人数、Xray 进程状态
4. 每 30s `ReportTraffic`：从 Xray Stats API 批量拉取 → 上报 → 本地清零
5. 接收"封禁名单"事件 → 立即把对应 UUID 从 Xray 配置中移除
6. 失联时本地缓冲流量（BoltDB），重连后补报

### 7.3 节点本地存储
- `/etc/proxy-vpn/agent.yaml`：node_token、控制面地址、TLS 证书
- `/var/lib/proxy-vpn/state.db`：BoltDB，存断线期间缓冲数据
- `/etc/xray/config.json`：Xray 配置（agent 渲染）

### 7.4 节点出口配置（Engine 路由 + 多协议混合）
- **Engine 字段**：`Node.engine` 枚举 `xray` | `sing-box`，默认 `xray`。控制面按节点级动态选择渲染器（`nodecfg.RenderXray` 或 `RenderSingBox`），node-agent 不感知引擎差异——只负责落盘 + 执行 `reloadCmd`。
- **多 inbound 混合**：`Node.inbounds` 字段（JSON 数组）允许一台 VPS 同时暴露多个协议/端口。`Node.AllInbounds()` 把主字段当作第 0 个 inbound，再追加 `inbounds` 中的额外条目，全部映射到同一份用户列表。客户端订阅产出中会按 `name [protocol]` 后缀展开为 N 个虚拟节点条目。
- **Xray inbound 协议**：VLESS+Reality+Vision（首选）、Hysteria2、Trojan、SS-2022
- **Sing-box inbound 协议**：VLESS+Reality、Trojan、Hysteria2、SS-2022（`2022-blake3-aes-128-gcm`）
- **Stats**：xray 启用 `stats` API；sing-box 启用 `experimental.v2ray_api.stats.users[]`，agent 用同一份 traffic 拉取协议批量上报

### 7.5 Xray 配置策略
- 协议：VLESS + Reality + Vision（首选）、Hysteria2、Trojan
- 入站监听端口分组（443 主、备用端口）
- `stats` API 开启 → agent 拉用户流量
- 路由：地理分流（中国直连/国外代理）、广告拦截可选

### 7.6 节点故障转移
- 心跳 `> 90s` 未到 → `online=false`，自动从订阅剔除
- Node-agent 内置自检：Xray 进程崩溃 → 5s 内拉起 + 告警
- 节点完全离线 → 用户客户端自动按订阅切换其它节点

---

## 8. 支付集成详细

### 8.1 支付宝（当面付）
- 应用 ID + RSA2 公私钥（应用私钥 + 支付宝公钥）
- 接口：`alipay.trade.precreate` 生成二维码 URL
- 回调：`POST /callback/alipay`，`alipaySdk.verifyV1`（验签 `RSA2`）
- 幂等：以 `trade_no` 唯一约束 + 状态机判断

### 8.2 微信 Native v3
- 商户号 + APIv3 密钥 + 商户证书 + 微信平台证书
- 接口：`v3/pay/transactions/native`
- 回调：`POST /callback/wechat`，AES-256-GCM 解密报文
- 幂等：`out_trade_no` 唯一

### 8.3 USDT (TRC20)
- 为每笔订单生成"金额尾数"，例如 `3.21 USDT`、`5.23 USDT`，避免混淆
- 收款地址池：1 个主地址即可（金额尾数足够区分）；高并发可分地址
- watcher：TronGrid API 每 10s 拉 `/v1/accounts/<addr>/transactions/trc20?limit=50`
- 匹配规则：`amount == expected && tx_time within [order.created_at, order.expire_at]`
- 链上 12 确认后标记完成（TRC20 实际 ~19 块）

---

## 9. 部署拓扑

### 9.1 MVP（单区域）
```
[Cloudflare]
     │
[Caddy 4C8G  日本东京]
     ├─ api / admin / subscription / webhook (容器)
     ├─ MySQL 8 (含 7 天本地备份, 每日 S3)
     ├─ Redis 7 (AOF + RDB)
     ├─ Asynq worker + scheduler
     ├─ usdt-watcher
     └─ Prometheus + Grafana + Loki

[节点 VPS 群]  (US-W, US-E, JP-TYO, JP-OSA, ...)
     ├─ node-agent (systemd)
     └─ xray-core (systemd)
```

### 9.2 增长期（多区域）
- 控制面：日本主 + 美国/新加坡备，DB 单主多从异步复制
- ClickHouse 集群：3 节点分片
- API：Cloudflare Load Balancer 健康检查 + 故障切换
- Redis：哨兵或 Redis Cluster
- 节点：扩展到欧洲、东南亚

### 9.3 Kubernetes 演进
- 控制面整体迁移到 K8s（GKE / EKS / Self-managed）
- Helm Chart：`api`、`admin`、`worker`、`scheduler`、`subscription` 5 个 Deployment
- 节点保持裸 VPS（K8s 不适合长连接 + 高带宽场景）

---

## 10. 安全详细

| 威胁 | 防护 |
|------|------|
| SQL 注入 | ORM 参数化；禁止字符串拼接 |
| XSS | 前端框架默认转义；CSP 头 |
| CSRF | JWT (非 Cookie) 天然免疫；后台同源策略 + SameSite |
| SSRF | webhook 推送强制白名单域名 + DNS 重绑定防护 |
| 暴力破解 | 登录限流 + Argon2id + 5 次失败锁定 15min |
| 越权 | 中间件强制 `userId == resource.owner_id` 检查 |
| Replay | JWT `jti` 黑名单 + refresh token 一次性 |
| 中间人 | 全站 TLS 1.3 + HSTS + Cloudflare 强制 HTTPS |
| 节点滥用 | 设备指纹限制 + 单 UUID 多 IP 并发告警 |
| 支付伪造 | 回调验签 + 金额二次校验 + 通道对账 |
| 密钥泄漏 | 仓库扫描 + 密钥轮换 + Vault 集中管理 |
| 后台入侵 | 强制 2FA + IP 白名单 + 操作审计 + 异常登录告警 |
| 节点失陷 | 节点持有的 token 仅能上报，无法读取用户列表外的数据；token 可单独吊销 |

---

## 11. 容量与性能预估（MVP）

| 维度 | 估算 |
|------|------|
| 注册用户 | 5 万 → MySQL `users` 表 ~10 MB |
| 在线用户 | 5000 并发 → 节点带宽：按人均 2 Mbps = 10 Gbps，分摊 10 节点 ≈ 1 Gbps/节点 |
| API QPS | 高峰 ~1k RPS（含订阅拉取） → 2 实例 (2C4G) 足够 |
| 订阅缓存 | Redis ~50 MB |
| 流量记录 (ClickHouse) | 5 万用户 × 30 天 × 24 行 = 3600 万行/月，~2 GB |
| 备份 | 数据库 1 GB / 天 → S3 30 GB/月 |

### 11.1 跨区灾备拓扑（Phase 14-A）

控制面默认单 Region；规模化后切换到「主 + 暖备」双区拓扑：

| 资源 | 主区 (us-east-1) | 备区 (ap-tokyo-1) |
|---|---|---|
| api / admin / user-web / worker | active | warm standby（同样起 pod） |
| MySQL | primary（读写） | read replica（异步复制，lag ≤ 30s） |
| Redis | Sentinel 3 节点（master 在本区） | Sentinel 3 节点（cross-region observer） |
| 流量入口 | Cloudflare LB Geo Steering | 失败时自动接管 |

- 配置层：`MySQLConfig.ReadReplicas` 注入只读 DSN 列表，自动挂 `gorm.io/plugin/dbresolver`；`RedisConfig.Mode=sentinel` 切到 `NewFailoverClient`
- 探活层：`/readyz` 输出 `mysql.write / mysql.read / redis` 子状态，503 时 LB 摘除单 pod；Cloudflare HC 30s 探备区 `verify-region.sh`
- 故障转移：`deploy/scripts/failover.sh promote` 手动 promote read replica → primary（runbook 见 `docs/ops.md`）
- 目标：RPO ≤ 30s、RTO ≤ 5min

详见 `docs/ops.md` § 跨区灾备。

---

## 12. 演进路线（与开发顺序文档呼应）

| 版本 | 关键能力 | 主要风险消除 |
|------|---------|------------|
| v0.1 | 项目骨架 + CI                     | 工程规范  |
| v0.2 | 基础设施 + 健康/就绪/指标         | 可观测    |
| v0.3 | 用户体系（含 2FA）                | 鉴权安全  |
| v0.4 | 套餐 + 订单（mock 支付）           | 业务流程  |
| v0.5 | 真实支付（支付宝/微信/USDT）      | 资金流    |
| v0.6 | 节点纳管 + 订阅生成               | 数据面联通 |
| v0.7 | 流量上报 + 配额                   | 计费闭环  |
| v0.8 | 管理后台                          | 运营效率  |
| v0.9 | 用户前端                          | 用户体验  |
| v1.0 | 安全压测 + 灰度 + SLA             | 商用就绪  |

---

## 13. 附录

### 13.1 端口分配
| 端口 | 用途 |
|------|------|
| 80   | Caddy HTTP → 跳转 443 |
| 443  | Caddy HTTPS（api/admin/sub） |
| 8080 | api（内网） |
| 8081 | admin（内网） |
| 8082 | subscription（内网） |
| 8083 | webhook（内网） |
| 9090 | node gRPC |
| 9091 | Prometheus |
| 3000 | Grafana |
| 3306 | MySQL（仅内网） |
| 6379 | Redis（仅内网） |

### 13.2 命名空间约定
- 数据库表名：snake_case 复数（`users`、`orders`）
- Redis 键：`<bizDomain>:<entity>:<id>[:<field>]`，例如 `auth:refresh:rt_xxx`、`rl:user:1024:login`
- Go 包名：小写单词，不缩写
- HTTP 路径：kebab-case 复数，`/api/v1/data-packs`
- gRPC service：`<Domain>Service`，例如 `NodeAgentService`

### 13.3 时区与时间
- 所有存储：UTC（DB 列 `datetime`，应用 `time.Time`）
- 展示：客户端本地化
- 计费日切：UTC 00:00

---

**变更日志**
- v0.1 (2026-05-26)：初版详细架构。
