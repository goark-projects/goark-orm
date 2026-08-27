# Goark ORM Database Matrix

## Scope

The matrix records SQL dialect behavior and reusable real database test coverage. Core packages do not import concrete drivers and do not manage migrations. Driver registration, DSNs, setup SQL, and cleanup SQL belong to the caller's test harness.

## Real Support Boundary

PostgreSQL and MySQL are the only current real supported database targets. The reusable standard compatibility suite creates DDL and executable cases only for those two engines. Other dialects listed below are SQL generation capabilities, not a current real database support claim.

## SQL Generation Dialects

| Database | `DbType` | Factory | Placeholder | Identifier quote | Pagination |
| --- | --- | --- | --- | --- | --- |
| Question placeholder | `question` | `NewQuestionDialect` | `?` | backtick | `LIMIT ? OFFSET ?` |
| PostgreSQL | `postgres` | `NewPostgresDialect` | `$1` | double quote | `LIMIT $1 OFFSET $2` |
| MySQL | `mysql` | `NewMySQLDialect` | `?` | backtick | `LIMIT ? OFFSET ?` |
| MariaDB | `mariadb` | `NewMariaDBDialect` | `?` | backtick | `LIMIT ? OFFSET ?` |
| SQLite | `sqlite` | `NewSQLiteDialect` | `?` | double quote | `LIMIT ? OFFSET ?` |
| SQL Server | `sqlserver` | `NewSQLServerDialect` | `@p1` | square brackets | `OFFSET @p2 ROWS FETCH NEXT @p1 ROWS ONLY`; a stable fallback order is added when no top-level `ORDER BY` exists |
| Oracle | `oracle` | `NewOracleDialect` | `:1` | double quote | `OFFSET :2 ROWS FETCH NEXT :1 ROWS ONLY` |

## Dialect Capabilities

`orm.NewDialectCapabilities(dbType)` and `orm.DialectCapabilitiesOf(dialect)` expose SQL capability metadata. The metadata describes SQL generation behavior; driver-level details such as OUT parameter support, multi-result sets, and binary JSON formats must still be verified against the selected driver.

| Database | Generated key | Upsert | Row lock | JSON | Batch insert | Savepoint |
| --- | --- | --- | --- | --- | --- | --- |
| Question placeholder | unspecified | unspecified | unspecified | unspecified | supported | supported |
| PostgreSQL | `RETURNING` | `ON CONFLICT` | `FOR UPDATE`, `SKIP LOCKED`, `NOWAIT` | native | supported | supported |
| MySQL | `LastInsertId` | `ON DUPLICATE KEY UPDATE` | `FOR UPDATE` | native | supported | supported |
| MariaDB | `LastInsertId` | `ON DUPLICATE KEY UPDATE` | `FOR UPDATE` | native | supported | supported |
| SQLite | `LastInsertId` | `ON CONFLICT` | unspecified | extension dependent | supported | supported |
| SQL Server | `OUTPUT inserted` | `MERGE` | `WITH (...)` hints | native | supported | supported |
| Oracle | `RETURNING INTO` | `MERGE` | `FOR UPDATE`, `NOWAIT` | native | supported | supported |

## SQL Helper APIs

| API | Purpose | Coverage |
| --- | --- | --- |
| `BuildUpsertSQL` | Builds a parameterized `SQLSource` for provider or direct compilation paths | PostgreSQL/SQLite `ON CONFLICT`, MySQL/MariaDB `ON DUPLICATE KEY UPDATE` |
| `RowLockClause` | Builds row-lock SQL clauses | `FOR UPDATE`, `SKIP LOCKED`, `NOWAIT`, and SQL Server lock hints |
| `NewGeneratedKeyPlan` | Builds generated-key readback plans | `LastInsertId`, `RETURNING`, `OUTPUT inserted`, and `RETURNING INTO` |

These APIs produce SQL or execution plans only. They do not create tables and do not own schema lifecycle.

## Real Database Suite

The standard suite is disabled unless all required environment variables are provided and the caller's test binary imports a concrete driver. It currently supports PostgreSQL and MySQL only; other database engines are intentionally outside the standard real-database support boundary until they get their own verified matrix.

