# proxy-vpn Helm Chart

Deploys the API + worker (and optionally node-agent) for [`proxy_VPN`](https://github.com/0x1F6A/proxy_VPN).

## Quick start

```bash
# 1. Build & push per-binary images (one image per cmd/ entrypoint).
docker build -f deploy/Dockerfile --build-arg BIN=api    -t ghcr.io/0x1f6a/proxy-vpn-api:0.1.0 .
docker build -f deploy/Dockerfile --build-arg BIN=worker -t ghcr.io/0x1f6a/proxy-vpn-worker:0.1.0 .
docker push ghcr.io/0x1f6a/proxy-vpn-api:0.1.0
docker push ghcr.io/0x1f6a/proxy-vpn-worker:0.1.0

# 2. Install / upgrade
helm upgrade --install proxy-vpn deploy/helm/proxy-vpn \
  --namespace proxy-vpn --create-namespace \
  --set image.tag=0.1.0 \
  --set env.PROXYVPN_JWT__SECRET="$(openssl rand -hex 32)" \
  --set env.PROXYVPN_DB__DSN='user:pass@tcp(mysql.proxy-vpn.svc:3306)/proxy_vpn?...' \
  --set env.PROXYVPN_REDIS__ADDR=redis-master.proxy-vpn.svc:6379

# 3. Verify
kubectl -n proxy-vpn get pods
kubectl -n proxy-vpn port-forward svc/proxy-vpn-api 8080:8080
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics | head
```

## Blue-Green

Two parallel releases, two colours:

```bash
helm upgrade --install proxy-vpn-blue  deploy/helm/proxy-vpn --set api.bluegreen.colour=blue  --set image.tag=0.1.0
helm upgrade --install proxy-vpn-green deploy/helm/proxy-vpn --set api.bluegreen.colour=green --set image.tag=0.2.0
```

Then a single Service `proxy-vpn-api` (created out-of-band or via separate manifest) selects `colour=blue` or `colour=green`; flip the label to switch traffic. See `docs/ops.md` for full runbook.

## Canary with Istio

Two releases as above, then route by weight in a `VirtualService` (example in `docs/ops.md`).

## Probes & Metrics

Hits the endpoints from `internal/pkg/httpx` (Phase 10):

- `/healthz` — liveness
- `/readyz`  — readiness (MySQL + Redis ping)
- `/metrics` — Prometheus

Set `serviceMonitor.enabled=true` if Prometheus Operator CRDs are installed.

## External secrets

Set `externalSecret.enabled=true` and `externalSecret.name=my-secret` to skip rendering the Secret and bind to an existing one (e.g. created by external-secrets operator from Vault/AWS SM/etc). The secret must contain the same `PROXYVPN_*` keys.
