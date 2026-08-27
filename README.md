# Goark ORM

Goark ORM is a Go-native data mapper for `database/sql`. It keeps mapper metadata explicit, generates stable Go code, and exposes small runtime contracts for sessions, transactions, type handlers, SQL building, result mapping, routing, caching, and testable database access.

Default documentation is written in English. Chinese documentation is available in [README.zh-CN.md](README.zh-CN.md), and the examples guide is available in [docs/examples.md](docs/examples.md) and [docs/examples.zh-CN.md](docs/examples.zh-CN.md).

## Module

```text
module goark.dev/orm
```

`orm.APIVersion` currently reports `v1`.

## Design Boundaries

- Runtime code uses explicit metadata registration. It does not scan mappers, XML files, or entities at runtime.
- Generated mapper code depends on the `orm.Session` interface, so auto-commit sessions, transaction sessions, routing sessions, batch sessions, and streaming signatures share the same generated surface.
- Core packages do not import concrete database drivers. Real database tests are enabled by the caller through explicit driver imports and environment variables.
- Migration and DDL lifecycle management are intentionally outside this module.
- Raw SQL substitution through `${}` accepts only explicit `RawSQLToken` values such as `RawIdentifier` and `RawOrderBy`.
- JSON processing is routed through the internal JSON codec backed by ByteDance Sonic.

## Features

- Entity metadata from `//goark-orm:entity` and strict `goark-orm` struct tags.
- Mapper metadata from `//goark-orm:mapper`, SQL method annotations, and XML mapper files.
- Generated metadata registration, entity row scanners, mapper implementations, typed field constants, `BaseMapper` factories, and `Service` factories.
- Dynamic XML SQL nodes: `sql/include`, `bind`, `if`, `where`, `set`, `trim`, `foreach`, and `choose/when/otherwise`.
- Safe expression evaluation for dynamic SQL, including boolean logic, comparisons, arithmetic, ternary expressions, collection tests, `empty`, `in/not in`, and whitelisted read-only string or collection helpers.
- PostgreSQL and MySQL are the current real supported database targets. MariaDB, SQLite, SQL Server, Oracle, and the question-placeholder dialect remain SQL generation dialects only.
- Statement options for timeout, fetch size, result set type, ordered result mapping, generated key columns, cache behavior, and interceptor ignore lists.
- Callable statements with IN, OUT, INOUT parameters, `sql.Out` binding, and ordered multi-result-set scanning.
- Result maps with constructor arguments, associations, collections, discriminator branches, nested selects, explicit lazy loading, column prefixes, and not-null guards.
- Type handlers at registry and session level, including JSON, time, decimal, string, bool, and bytes handlers.
- `BaseMapper`, `Service`, `QueryWrapper`, `UpdateWrapper`, typed fields, pagination, batch writes, logical delete, optimistic locking, key generation, and automatic fill hooks.
- SQL provider descriptors and fluent SQL builders for select, insert, update, delete, upsert, row locks, generated key plans, and cache key extensions.
- SQL session middleware and interceptors for statement execution, statement handling, parameter handling, result set handling, pagination, tenant constraints, data permissions, dynamic table names, SQL observation, block-attack protection, illegal SQL rules, read-only sessions, and custom governance rules.
- Local cache, namespace-level second-level cache, LRU eviction, blocking cache miss coalescing, cache stats, and transaction-aware cache publication.
- Routing sessions and routing factories for explicit data-source selection, read/write split routing, and statement-based routing.
- `ormgen` schema introspection, reverse engineering, custom template rendering, schema drift detection, and schema compatibility validation helpers.
- `ormtest` real database suites for ping, setup/cleanup, query, pagination, writes, batch execution, type handlers, upsert, generated keys, row locks, and callable statements.

## Quick Start

Declare an entity and mapper:

```go
package user

import (
	"context"

	orm "goark.dev/orm"
)

//goark-orm:entity(table="sys_user")
type User struct {
	ID     int64  `goark-orm:"column='id';primary-key=true;auto-increment=true"`
	Name   string `goark-orm:"column='name';size=64;nullable=false"`
	Status string `goark-orm:"column='status';size=32;nullable=false"`
}

//goark-orm:mapper(namespace="example.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id, name, status from sys_user where id = #{id}")
	FindByID(ctx context.Context, id int64) (*User, error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListByStatus(ctx context.Context, status string, page orm.PageRequest) (orm.Page[User], error)
}
```

Generate mapper code:

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
```

Use generated metadata and a session:

```go
registry := orm.NewRegistry()
if err := RegisterGoarkORMMetadata(registry); err != nil {
	return err
}
if err := registry.Validate(); err != nil {
	return err
}

session, err := orm.NewSQLSession(registry, db, orm.NewPostgresDialect())
if err != nil {
	return err
}

mapper := NewUserMapper(session)
user, err := mapper.FindByID(ctx, 7)
if err != nil {
	return err
}
_ = user
```

Use generated field constants with the generic mapper:

```go
baseMapper, err := NewUserBaseMapper(session)
if err != nil {
	return err
}

page, err := baseMapper.SelectPage(
	ctx,
	orm.NewPageRequest(1, 20),
	orm.NewQueryWrapper[User]().
		Eq(UserFields.Status, "ACTIVE").
		OrderByAsc(UserFields.ID),
)
if err != nil {
	return err
}
_ = page
```

## CLI Configuration

Multi-package generation can be driven by a committed JSON file:

```json
{
  "databaseId": "postgres",
  "typeHandlers": ["json", "decimal"],
  "buildTags": ["enterprise"],
  "packages": [
    {
      "dir": "internal/user",
      "output": "internal/user/zz_goark_orm_user_gen.go"
    },
    {
      "dir": "internal/order"
    }
  ]
}
```

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json --check
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json --diff
```

