# Observability & Deployment Operations

This document covers the runtime hooks the API exposes for monitoring and
canary / blue-green deployments. Everything here is already wired in — no
code changes needed to operate. Operators only need to scrape `/metrics`
and arrange traffic shifting in front of the deployment.

## Health & Readiness

| Endpoint    | Purpose                                                                                          |
|-------------|--------------------------------------------------------------------------------------------------|
| `/healthz`  | Liveness. Returns 200 as long as the process is alive (used by k8s `livenessProbe`).             |
| `/readyz`   | Readiness. Iterates `Options.ReadinessChecks`：`mysql.write`（主库 Ping），可选 `mysql.read`（read replica Ping，仅当配置了 read replica 时启用），`redis`（主库或 sentinel Ping）。 |
| `/metrics`  | Prometheus exposition format. Includes Go runtime metrics + custom HTTP metrics below.           |

`/readyz` returns 503 when any check fails, with `{ "status": "not_ready",
"checks": {"mysql.write": "fail: ...", "mysql.read": "ok", "redis": "ok"} }`.
Wire it to the k8s `readinessProbe` so traffic is withdrawn from the pod
before failures propagate.

## HTTP Metrics

The `httpx.Metrics()` middleware (installed automatically by
`httpx.NewRouter`) records:

- `http_requests_total{method,route,status}` — counter
- `http_request_duration_seconds{method,route}` — histogram, buckets
  `0.005s..20s` exponential ×2
- `http_requests_in_flight` — gauge

`route` is `c.FullPath()`, e.g. `/api/v1/admin/users/:id/ban`, **not** the
literal request path; this keeps label cardinality bounded even when
clients hit ids. Unmatched (404) requests are labelled `route="unknown"`.

### Recommended Grafana panels

1. **Request rate** — `sum by (route) (rate(http_requests_total[1m]))`
2. **p99 latency**  — `histogram_quantile(0.99,
   sum by (le, route) (rate(http_request_duration_seconds_bucket[5m])))`
3. **Error rate**   — `sum by (route) (rate(http_requests_total{status=~"5.."}[1m]))`
4. **Inflight**     — `http_requests_in_flight`

### Alerting starter rules

```yaml
groups:
  - name: proxy_vpn
    rules:
      - alert: APIHighErrorRate
        expr: sum(rate(http_requests_total{status=~"5.."}[5m])) /
              sum(rate(http_requests_total[5m])) > 0.02
        for: 10m
        labels: {severity: page}
      - alert: APIP99LatencyHigh
        expr: histogram_quantile(0.99,
              sum by (le) (rate(http_request_duration_seconds_bucket[5m]))) > 1
        for: 10m
        labels: {severity: page}
      - alert: APIReadinessFlapping
        expr: avg_over_time(up{job="proxy-vpn-api"}[5m]) < 0.9
        for: 5m
        labels: {severity: ticket}
```

## Blue-Green Deployment

The API is stateless. Two strategies are supported:

### A. Plain blue-green (recommended for hotfixes)

1. Deploy the new version as the **green** colour (separate Deployment,
   same Service selector after switch).
2. Wait for `kubectl rollout status` and verify each pod returns 200 on
   `/readyz` (DB & Redis reachable).
3. Run smoke against the green pods directly via a pre-switch ClusterIP
   (or `kubectl port-forward`). Smoke set: register → login → list plans →
   create order → mock-pay → fetch subscription.
4. Switch the public Service selector from `colour=blue` to `colour=green`.
   Connections drain on the blue side because `terminationGracePeriodSeconds`
   gives in-flight requests time to finish.
5. Keep blue running for ≥10 minutes for instant rollback (just flip the
   selector back).

### B. Canary (recommended for schema-touching changes)

Use an L7 proxy (nginx, Traefik, Istio) to split traffic by weight:

```yaml
# istio VirtualService excerpt
spec:
  http:
    - route:
        - destination: { host: api, subset: stable }
          weight: 95
        - destination: { host: api, subset: canary }
          weight: 5
```

Steps:

1. Run new migrations (forwards-compatible — the old binary must still
   work). Phase 6/8/9 migrations are additive (`ALTER TABLE ADD COLUMN`)
   and therefore canary-safe.
2. Deploy canary with weight 5 %. Watch:
   - error-rate alert above
   - `http_request_duration_seconds` p99 for canary vs stable
   - business metrics: order paid/cancelled ratios from
     `/api/v1/admin/dashboard`
