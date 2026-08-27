# Goark ORM 数据库矩阵

## 目标

本矩阵用于约束 MyBatis-Plus 风格多数据库体验。`goark-orm` core 不直接依赖具体数据库驱动，真实数据库验证由调用方或发布机通过环境变量显式启用。

## 方言覆盖

| 数据库 | DbType | 方言工厂 | 占位符 | 标识符 | 分页语义 |
| --- | --- | --- | --- | --- | --- |
| 通用 JDBC 风格 | `question` | `NewQuestionDialect` | `?` | 反引号 | `LIMIT ? OFFSET ?` |
| PostgreSQL | `postgres` | `NewPostgresDialect` | `$1` | 双引号 | `LIMIT $1 OFFSET $2` |
| MySQL | `mysql` | `NewMySQLDialect` | `?` | 反引号 | `LIMIT ? OFFSET ?` |
| MariaDB | `mariadb` | `NewMariaDBDialect` | `?` | 反引号 | `LIMIT ? OFFSET ?` |
| SQLite | `sqlite` | `NewSQLiteDialect` | `?` | 双引号 | `LIMIT ? OFFSET ?` |
| SQL Server | `sqlserver` | `NewSQLServerDialect` | `@p1` | 方括号 | `OFFSET @p2 ROWS FETCH NEXT @p1 ROWS ONLY`，无 `ORDER BY` 时补稳定兜底排序 |
| Oracle | `oracle` | `NewOracleDialect` | `:1` | 双引号 | `OFFSET :2 ROWS FETCH NEXT :1 ROWS ONLY` |

## 方言能力

运行时通过 `orm.NewDialectCapabilities(dbType)` 或 `orm.DialectCapabilitiesOf(dialect)` 查询能力。该能力表描述 SQL 方言语义，不绑定具体 `database/sql` driver；驱动是否支持 OUT 参数、多结果集、JSON 二进制格式等细节仍由业务侧真实库测试确认。

| 数据库 | 主键回读 | UPSERT | 行锁 | JSON | 批量 INSERT | Savepoint |
| --- | --- | --- | --- | --- | --- | --- |
| 通用 JDBC 风格 | 未声明 | 未声明 | 未声明 | 未声明 | 支持 | 支持 |
| PostgreSQL | `RETURNING` | `ON CONFLICT` | `FOR UPDATE`，支持 `SKIP LOCKED` / `NOWAIT` | 原生 | 支持 | 支持 |
| MySQL | `LastInsertId` | `ON DUPLICATE KEY UPDATE` | `FOR UPDATE` | 原生 | 支持 | 支持 |
| MariaDB | `LastInsertId` | `ON DUPLICATE KEY UPDATE` | `FOR UPDATE` | 原生 | 支持 | 支持 |
| SQLite | `LastInsertId` | `ON CONFLICT` | 未声明 | 扩展能力 | 支持 | 支持 |
| SQL Server | `OUTPUT inserted` | `MERGE` | `WITH (...)` 锁提示 | 原生 | 支持 | 支持 |
| Oracle | `RETURNING INTO` | `MERGE` | `FOR UPDATE`，支持 `NOWAIT` | 原生 | 支持 | 支持 |

## 方言 SQL 辅助 API

`goark-orm` core 只生成参数化 SQL 或执行计划，不持有具体数据库连接，也不处理 Migration / DDL 生命周期。

| API | 用途 | 覆盖范围 |
| --- | --- | --- |
| `BuildUpsertSQL` | 生成 `SQLSource`，可直接交给 Provider 或 `CompileSQLContext` | PostgreSQL/SQLite `ON CONFLICT`，MySQL/MariaDB `ON DUPLICATE KEY UPDATE` |
| `RowLockClause` | 生成 SELECT 行锁子句 | `FOR UPDATE`、`SKIP LOCKED`、`NOWAIT`，以及 SQL Server 锁提示 |
| `NewGeneratedKeyPlan` | 返回主键回读计划 | `LastInsertId`、`RETURNING`、`OUTPUT inserted`、`RETURNING INTO` |

