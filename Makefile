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