3. Step weights `5→25→50→100` with a ≥30 minute soak at each step.
4. Demote the previous stable subset when canary holds 100 % for 24 h.

### Pre-flight checklist (every release)

- [ ] `go test ./...` green on CI for the tagged commit
- [ ] Migrations are forwards-compatible (no `DROP COLUMN` / type narrowing)
- [ ] `CHANGELOG.md` updated
- [ ] Image tag set in deployment manifest
- [ ] Smoke script run against green
- [ ] Rollback procedure (Service selector flip) documented and tested
      against the previous tag

### Rollback

Blue-green: switch Service selector back.
Canary: set canary weight to 0; the stable subset is unchanged.
Database: roll-forward only — every migration we ship is additive, so
rollback never needs `migrate down`. If a column behaviour changes, the
old binary continues reading the old shape via `COALESCE` (see Phase 8
`admin_user.go` for the established pattern).

## 跨区灾备 (Phase 14-A)

控制面单 Region 部署是 SPOF。本节描述如何用 read replica + Redis Sentinel
+ Helm region 标签 + DNS 健康检查把控制面扩成「主备」拓扑，主区宕机时
RPO ≤ 30s、RTO ≤ 5min（手动 + DNS 切换）。

### 拓扑

```
                    +------------------+
                    |   Cloudflare     |
                    |   DNS HealthCheck|
                    +---------+--------+
                              |
                  failover (primary down)
                              |
   +--------------------------+--------------------------+
   v                                                     v
 [ Region A: us-east-1 ]                       [ Region B: ap-tokyo-1 ]
   api/admin/user-web (active)                   api/admin/user-web (warm standby)
   MySQL primary                                 MySQL read replica (async)
   Redis Sentinel (3-node)                       Redis Sentinel (3-node, cross-region observer)
```

### 配置示例

values-prod-us-east.yaml：
```yaml
region: us-east-1
topologySpread: { enabled: true }
pdb: { enabled: true, minAvailable: 1 }
env:
  PROXYVPN_DB__DSN: "app:pass@tcp(mysql-primary.us-east:3306)/proxy_vpn?parseTime=true"
  PROXYVPN_DB__READ_REPLICAS: "app:pass@tcp(mysql-ro.us-east:3306)/proxy_vpn?parseTime=true,app:pass@tcp(mysql-ro2.us-east:3306)/proxy_vpn?parseTime=true"
  PROXYVPN_DB__RESOLVER_POLICY: random
  PROXYVPN_REDIS__MODE: sentinel
  PROXYVPN_REDIS__MASTER_NAME: mymaster
  PROXYVPN_REDIS__SENTINEL_ADDRS: "sentinel-0.us-east:26379,sentinel-1.us-east:26379,sentinel-2.us-east:26379"
```

values-prod-ap-tokyo.yaml 同上，DSN/Sentinel 地址改成本区域 read replica
作为本地读源、本区域 Sentinel 集群。`region` 字段改为 `ap-tokyo-1`。

### RPO / RTO 目标

| 指标 | 目标 | 备注 |
|---|---|---|
| RPO | ≤ 30s | MySQL 异步复制 lag 上限；超过即告警 |
| RTO | ≤ 5min | DNS TTL=60s + failover.sh 手动 promote |
| 探测频率 | 30s | Cloudflare HC 至 verify-region.sh |

### DNS 健康检查（Cloudflare 示例）

1. 在 Cloudflare 创建 Health Check：`https://api.us-east.example.com/readyz`，
   每 30s 探测，30s 超时，连续 3 次失败 marked DOWN。
2. 创建 Load Balancer，origin pool = [us-east, ap-tokyo]，启用 Geo
   Steering 把亚太流量送 tokyo、其他送 us-east。
3. Health Check 失败时自动把全部流量切到剩余 origin。

### 演练步骤（每季度跑一次）

1. 在 stage 环境执行 `deploy/scripts/verify-region.sh https://api.stage-b.example.com`
   确认备区健康
2. `deploy/scripts/failover.sh check mysql-ro.stage-b` 检查复制 lag < 5s
3. 在备区 promote：`deploy/scripts/failover.sh promote mysql-ro.stage-b $ROOT_PASS`
4. 更新备区 values 的 DSN 指向 promoted host，`helm upgrade`
5. 在 Cloudflare 把 us-east origin 手动 disable，确认流量 5min 内全切到 tokyo
6. 写 runbook 记录 + 反向恢复（把老主库重做成新主库的 replica）