## 默认验证

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
GOWORK=off go test -run '^$' -bench . -benchmem ./
```

## 真实数据库兼容性套件

默认不会执行真实数据库连接。需要验证具体驱动时，业务工程或本地发布包显式提供驱动导入和 DSN。`goark-orm` core 不 blank import 任何具体驱动，真实库测试二进制必须由调用方注册驱动：

```bash
GOARK_ORM_INTEGRATION_DRIVER=postgres \
GOARK_ORM_INTEGRATION_DSN='postgres://user:pass@127.0.0.1:5432/goark?sslmode=disable' \
GOARK_ORM_INTEGRATION_DBTYPE=postgres \
GOWORK=off go test -run TestIntegrationDatabaseSuite_whenConfigured ./...
```

`ormtest.RunDatabaseSuiteFromEnv` 会读取以下环境变量：

| 环境变量 | 说明 |
| --- | --- |
| `GOARK_ORM_INTEGRATION_DRIVER` | 已注册的 `database/sql` driver 名称 |
| `GOARK_ORM_INTEGRATION_DSN` | 真实数据库 DSN |
| `GOARK_ORM_INTEGRATION_DBTYPE` | 可选方言类型，支持 `postgres`、`mysql`、`mariadb`、`sqlite`、`sqlserver`、`oracle` |
| `GOARK_ORM_INTEGRATION_SETUP_SQL` | 可选 setup SQL，支持 JSON 字符串数组或分隔符文本 |
| `GOARK_ORM_INTEGRATION_CLEANUP_SQL` | 可选 cleanup SQL，支持 JSON 字符串数组或分隔符文本 |
| `GOARK_ORM_INTEGRATION_SQL_SEPARATOR` | 可选多 SQL 分隔符，默认是 `-- goark-orm statement --` 独立分隔段 |
| `GOARK_ORM_INTEGRATION_TIMEOUT` | 可选用例超时，支持 Go duration 或整数秒 |

core 仓库不提交临时 SQL、不生成迁移草稿、不硬编码私有 DSN。真实库用例应只做可回滚的兼容性检查，或者在外部测试库中执行。

### 标准兼容矩阵

`ormtest.NewCompatibilitySuiteConfig` 会构造一组可回滚的标准用例，覆盖安全绑定、CRUD、ResultMap、分页、批处理和 TypeHandler 入库/出库链路。当前标准 DDL 支持 `postgres`、`mysql`、`mariadb`、`sqlite` 和 `question`，PG/MySQL 是默认真实库验证目标。

业务工程或临时测试 harness 只负责导入驱动并调用：

```go
func TestGoarkORMCompatibility(t *testing.T) {
	ormtest.RunCompatibilitySuiteFromEnv(t)
}
```

若同一个数据库里并发跑多套测试，可通过 `ormtest.WithCompatibilityTable("schema.table_name")` 指定隔离表名。表名只允许 schema/table 分段标识符，防止测试配置把任意 SQL 拼入 DDL。

## 存储过程能力

`goark-orm` core 已通过 fake driver 覆盖 `sql.Out`、INOUT 回写和多结果集扫描链路。真实数据库的存储过程语法、OUT 参数绑定和多结果集支持由具体驱动决定，使用方需要在业务工程或本地环境按上面的环境变量启用真实库 smoke。

| 能力 | core 行为 | 真实库注意事项 |
| --- | --- | --- |
| IN 参数 | 统一走 `#{}` 安全绑定和 TypeHandler | 遵守对应方言的占位符编译结果 |
| OUT 参数 | 绑定为 `sql.Out{Dest: ptr}` | 驱动必须支持 `database/sql` OUT 参数 |
| INOUT 参数 | 绑定为 `sql.Out{Dest: ptr, In: true}` | 驱动必须支持输入输出同参 |
| 多结果集 | 按 `StatementMeta.ResultSets` 顺序调用 `NextResultSet` | 驱动必须实现多结果集前进语义 |
