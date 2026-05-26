# proxy_VPN Makefile
#
# Common entry points used by developers and CI. All Go targets default to
# the ./... package set so adding new modules requires no Makefile edits.

SHELL := /usr/bin/env bash
GO ?= go
PKG := github.com/0x1F6A/proxy_VPN
BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

GOBIN_LOCAL := $(CURDIR)/$(BIN_DIR)
TOOLS_DIR := $(CURDIR)/.tools
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.61.0

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## Run go mod tidy
	$(GO) mod tidy

.PHONY: build
build: ## Build all binaries under cmd/
	@mkdir -p $(BIN_DIR)
	@for d in cmd/*/; do \
		name=$$(basename $$d); \
		echo ">> building $$name"; \
		$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$$name ./$$d || exit 1; \
	done

.PHONY: run-api
run-api: ## Run the api binary locally
	$(GO) run ./cmd/api

.PHONY: test
test: ## Run all unit tests with race detector
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover: ## Run tests with coverage report
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -n 1

.PHONY: test-integration
test-integration: ## Run integration tests (requires Docker; spins up MySQL+Redis containers)
	$(GO) test -tags=integration -count=1 -timeout=10m ./...

.PHONY: test-e2e
test-e2e: ## Run e2e API tests (requires Docker; spins up MySQL+Redis + boots full API in-process)
	$(GO) test -tags="integration e2e" -count=1 -timeout=10m ./cmd/api/... ./cmd/admin/...

.PHONY: web-admin-install
web-admin-install: ## Install web/admin dependencies
	cd web/admin && npm install --no-fund --no-audit

.PHONY: web-admin-build
web-admin-build: ## Build the React admin SPA and stage it for go:embed
	cd web/admin && npm run build
	rm -rf cmd/admin/dist
	mkdir -p cmd/admin/dist
	cp -R web/admin/dist/. cmd/admin/dist/

.PHONY: web-admin-dev
web-admin-dev: ## Run the React admin SPA dev server (proxies /api to :8081)
	cd web/admin && npm run dev

.PHONY: web-user-install
web-user-install: ## Install web/user dependencies
	cd web/user && npm install --no-fund --no-audit

.PHONY: web-user-build
web-user-build: ## Build the React user SPA and stage it for go:embed
	cd web/user && npm run build
	rm -rf cmd/user-web/dist
	mkdir -p cmd/user-web/dist
	cp -R web/user/dist/. cmd/user-web/dist/

.PHONY: web-user-dev
web-user-dev: ## Run the React user SPA dev server (proxies /api and /sub to :8082)
	cd web/user && npm run dev

# ---------- 本地手动联调 (cmd/admin) ----------
DEV_DSN ?= root:root@tcp(127.0.0.1:3306)/proxy_vpn?charset=utf8mb4&parseTime=true&loc=UTC
DEV_ADMIN_EMAIL ?= admin@local.test
DEV_ADMIN_PASSWORD ?= admin123

.PHONY: dev-up
dev-up: ## 启动本地 MySQL/Redis/MailHog/Grafana 等开发依赖
	@[ -f config.yaml ] || (cp config.example.yaml config.yaml && echo ">> 已生成 config.yaml")
	docker compose -f deploy/docker-compose.dev.yml up -d
	@echo ">> 等待 MySQL 健康..." && \
	for i in $$(seq 1 30); do \
	  docker exec proxy-vpn-mysql mysqladmin ping -h127.0.0.1 -uroot -proot --silent >/dev/null 2>&1 && break; \
	  sleep 1; \
	done; echo "   MySQL ready"

.PHONY: dev-down
dev-down: ## 停止本地开发依赖
	docker compose -f deploy/docker-compose.dev.yml down

.PHONY: dev-migrate
dev-migrate: ## 把 internal/migrations 全部 up.sql 顺序应用到本地 MySQL
	@for f in internal/migrations/*.up.sql; do \
	  echo ">> apply $$f"; \
	  docker exec -i proxy-vpn-mysql mysql -uroot -proot proxy_vpn < $$f || exit 1; \
	done
	@echo "✅ migrations applied"

.PHONY: dev-seed-admin
dev-seed-admin: ## 创建/重置一个 admin 账号（默认 admin@local.test / admin123）
	$(GO) run ./cmd/seed-admin \
	  --dsn '$(DEV_DSN)' \
	  --email '$(DEV_ADMIN_EMAIL)' \
	  --password '$(DEV_ADMIN_PASSWORD)' \
	  --role admin

.PHONY: run-admin
run-admin: web-admin-build ## 构建 SPA 后启动 cmd/admin (:8081)，浏览器开 http://127.0.0.1:8081
	PROXYVPN_HTTP__ADDR=":8081" $(GO) run ./cmd/admin

.PHONY: run-user-web
run-user-web: web-user-build ## 构建 SPA 后启动 cmd/user-web (:8082)，浏览器开 http://127.0.0.1:8082
	PROXYVPN_HTTP__ADDR=":8082" $(GO) run ./cmd/user-web

.PHONY: dev-admin
dev-admin: dev-up dev-migrate dev-seed-admin ## 一键: 起依赖 + 迁库 + 建管理员 (再手动跑 make run-admin)
	@echo ""
	@echo "🎉 准备就绪。下一步:"
	@echo "    make run-admin          # 构建 SPA 并启动 cmd/admin"
	@echo "    浏览器打开 http://127.0.0.1:8081"
	@echo "    账号 $(DEV_ADMIN_EMAIL) / 密码 $(DEV_ADMIN_PASSWORD)"

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

$(GOLANGCI_LINT):
	@mkdir -p $(TOOLS_DIR)
	GOBIN=$(TOOLS_DIR) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix ./...

.PHONY: ci
ci: tidy vet lint test ## Aggregate target run by CI

.PHONY: clean
clean: ## Remove build & test artifacts
	rm -rf $(BIN_DIR) coverage.out

.PHONY: docker
docker: ## Build the api docker image
	docker build -f deploy/Dockerfile -t proxy_vpn/api:$(VERSION) .