Use `ormtest.SupportedCompatibilityDBTypes()` or `ormtest.IsCompatibilityDBTypeSupported(dbType)` when a caller-owned harness needs to branch on the current standard support boundary. The list is intentionally limited to `postgres` and `mysql`.

```bash
GOARK_ORM_INTEGRATION_DRIVER=postgres \
GOARK_ORM_INTEGRATION_DSN='postgres://user:pass@127.0.0.1:5432/goark?sslmode=disable' \
GOARK_ORM_INTEGRATION_DBTYPE=postgres \
GOWORK=off go test -run TestORMDatabaseCompatibility ./...
```

| Environment variable | Description |
| --- | --- |
| `GOARK_ORM_INTEGRATION_DRIVER` | Registered `database/sql` driver name |
| `GOARK_ORM_INTEGRATION_DSN` | Real database DSN |
| `GOARK_ORM_INTEGRATION_DBTYPE` | Required standard-suite database type: `postgres` or `mysql` |
| `GOARK_ORM_INTEGRATION_SETUP_SQL` | Optional setup SQL as a JSON string array or separated text |
| `GOARK_ORM_INTEGRATION_CLEANUP_SQL` | Optional cleanup SQL as a JSON string array or separated text |
| `GOARK_ORM_INTEGRATION_SQL_SEPARATOR` | Optional multi-statement separator; default is `-- goark-orm statement --` as a standalone segment |
| `GOARK_ORM_INTEGRATION_TIMEOUT` | Optional test timeout as a Go duration or integer seconds |

## Standard Compatibility Cases

`ormtest.NewCompatibilitySuiteConfig` builds rollback-friendly cases for:

- ping and setup/cleanup execution
- safe parameter binding
- query and single-row query
- pagination
- inserts, updates, deletes, and batch execution
- type handler round trips
- upsert
- generated key readback
- row-lock smoke paths
- callable statements

Current standard DDL support covers only `postgres` and `mysql`. The generic dialect APIs may still compile SQL for other dialects, but the reusable real database suite does not claim support for them.

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

Use `ormtest.WithCompatibilityTable("schema.table_name")` when multiple suites need isolated table names in one database. Table names are parsed as identifier segments to keep test configuration from injecting arbitrary SQL into DDL.

## Local PostgreSQL/MySQL Matrix Verification

Use `scripts/verify-real-db.ps1` to run the standard PostgreSQL and MySQL suites from an isolated temporary module. The script imports concrete drivers only inside the temporary module, keeps `goark.dev/orm` dependency-light, and removes the temporary module after the run.

```powershell
$env:GOARK_ORM_POSTGRES_DSN = 'postgres://user:pass@127.0.0.1:5432/goark-orm-test?sslmode=disable'
$env:GOARK_ORM_MYSQL_DSN = 'user:pass@tcp(127.0.0.1:3306)/goark-orm-test?parseTime=true'
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db.ps1
```

Optional variables:

| Environment variable | Default | Description |
| --- | --- | --- |
| `GOARK_ORM_POSTGRES_DRIVER` | `pgx` | Registered PostgreSQL `database/sql` driver name |
| `GOARK_ORM_POSTGRES_DSN` | none | PostgreSQL DSN; empty value skips PostgreSQL |
| `GOARK_ORM_MYSQL_DRIVER` | `mysql` | Registered MySQL `database/sql` driver name |
| `GOARK_ORM_MYSQL_DSN` | none | MySQL DSN; empty value skips MySQL |

## Callable Statement Notes

The core test suite covers `sql.Out`, INOUT writeback, and multi-result-set scanning with fake drivers. Real database callable syntax and driver support still need a driver-specific smoke test.

| Capability | Core behavior | Real database note |
| --- | --- | --- |
| IN parameter | Uses `#{}` binding and TypeHandler conversion | Follows compiled placeholders for the selected dialect |
| OUT parameter | Binds `sql.Out{Dest: ptr}` | Driver must support `database/sql` OUT parameters |
| INOUT parameter | Binds `sql.Out{Dest: ptr, In: true}` | Driver must support input and output through the same parameter |
| Multiple result sets | Scans in `StatementMeta.ResultSets` order | Driver must implement result set advancement |

## Default Local Checks

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
GOWORK=off go test -run '^$' -bench . -benchmem ./
git diff --check
```
