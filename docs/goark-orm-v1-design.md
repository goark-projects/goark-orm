# Goark ORM V1 Architecture Notes

## Status

Accepted for the current V1 implementation.

## Context

Goark ORM is an independent data mapping module. The runtime uses deterministic generated metadata and explicit registration. Source files, XML mapper files, and struct tags are generator inputs; they are not scanned by the runtime.

## Goals

- Keep mapper namespaces explicit and globally unique inside a registry.
- Support annotation SQL, XML SQL, provider SQL, generated common CRUD, batch execution, streaming query, callable statements, transactions, routing sessions, and cache behavior through one metadata model.
- Keep the hot path low-reflection. Generated row scanners are preferred when registered; controlled fallback paths handle advanced result maps and type-handler cases.
- Keep database driver registration outside core packages.
- Keep schema lifecycle outside the ORM boundary.
- Keep Go APIs explicit, small, and testable.

## Non-Goals

- No runtime mapper scanning.
- No runtime XML scanning.
- No runtime entity modeling.
- No transparent lazy-loading proxies.
- No persistent context with implicit dirty checking.
- No migration generation or DDL lifecycle management.
- No concrete database driver imports in core packages.
- No dependency from `goark.dev/orm` core to Goark core, boot, or CLI packages.

## Data Flow

```text
goark-orm generate orm
        |
        |-- scan //goark-orm:entity and goark-orm struct tags
        |-- scan //goark-orm:mapper and method SQL annotations
        |-- parse XML mapper files
        |-- validate mapper, entity, statement, result map, and parameter contracts
        v
zz_goark_orm_<package>_gen.go
        |
        |-- RegisterGoarkORMMetadata
        |-- EntityMeta / MapperMeta / StatementMeta
        |-- generated RowScanner functions
        |-- generated Mapper implementations
        |-- BaseMapper and Service factories
        v
Registry
        |
        v
Session / Executor / Dialect / TypeHandler / Interceptor / Cache
        |
        v
database/sql
```

## Annotation And Tag Contracts

Annotations use the `//goark-orm:xxx` prefix:

```go
//goark-orm:mapper(namespace="example.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface{}
```

Entity fields use the Go struct tag key `goark-orm`:

```go
ID int64 `goark-orm:"column='id';primary-key=true;auto-increment=true"`
```

Rules:

- Mapper namespace is required.
- Mapper namespace must be explicitly declared.
- Mapper namespace must be unique in the registry.
- Entity table name is required.
- Persistent fields require a `column` tag.
- Non-persistent fields require `transient=true`.
- A mapped entity needs at least one primary key for generated common CRUD.
- SQL method annotations are mutually exclusive: `select`, `insert`, `update`, `delete`, or `call`.
- A method must not be bound by both XML and annotation SQL.

## Runtime Packages

```text
dialect        SQL placeholders, identifier quoting, pagination, capability metadata
metadata       Entity, mapper, statement, result map, cache, and dynamic SQL metadata
statement      SQL source, provider, raw token, dynamic SQL, and parameter binding
executor       query, query-one, exec, callable, batch, and result mapping
session        SQLSession, factory, transaction session, routing session, and caches
typehandler    JSON, time, decimal, bool, bytes, and custom conversion SPI
interceptor    pagination, tenant, data permission, dynamic table, guard, read-only, observer
ormgen         source scanning, XML parsing, model validation, rendering, schema tools
ormtest        caller-owned real database test suites
```

## Runtime Configuration

`Configuration` is the direct runtime configuration model. It controls dialect, database id, cache defaults, local cache scope, underscore-to-camel mapping, default executor type, default statement timeout, fetch size, and global entity behavior.

The JSON configuration loader is strict and decodes through the internal Sonic-backed JSON codec. `AssembleMyBatisConfig` keeps assembly explicit: callers pass the registry, optional database handle, and named type handlers. It validates the configuration, type handler names, mapper namespaces, and registry metadata before creating a session factory.

## Transactions

`SQLSessionFactory` creates auto-commit sessions and transaction sessions over `database/sql`. Generated mappers only require `orm.Session`, so transaction sessions and batch sessions can be used without generating separate mapper code.

```go
err := factory.InTx(ctx, nil, func(ctx context.Context, session orm.Session) error {
	baseMapper, err := NewUserBaseMapper(session)
	if err != nil {
		return err
	}
	_, err = baseMapper.UpdateWithWrapper(
		ctx,
		orm.NewUpdateWrapper[User]().
			SetTyped(UserTypedFields.Status, "LOCKED").
			EqTyped(UserTypedFields.ID, int64(7)),
	)
	return err
})
```

`BatchSession` queues write statements, flushes them in order, and automatically flushes before reads.

## Caching

Local cache is scoped to a session by default and is invalidated on writes, commit, rollback, and close. Statement scope is available for one-statement caching only.

Second-level cache is namespace scoped. It supports LRU eviction, TTL expiration, blocking miss coalescing, cache references, and stats snapshots. Transaction sessions publish cache writes and invalidations only after commit; rollback drops pending cache changes.

## Result Mapping

Generated row scanners are used first for simple entity mappings. Result maps cover constructor mapping, associations, collections, discriminator cases, nested selects, column prefixes, not-null guards, and explicit lazy loading. Advanced result maps use controlled fallback paths because they need runtime aggregation or nested query behavior.

## Interceptors And Middleware

`StatementInterceptor` wraps SQL after dynamic SQL rendering and before dialect placeholder compilation. Built-in interceptors include:

- block-attack protection
- SQL observation
- tenant condition and insert-field injection
- data permission condition injection
- dynamic table name rewriting
- pagination
- entity semantic rewriting
- SQL governance rules
- illegal SQL protection
- read-only protection

Middleware contracts allow decorator-style extension for statement execution, statement handling, parameter handling, and result set handling.

## Routing

`RoutingSession` delegates generated mapper calls to named sessions. Resolution order is:

1. explicit `WithDataSource(ctx, key)`
2. configured `DataSourceResolver`
3. default data source

The routing layer does not create cross-database transaction semantics. Atomic work must use a single data source transaction or an external transaction coordinator.

## Schema Tools

`ormgen.SQLSchemaIntrospector` reads database metadata through an already registered `database/sql` connection. Reverse engineering builds package models and renders Go source. Drift helpers compare a registry's entity metadata with a schema model. `ValidateSQLSchemaCompatibility` chains introspection, model build, render smoke, and optional drift validation.

## Key Decisions

| Decision | Result |
| --- | --- |
| Mapper namespace | Explicit and globally unique |
| Runtime metadata | Generated and explicitly registered |
| Runtime discovery | No mapper, XML, or entity scanning |
| Database drivers | Imported only by callers or test harnesses |
| Migrations | Outside ORM core |
| Raw SQL substitution | Restricted to explicit safe tokens |
| JSON codec | Internal codec backed by ByteDance Sonic |
| Goark ecosystem adapter | Not part of core runtime |

## Verification

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
git diff --check
```