The CLI configuration controls source scanning and file output only. It does not connect to a database and does not generate migration files.

## Runtime Configuration

`Configuration` is the direct runtime model:

```go
config := orm.DefaultConfiguration().
	WithLocalCache(true).
	WithSecondLevelCache(true).
	WithMapUnderscoreToCamelCase(true)

config.Dialect = orm.NewPostgresDialect()
config.LocalCacheScope = orm.LocalCacheScopeSession
config.DefaultExecutorType = orm.ExecutorTypeReuse
config.GlobalConfig.DbConfig.IDType = orm.IDTypeAssignID
config.GlobalConfig.DbConfig.TablePrefix = "sys_"
config.GlobalConfig.DbConfig.InsertStrategy = orm.FieldStrategyNotEmpty
config.GlobalConfig.DbConfig.UpdateStrategy = orm.FieldStrategyNotEmpty

session, err := orm.NewSQLSession(registry, db, nil, orm.WithConfiguration(config))
if err != nil {
	return err
}
```

The JSON configuration decoder is strict and uses the internal Sonic-backed JSON codec. `LoadAndAssembleMyBatisConfig` loads the file and assembles runtime objects in one step:

```go
assembled, err := orm.LoadAndAssembleMyBatisConfig("orm-runtime.json", orm.MyBatisAssembly{
	Registry: registry,
	DB:       db,
	TypeHandlers: map[string]orm.TypeHandler{
		"json": orm.NewJSONTypeHandler(),
	},
})
if err != nil {
	return err
}
session := assembled.Session
defer session.Close()
_ = session
```

## Goark Boot-Style Assembly

`goark.dev/orm/ormboot` provides a small adapter boundary for Goark-style application bootstrapping without importing Goark core packages into the ORM runtime. Applications pass a `*sql.DB`, MyBatis-style runtime config, and generated metadata registrars, then register the returned bean instances in their own container:

```go
assembler, err := ormboot.New(ormboot.Config{
	DB:            db,
	MyBatisConfig: config,
	MetadataRegistrars: []ormboot.MetadataRegistrar{
		RegisterGoarkORMMetadata,
	},
})
if err != nil {
	return err
}
runtime, err := assembler.Assemble(ctx)
if err != nil {
	return err
}
defer runtime.Close()
factory := runtime.SessionFactory()
_ = factory
```

The adapter owns only ORM sessions it creates. The caller still owns driver imports, `*sql.DB` lifecycle, and real transaction manager integration.

## Provider SQL

Providers are registered explicitly and can use the SQL builder:

```go
err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
	"UserSQL.ListByStatus",
	func(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs) (orm.SQLSource, error) {
		return orm.NewSelectSQLBuilder().
			Select("id", "name", "status").
			From("sys_user").
			WhereEq("status", args["status"]).
			OrderByAsc("id").
			Limit(args["limit"]).
			CacheKey("tenant:" + args["tenant"].(string)).
			Build()
	},
	orm.WithSQLProviderCommands(orm.StatementCommandSelect),
	orm.WithSQLProviderStatements("example.user.UserMapper.ListByStatus"),
))
if err != nil {
	return err
}
```

## Transactions And Batch

```go
factory, err := orm.NewSQLSessionFactory(registry, db, orm.NewPostgresDialect())
if err != nil {
	return err
}

err = factory.InTx(ctx, nil, func(ctx context.Context, session orm.Session) error {
	batch, err := orm.NewBatchSession(session)
	if err != nil {
		return err
	}
	baseMapper, err := NewUserBaseMapper(batch)
	if err != nil {
		return err
	}
	_, err = baseMapper.UpdateWithWrapper(
		ctx,
		orm.NewUpdateWrapper[User]().
			SetTyped(UserTypedFields.Status, "LOCKED").
			EqTyped(UserTypedFields.ID, int64(7)),
	)
	if err != nil {
		return err
	}
	_, err = batch.Flush(ctx)
	return err
})
```

## Schema Utilities

`ormgen` can inspect an existing schema through `database/sql`, build a package model, render Go source, and optionally compare the database shape with registered metadata:

```go
report, err := ormgen.ValidateSQLSchemaCompatibility(ctx, ormgen.SQLSchemaCompatibilityConfig{
	DBType:      orm.DbTypePostgres,
	SQLQueryer: db,
	Schema:     "public",
	Tables:     []string{"sys_user"},
	PackageName: "user",
	Registry:   registry,
})
if err != nil {
	return err
}
_ = report.Source
```

## Real Database Compatibility

The reusable database suite is disabled until the caller provides a driver and DSN. The standard suite currently supports PostgreSQL and MySQL only:

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

```bash
GOARK_ORM_INTEGRATION_DRIVER=postgres \
GOARK_ORM_INTEGRATION_DSN='postgres://user:pass@127.0.0.1:5432/goark?sslmode=disable' \
GOARK_ORM_INTEGRATION_DBTYPE=postgres \
GOWORK=off go test -run TestORMDatabaseCompatibility ./...
```

The standard PostgreSQL/MySQL compatibility matrix includes callable statement coverage. Details are documented in [docs/database-matrix.md](docs/database-matrix.md).

## Local Verification

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
git diff --check
```

Release maintainers can run the local gate:

```bash
GOWORK=off ./scripts/verify-release.sh
```

## More Documentation

- [Examples Guide](docs/examples.md)
- [API Compatibility](docs/api-compatibility.md)
- [Database Matrix](docs/database-matrix.md)
- [Provider And SQL Builder](docs/provider-builder.md)
- [Architecture Notes](docs/goark-orm-v1-design.md)
- [Release Gates](docs/release-gates.md)

## License

Apache License 2.0.
