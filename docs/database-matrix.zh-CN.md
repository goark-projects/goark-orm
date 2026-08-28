# Goark ORM 数据库矩阵

## 范围

本矩阵记录 SQL 方言行为和可复用真实数据库测试覆盖。Core packages 不导入具体驱动，也不管理 migration。驱动注册、DSN、setup SQL 和 cleanup SQL 属于调用方测试 harness。

## 真实支持边界

PostgreSQL、MySQL、MariaDB、SQLite、SQL Server 和 Oracle 是当前标准真实数据库目标。可复用兼容性套件为这些引擎创建 DDL 和可执行 case，且不把具体驱动导入 core。问号占位符方言只作为 SQL 生成能力存在。

## SQL 生成方言

| 数据库 | `DbType` | Factory | Placeholder | Identifier quote | Pagination |
| --- | --- | --- | --- | --- | --- |
| Question placeholder | `question` | `NewQuestionDialect` | `?` | backtick | `LIMIT ? OFFSET ?` |
| PostgreSQL | `postgres` | `NewPostgresDialect` | `$1` | double quote | `LIMIT $1 OFFSET $2` |
| MySQL | `mysql` | `NewMySQLDialect` | `?` | backtick | `LIMIT ? OFFSET ?` |
| MariaDB | `mariadb` | `NewMariaDBDialect` | `?` | backtick | `LIMIT ? OFFSET ?` |
| SQLite | `sqlite` | `NewSQLiteDialect` | `?` | double quote | `LIMIT ? OFFSET ?` |
| SQL Server | `sqlserver` | `NewSQLServerDialect` | `@p1` | square brackets | `OFFSET @p2 ROWS FETCH NEXT @p1 ROWS ONLY`；缺少顶层 `ORDER BY` 时会添加稳定 fallback order |
| Oracle | `oracle` | `NewOracleDialect` | `:1` | double quote | `OFFSET :2 ROWS FETCH NEXT :1 ROWS ONLY` |

## 方言能力

`orm.NewDialectCapabilities(dbType)` 和 `orm.DialectCapabilitiesOf(dialect)` 暴露 SQL 能力元数据。该元数据描述 SQL 生成行为；OUT parameter 支持、多结果集、二进制 JSON 格式等驱动层细节仍必须按所选驱动验证。

| 数据库 | Generated key | Upsert | Row lock | JSON | Batch insert | Savepoint |
| --- | --- | --- | --- | --- | --- | --- |
| Question placeholder | unspecified | unspecified | unspecified | unspecified | supported | supported |
| PostgreSQL | `RETURNING` | `ON CONFLICT` | `FOR UPDATE`、`SKIP LOCKED`、`NOWAIT` | native | supported | supported |
| MySQL | `LastInsertId` | `ON DUPLICATE KEY UPDATE` | `FOR UPDATE` | native | supported | supported |
| MariaDB | `LastInsertId` | `ON DUPLICATE KEY UPDATE` | `FOR UPDATE` | native | supported | supported |
| SQLite | `LastInsertId` | `ON CONFLICT` | unspecified | extension dependent | supported | supported |
| SQL Server | `OUTPUT inserted` | `MERGE` | `WITH (...)` hints | native | supported | supported |
| Oracle | `RETURNING INTO` | `MERGE` | `FOR UPDATE`、`NOWAIT` | native | supported | supported |

## SQL 辅助 API

| API | 目的 | 覆盖 |
| --- | --- | --- |
| `BuildUpsertSQL` | 为 Provider 或直接编译路径构造参数化 `SQLSource` | PostgreSQL/SQLite `ON CONFLICT`、MySQL/MariaDB `ON DUPLICATE KEY UPDATE`、SQL Server/Oracle `MERGE` |
| `RowLockClause` | 构造行锁 SQL 子句 | `FOR UPDATE`、`SKIP LOCKED`、`NOWAIT` 和 SQL Server lock hints |
| `NewGeneratedKeyPlan` | 构造生成主键回读计划 | `LastInsertId`、`RETURNING`、`OUTPUT inserted`、`RETURNING INTO` |

