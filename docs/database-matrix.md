# Goark ORM 数据库矩阵

## 目标

本矩阵用于约束 MyBatis-Plus 风格多数据库体验。`goark-orm` core 不直接依赖具体数据库驱动，真实数据库验证由调用方或 CI 通过环境变量显式启用。

## 方言覆盖

| 数据库 | DbType | 方言工厂 | 占位符 | 分页语义 |
| --- | --- | --- | --- | --- |
| 通用 JDBC 风格 | `question` | `NewQuestionDialect` | `?` | `LIMIT ? OFFSET ?` |
| PostgreSQL | `postgres` | `NewPostgresDialect` | `$1` | `LIMIT $1 OFFSET $2` |
| MySQL | `mysql` | `NewMySQLDialect` | `?` | `LIMIT ? OFFSET ?` |
| MariaDB | `mariadb` | `NewMariaDBDialect` | `?` | `LIMIT ? OFFSET ?` |
| SQLite | `sqlite` | `NewSQLiteDialect` | `?` | `LIMIT ? OFFSET ?` |
| SQL Server | `sqlserver` | `NewSQLServerDialect` | `@p1` | `OFFSET @p2 ROWS FETCH NEXT @p1 ROWS ONLY` |
| Oracle | `oracle` | `NewOracleDialect` | `:1` | `OFFSET :2 ROWS FETCH NEXT :1 ROWS ONLY` |

## 默认验证

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
GOWORK=off go test -run '^$' -bench . -benchmem ./
```

## 真实数据库 Smoke

默认不会执行真实数据库连接。需要验证具体驱动时，业务工程或 CI 显式提供驱动导入和 DSN：

```bash
GOARK_ORM_INTEGRATION_DRIVER=postgres \
GOARK_ORM_INTEGRATION_DSN='postgres://user:pass@127.0.0.1:5432/goark?sslmode=disable' \
GOWORK=off go test -run TestIntegrationDatabaseSmoke_whenConfigured ./...
```

core 仓库不提交临时 SQL、不生成迁移草稿、不硬编码私有 DSN。真实库用例应只做可回滚的 smoke 或在外部测试库中执行。
