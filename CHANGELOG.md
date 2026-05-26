# Changelog

All notable changes to this project will be documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
