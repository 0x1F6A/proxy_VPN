# 数据库迁移 (golang-migrate)

迁移文件命名：`NNNNNN_<name>.up.sql` / `NNNNNN_<name>.down.sql`，序号 6 位、递增、不重复。

## 本地执行

依赖（在 Phase 1 docker-compose 中已包含 MySQL；本地可用 brew 安装 CLI）：

```bash
brew install golang-migrate              # 或 go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

应用所有迁移：

```bash
migrate -path internal/migrations \
        -database "mysql://root:root@tcp(127.0.0.1:3306)/proxy_vpn?multiStatements=true" \
        up
```

回滚最近一次：

```bash
migrate -path internal/migrations \
        -database "mysql://root:root@tcp(127.0.0.1:3306)/proxy_vpn?multiStatements=true" \
        down 1
```

## 约定

1. **不可变性**：已合并到 main 的迁移文件**永不修改**，需要变更请新增迁移。
2. **可回滚**：每个 up 必须有对应 down；break-change 类变更可仅写空 down + 文档说明。
3. **小步迁移**：一个文件只做一个语义变更（例如"增加索引"或"新增表"）。
4. **生产先备份**：上线前 dump 一份并验证 down 路径。
5. **多语句**：MySQL 连接串必须带 `multiStatements=true`，否则 `;` 分隔的 DDL 会失败。
6. **应用入口**：API 启动时**不**自动执行迁移；由独立命令 `cmd/api migrate up` (后续阶段加) 或 CI/CD 任务触发，避免多副本竞争。

## 迁移列表

| 序号 | 文件 | 说明 |
|------|------|------|
| 000001 | `init_schema` | 第一版完整 schema（用户/订单/支付/节点/流量/工单/审计） |
