# proxy_VPN

> 企业级商用 VPN（"机场"）服务 — 多协议、多地域、套餐 + 流量包混合计费。

[![CI](https://github.com/0x1F6A/proxy_VPN/actions/workflows/ci.yml/badge.svg)](https://github.com/0x1F6A/proxy_VPN/actions/workflows/ci.yml)
[![Security](https://github.com/0x1F6A/proxy_VPN/actions/workflows/security.yml/badge.svg)](https://github.com/0x1F6A/proxy_VPN/actions/workflows/security.yml)

## 文档

- [整体服务框架](./整体服务框架.md)
- [API 设计文档](./docs/api.md)
- [开发顺序与稳定性检查](./docs/开发顺序与稳定性检查.md)

## 当前阶段

**Phase 0：项目骨架** — 已交付 Go module、Makefile、CI 流水线、Dockerfile、HTTP 健康/就绪/指标端点。

## 本地快速开始

依赖：Go 1.24+、Make、Docker（可选）。

```bash
git clone https://github.com/0x1F6A/proxy_VPN.git
cd proxy_VPN

cp config.example.yaml config.yaml   # 按需修改

make tidy        # 拉取依赖
make ci          # vet + lint + test
make run-api     # 启动 API，默认监听 :8080

curl localhost:8080/healthz
curl localhost:8080/readyz
curl localhost:8080/metrics | head
```

## 常用命令

| 命令 | 作用 |
|------|------|
| `make build` | 构建所有 `cmd/*` 下的二进制到 `bin/` |
| `make test` | 跑全部单元测试（含 -race） |
| `make cover` | 跑测试并输出覆盖率 |
| `make lint` | 运行 golangci-lint（首次自动安装到 `.tools/`） |
| `make ci` | tidy + vet + lint + test，等同 CI 流水线 |
| `make docker` | 构建 api 镜像 |

## 仓库结构

```
proxy_VPN/
├── cmd/                # 各可执行入口（api, admin, node-agent, usdt-watcher）
├── internal/
│   ├── pkg/            # 通用基础包（config, logger, httpx, middleware）
│   ├── user/  order/  billing/  node/  subscription/  traffic/
│   ├── model/          # 数据模型
│   └── migrations/     # SQL 迁移
├── api/proto/          # gRPC proto 定义
├── web/                # user (Next.js) + admin (Vue) 前端
├── deploy/             # Dockerfile / compose / k8s / 安装脚本
├── docs/               # 设计文档
└── .github/            # CI、PR/Issue 模板、CODEOWNERS
```

## 配置

配置加载优先级：**环境变量 > `./config.yaml` > 默认值**。

环境变量前缀 `PROXYVPN_`，嵌套用双下划线分隔：

```bash
PROXYVPN_HTTP__ADDR=":9090" \
PROXYVPN_LOG__LEVEL="debug" \
PROXYVPN_JWT__SECRET="real-secret" \
go run ./cmd/api
```

## 贡献

- 分支：`feature/<phase>-<name>`、`hotfix/<id>`
- 提交：[Conventional Commits](https://www.conventionalcommits.org/)
- PR 需通过 CI + 至少 1 人 review，涉及钱/鉴权/密钥需 2 人 review

## License

私有项目，未授权前禁止使用、复制或再分发。
