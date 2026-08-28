# Goark ORM

Goark ORM is a Go-native data mapper for `database/sql`. It uses explicit metadata, deterministic generated code, small runtime contracts, and reusable real-database verification tools for production Go services.

English is the default documentation language. Chinese documentation is available in [README.zh-CN.md](README.zh-CN.md); each long-form guide and example README also has a `*.zh-CN.md` mirror.

## Module

```text
module goark.dev/orm
```

`orm.APIVersion` currently reports `v1`.

## Design Rules

- Runtime metadata is registered explicitly through generated code.
- Source files, XML mapper files, and struct tags are generator inputs; runtime code does not scan them.
- Generated mapper implementations depend on `orm.Session`, so auto-commit sessions, transaction sessions, batch sessions, routing sessions, and streaming signatures share the same generated surface.
- Core packages do not import concrete database drivers. Applications and test harnesses own driver imports, DSNs, connection pools, schema setup, and cleanup.
- Schema migrations and DDL lifecycle are outside this module.
- Raw SQL substitution through `${}` accepts only explicit `RawSQLToken` values such as `RawIdentifier` and `RawOrderBy`.
- JSON handling is routed through the internal Sonic-backed codec.

## Capability Map

| Area | Current capability |
| --- | --- |
| Entity mapping | `//goark-orm:entity`, strict `goark-orm` struct tags, typed field constants, generated row scanners |
| Mapper mapping | `//goark-orm:mapper`, SQL method annotations, XML mapper files, provider statements |
| Dynamic SQL | `sql/include`, `bind`, `if`, `where`, `set`, `trim`, `foreach`, `choose/when/otherwise`, safe expression evaluation |
| CRUD helpers | `BaseMapper`, `Service`, chain query/update APIs, typed wrappers, pagination, field-value queries, ID lists |
| Write semantics | Batch writes, generated keys, insert/update/where field strategies, automatic fill, optimistic locking, logical delete |
| Result mapping | Constructor args, associations, collections, discriminator branches, nested selects, named result sets, explicit lazy loading |
| Runtime extension | Type handlers, SQL providers, interceptors, handler middleware, audit middleware, cache SPI |
| Governance | Block-attack protection, illegal SQL rules, read-only sessions, tenant constraints, data permissions, dynamic table names, SQL observation |
| Caching | Session local cache, namespace second-level cache, LRU, TTL, blocking miss coalescing, transaction-aware publication |
| Routing | Explicit data-source selection, read/write split, statement-based routing |
| Dialects | PostgreSQL, MySQL, MariaDB, SQLite, SQL Server, Oracle, plus a question-placeholder SQL generation dialect |
| Tooling | CLI generation, multi-package generator config, schema introspection, reverse engineering, drift checks, real database suites and benchmarks |

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

Register metadata and use a session:

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

names, err := orm.SelectFieldValues(
	ctx,
	baseMapper,
	UserTypedFields.Name,
	orm.NewQueryWrapper[User]().EqTyped(UserTypedFields.Status, "ACTIVE"),
)
if err != nil {
	return err
}
_ = names
```

## Generator Configuration

Generation can be driven by a committed JSON file. The decoder is strict and uses the internal Sonic-backed JSON codec.

```json
{
  "databaseId": "postgres",
  "typeHandlers": ["json", "decimal"],
  "buildTags": ["enterprise"],
  "naming": {
    "table": "snake_case",
    "column": "snake_case",
    "tablePrefix": "sys_"
  },
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

Generator configuration controls source scanning and file output only. It does not connect to a database and does not generate migrations.

## Runtime Configuration

`Configuration` is the direct runtime model. Use `RuntimeConfig` when loading JSON configuration:

```json
{
  "settings": {
    "cacheEnabled": true,
    "localCacheEnabled": true,
    "localCacheScope": "SESSION",
    "mapUnderscoreToCamelCase": true,
    "defaultExecutorType": "REUSE",
    "preparedStatementCacheSize": 256,
    "defaultStatementTimeout": "2s",
    "defaultFetchSize": 512,
    "defaultResultSetType": "FORWARD_ONLY",
    "nullableOnForEach": true,
    "shrinkWhitespacesInSql": true,
    "jdbcTypeForNull": "OTHER",
    "autoMappingBehavior": "FULL",
    "autoMappingUnknownColumnBehavior": "NONE",
    "databaseId": "postgres"
  },
  "environment": {
    "id": "production",
    "dbType": "postgres"
  },
  "global": {
    "dbConfig": {
      "idType": "assign_id",
      "tablePrefix": "sys_",
      "logicDeleteField": "Deleted",
      "logicDeleteValue": true,
      "logicNotDeleteValue": false,
      "insertStrategy": "not_empty",
      "updateStrategy": "not_null",
      "whereStrategy": "not_zero"
    }
  },
  "typeHandlers": [
    {
      "name": "json"
    }
  ],
  "mappers": [
    {
      "namespace": "example.user.UserMapper"
    }
  ],
  "plugins": [
    {
      "name": "blockAttack",
      "order": 10
    }
  ]
}
```

```go
assembled, err := orm.LoadAndAssembleRuntimeConfig("orm-runtime.json", orm.RuntimeAssembly{
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

Full generator and runtime configuration details are in [docs/configuration.md](docs/configuration.md).

## Boot-Style Assembly

`goark.dev/orm/ormboot` provides a small adapter boundary for applications that use a container or boot lifecycle. The adapter does not make the ORM runtime depend on a framework package.

```go
assembler, err := ormboot.New(ormboot.Config{
	DB:            db,
	RuntimeConfig: config,
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

The adapter owns only ORM sessions it creates. The caller still owns driver imports, `*sql.DB` lifecycle, and transaction manager integration.

## Real Database Verification

The reusable database suite is disabled until the caller provides a driver and DSN. The standard suite currently supports PostgreSQL, MySQL, MariaDB, SQLite, SQL Server, and Oracle.

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
# Set GOARK_ORM_INTEGRATION_DSN outside the repository before running.
GOARK_ORM_INTEGRATION_DRIVER=postgres \
GOARK_ORM_INTEGRATION_DBTYPE=postgres \
GOWORK=off go test -run TestORMDatabaseCompatibility ./...
```

The matrix covers CRUD, pagination, batch execution, TypeHandler round trips, native JSON columns, UPSERT, generated-key readback, row-lock smoke paths, and callable statement paths where portable driver behavior exists. Details are in [docs/database-matrix.md](docs/database-matrix.md).

## Local Verification

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
powershell -ExecutionPolicy Bypass -File scripts/verify-bench.ps1 -EnforceTime
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db-bench.ps1 -BenchTime 1s
git diff --check
```

Release maintainers can run the local release gate:

```bash
GOWORK=off ./scripts/verify-release.sh
```

## Documentation

- [Documentation Index](docs/README.md)
- [Feature Reference](docs/features.md)
- [Configuration Reference](docs/configuration.md)
- [Annotation, Tag, And XML Mapper Reference](docs/annotations.md)
- [Examples Guide](docs/examples.md)
- [Production Demo](docs/production-demo.md)
- [API Compatibility](docs/api-compatibility.md)
- [Database Matrix](docs/database-matrix.md)
- [Provider And SQL Builder](docs/provider-builder.md)
- [Architecture Notes](docs/goark-orm-v1-design.md)
- [Release Gates](docs/release-gates.md)
- [Changelog](CHANGELOG.md)
- [Example Workspace](examples/README.md)

## License

Apache License 2.0.