这些 API 只生成 SQL 或执行计划。它们不创建表，也不拥有 schema 生命周期。

## 真实数据库套件

标准套件默认禁用，只有提供全部必要环境变量且调用方测试 binary 导入具体驱动后才执行。当前支持 PostgreSQL、MySQL、MariaDB、SQLite、SQL Server 和 Oracle。问号占位符方言不属于标准真实数据库支持边界。

调用方 harness 需要按当前标准支持边界分支时，可使用 `ormtest.SupportedCompatibilityDBTypes()` 或 `ormtest.IsCompatibilityDBTypeSupported(dbType)`。该列表刻意限制在具备可执行套件覆盖的引擎：`postgres`、`mysql`、`mariadb`、`sqlite`、`sqlserver`、`oracle`。

```bash
# 执行前在仓库外设置 GOARK_ORM_INTEGRATION_DSN。
GOARK_ORM_INTEGRATION_DRIVER=postgres \
GOARK_ORM_INTEGRATION_DBTYPE=postgres \
GOWORK=off go test -run TestORMDatabaseCompatibility ./...
```

| 环境变量 | 说明 |
| --- | --- |
| `GOARK_ORM_INTEGRATION_DRIVER` | 已注册的 `database/sql` driver name |
| `GOARK_ORM_INTEGRATION_DSN` | 真实数据库 DSN |
| `GOARK_ORM_INTEGRATION_DBTYPE` | 必填标准套件数据库类型：`postgres`、`mysql`、`mariadb`、`sqlite`、`sqlserver` 或 `oracle` |
| `GOARK_ORM_INTEGRATION_SETUP_SQL` | 可选 setup SQL，格式为 JSON string array 或分隔文本 |
| `GOARK_ORM_INTEGRATION_CLEANUP_SQL` | 可选 cleanup SQL，格式为 JSON string array 或分隔文本 |
| `GOARK_ORM_INTEGRATION_SQL_SEPARATOR` | 可选多语句分隔符；默认是单独片段 `-- goark-orm statement --` |
| `GOARK_ORM_INTEGRATION_TIMEOUT` | 可选测试 timeout，可为 Go duration 或整数秒 |

## 标准兼容性 Cases

`ormtest.NewCompatibilitySuiteConfig` 构造 rollback-friendly cases，覆盖：

- ping 与 setup/cleanup 执行
- 安全参数绑定
- query 与 single-row query
- pagination
- inserts、updates、deletes 和 batch execution
- type handler round trips
- native JSON column validation 和 JSON scalar extraction
- upsert
- generated key readback
- row-lock smoke paths
- 具备可移植 `database/sql` 路径的 callable statements

当前标准 DDL 支持 `postgres`、`mysql`、`mariadb`、`sqlite`、`sqlserver` 和 `oracle`。PostgreSQL 使用 `JSONB`，MySQL/MariaDB 使用 `JSON`，SQL Server 使用带 `ISJSON` 的 `NVARCHAR(MAX)`，Oracle 使用带 `IS JSON` 的 `CLOB`，SQLite 使用 `json_valid` 保护文本。SQLite 跳过 callable，因为 `database/sql` SQLite drivers 不暴露可移植存储过程模型。

```go
package user_test

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"goark.dev/orm/ormtest"
)

func TestORMDatabaseCompatibility(t *testing.T) {
	ormtest.RunCompatibilitySuiteFromEnv(t)
}
```

同一数据库需要运行多个隔离套件时，使用 `ormtest.WithCompatibilityTable("schema.table_name")`。表名按 identifier segments 解析，避免测试配置把任意 SQL 注入 DDL。

## 本地真实数据库矩阵验证

使用 `scripts/verify-real-db.ps1` 从隔离临时 module 运行标准 PostgreSQL、MySQL、MariaDB、SQLite、SQL Server 和 Oracle 套件。该脚本只在临时 module 中导入具体驱动，保持 `goark.dev/orm` 依赖轻量，并在运行结束后删除临时 module。

```powershell
# 执行前在 shell 或 CI secret store 中设置 GOARK_ORM_POSTGRES_DSN、
# GOARK_ORM_MYSQL_DSN 或其他受支持的 DSN 变量。
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db.ps1
```

可选变量：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `GOARK_ORM_POSTGRES_DRIVER` | `pgx` | 已注册 PostgreSQL `database/sql` driver name |
| `GOARK_ORM_POSTGRES_DSN` | none | PostgreSQL DSN；空值跳过 PostgreSQL |
| `GOARK_ORM_MYSQL_DRIVER` | `mysql` | 已注册 MySQL `database/sql` driver name |
| `GOARK_ORM_MYSQL_DSN` | none | MySQL DSN；空值跳过 MySQL |
| `GOARK_ORM_MARIADB_DRIVER` | `mysql` | 已注册 MariaDB-compatible `database/sql` driver name |
| `GOARK_ORM_MARIADB_DSN` | none | MariaDB DSN；空值跳过 MariaDB |
| `GOARK_ORM_SQLITE_DRIVER` | `sqlite` | 已注册 SQLite `database/sql` driver name |
| `GOARK_ORM_SQLITE_DSN` | none | SQLite DSN；空值跳过 SQLite |
| `GOARK_ORM_SQLITE_IMPORT` | none | 设置 `GOARK_ORM_SQLITE_DSN` 时必填的 Go blank-import path，例如 `modernc.org/sqlite` |
| `GOARK_ORM_SQLSERVER_DRIVER` | `sqlserver` | 已注册 SQL Server `database/sql` driver name |
| `GOARK_ORM_SQLSERVER_DSN` | none | SQL Server DSN；空值会在存在 admin DSN 时使用 admin DSN，否则跳过 SQL Server |
| `GOARK_ORM_SQLSERVER_ADMIN_DSN` | none | 可选 SQL Server admin DSN，用于创建 `GOARK_ORM_SQLSERVER_DATABASE` |
| `GOARK_ORM_SQLSERVER_DATABASE` | `goark_orm_test` | 临时 harness 使用的 SQL Server database name |
| `GOARK_ORM_ORACLE_DRIVER` | `oracle` | 已注册 Oracle `database/sql` driver name |
| `GOARK_ORM_ORACLE_DSN` | none | Oracle DSN；空值跳过 Oracle |

## 本地真实数据库 Benchmark 验证

使用 `scripts/verify-real-db-bench.ps1` 运行同样 driver-isolated 临时 module 的 benchmark harness。

```powershell
# 执行数据库 benchmark harness 前，在 shell 或 CI secret store 中设置目标
# GOARK_ORM_*_DSN 变量。
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db-bench.ps1 -BenchTime 1s
```

Benchmark 矩阵包括：

- prepared query reuse with generated row scanner paths
- ResultMap plus JSON TypeHandler on native JSON-capable columns
- single-row insert with TypeHandler binding
- `NewMultiRowInsertSQLBuilder` 生成的 multi-row insert
- sequential `BatchSession` insert，用于对比
- 每种方言的 native upsert

绝对 `ns/op` 值依赖发布主机。强制延迟预算时，请使用稳定硬件、固定网络拓扑、已预热数据库连接池和重复运行。Allocation counts 以及 multi-row insert 相对 sequential batch 这类路径对比，是更可移植的回归信号。

## Callable Statement 说明

Core test suite 覆盖 fake drivers 上的 `sql.Out`、INOUT writeback 和 multi-result-set scanning。真实数据库 callable syntax 和驱动支持仍需要 driver-specific smoke test。

| 能力 | Core 行为 | 真实数据库说明 |
| --- | --- | --- |
| IN parameter | 使用 `#{}` binding 和 TypeHandler conversion | 遵循所选方言编译后的 placeholders |
| OUT parameter | 绑定 `sql.Out{Dest: ptr}` | Driver 必须支持 `database/sql` OUT parameters |
| INOUT parameter | 绑定 `sql.Out{Dest: ptr, In: true}` | Driver 必须支持同一参数的输入和输出 |
| Multiple result sets | 按 `StatementMeta.ResultSets` 顺序扫描 | Driver 必须实现 result set advancement |

## 默认本地检查

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
GOWORK=off go test -run '^$' -bench . -benchmem ./internal/runtime
git diff --check
```
