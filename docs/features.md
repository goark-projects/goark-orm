# Goark ORM Feature Reference

This document lists the implemented production surface in the current codebase. It describes Goark ORM as an independent Go data-mapping library and avoids assumptions about application frameworks, deployment platforms, or private database schemas.

## Core Runtime

| Feature | Public surface | Notes |
| --- | --- | --- |
| API identity | `ModulePath`, `APIVersion` | Module path is `goark.dev/orm`; current API version is `v1`. |
| Registry | `Registry`, `RegisterEntity`, `RegisterMapper`, `ValidateRegistry` | Metadata is registered explicitly. Validation checks mapper namespaces, statements, result maps, type handlers, providers, nested selects, and cache references. |
| Session | `Session`, `StatementSession`, `CallSession`, `SQLSession`, `ManagedSession` | Generated mapper code depends on interfaces, not concrete sessions. |
| Session factory | `SQLSessionFactory`, `InTx`, `OpenSession` | Owns ORM session creation over caller-owned `*sql.DB`. |
| Transaction session | `TxSession`, `InTx` | Publishes second-level cache mutations after commit and drops pending cache mutations on rollback. |
| Batch session | `BatchSession`, `Flush` | Queues write statements, flushes in order, and flushes before reads. |
| Routing session | `RoutingSession`, `RoutingSessionFactory`, `WithDataSource` | Supports explicit data-source selection and resolver-based routing. It does not create cross-database transactions. |
| Configuration | `Configuration`, `RuntimeConfig`, `RuntimeAssembly` | Direct Go configuration and strict JSON configuration are both supported. |
| Error classes | `ErrConfiguration`, `ErrRegistry`, `ErrBinding`, `ErrMapping`, `ErrExecutor` | Errors preserve typed classification for `errors.Is` and `errors.As`. |

## Mapper And Entity Generation

| Feature | Public surface | Notes |
| --- | --- | --- |
| Entity annotation | `//goark-orm:entity` | `table` is explicit unless generator naming derives it; `keySequence` is supported. |
| Struct tag | `goark-orm` | Tags are strict: unknown attributes, duplicate attributes, empty attributes, and unquoted string values fail generation. |
| Mapper annotation | `//goark-orm:mapper` | `namespace` is required and must be globally unique; `xml` attaches an XML mapper file. |
| Method statements | `//goark-orm:select`, `insert`, `update`, `delete`, `call` | Each method binds exactly one statement source: annotation SQL, annotation provider, or XML. |
| Generated entries | `RegisterGoarkORMMetadata`, `New<Entity>Mapper`, `New<Entity>BaseMapper`, `New<Entity>Service` | Generated names are stable within V1. |
| Row scanners | generated `RowScanner` functions | Simple entity mapping uses generated scanners first; advanced result maps use controlled fallback paths. |
| Typed fields | generated field constants and typed fields | Used by wrappers, field-value helpers, sorting, and update builders. |
| Multi-package config | `goark-orm generate orm --config` | Shared defaults and per-package overrides are supported. |
| Check mode | `--check` | Fails if generated files are stale. |
| Diff mode | `--diff` | Prints generated differences without writing files. |

## Annotation Reference

| Annotation | Scope | Attributes |
| --- | --- | --- |
| `//goark-orm:entity` | struct type | `table`, `keySequence` or `key-sequence` |
| `//goark-orm:mapper` | interface type | `namespace`, `xml` |
| `//goark-orm:select` | mapper method | `sql` or `provider`, `statementType`, `affectData`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, callable `parameters`, callable `resultSets` |
| `//goark-orm:insert` | mapper method | `sql` or `provider`, `statementType`, `useGeneratedKeys`, `keyProperty`, statement options, callable metadata when needed |
| `//goark-orm:update` | mapper method | `sql` or `provider`, `statementType`, statement options, `interceptorIgnore` |
| `//goark-orm:delete` | mapper method | `sql` or `provider`, `statementType`, statement options, `interceptorIgnore` |
| `//goark-orm:call` | mapper method | `sql` or `provider`, `statementType`, `parameters`, `resultSets`, statement options |

Rules:

- `sql` and `provider` are mutually exclusive.
- Multiple SQL annotations on one method are rejected.
- A method declared in XML must not also declare annotation SQL.
- Mapper methods require named parameters and the first parameter must be `context.Context`.
- Select methods return `(T, error)`, `([]T, error)`, `(orm.Page[T], error)`, `(*orm.Cursor[T], error)`, or use `orm.ResultHandler[T]` with `error`.
- Insert, update, and delete methods return `(int64, error)`.
- Call methods return `error` or `(orm.CallResult, error)` and validate OUT/INOUT/result-set parameters against pointer method parameters.

## Struct Tag Reference

| Attribute | Type | Effect |
| --- | --- | --- |
| `column` | string | Database column name. |
| `type` | string | Database type metadata for generated schema and compatibility checks. |
| `default` | string | Column default metadata. |
| `id-type` | string | Primary key strategy: `auto`, `input`, `assign_id`, `assign_uuid`. |
| `fill` | string | Strict fill timing: `insert`, `update`, `insert_update`. |
| `type-handler` | string | Named type handler used for this field. |
| `key-column` | string | Generated-key readback column. |
| `update` | string | Custom update expression. |
| `update-expression` | string | Alias for custom update expression; cannot be used with `update`. |
| `condition` | string | Custom condition template for wrappers. |
| `insert-strategy` | string | Field inclusion strategy for INSERT. |
| `update-strategy` | string | Field inclusion strategy for UPDATE. |
| `where-strategy` | string | Field inclusion strategy for WHERE. |
| `primary-key` | bool | Marks a primary key field. |
| `auto-increment` | bool | Marks a database-generated primary key. Requires `primary-key=true`. |
| `nullable` | bool | Column nullability metadata. |
| `select` | bool | `false` removes the field from generated default select lists. |
| `version` | bool | Enables optimistic-lock metadata. One field per entity. |
| `soft-delete` | bool | Enables logical-delete metadata. One field per entity. |
| `created-at` | bool | Marks created-at metadata. One field per entity. |
| `updated-at` | bool | Marks updated-at metadata. One field per entity. |
| `order-by` | bool | Adds field to generated default ordering metadata. |
| `order-desc` | bool | Uses descending order when `order-by=true`. |
| `transient` | bool | Excludes the field from persistence metadata. |
| `size` | int | Column size metadata. |
| `numeric-scale` | int | Decimal scale metadata. |
| `order-priority` | int | Ordering priority metadata. |

String tag values must use single quotes. Boolean values use `true` or `false`. Integer values are decimal integers.

## XML Mapper

| Element | Attributes | Notes |
| --- | --- | --- |
| `mapper` | `namespace` | Required root element. |
| `cache` | `eviction`, `size`, `flushInterval`, `readOnly`, `blocking` | Enables namespace second-level cache. |
| `cache-ref` | `namespace` | Reuses another namespace cache. |
| `sql` | `id` | Declares a reusable fragment expanded during generation. |
| `resultMap` | `id`, `type`, `extends`, `autoMapping` | Supports constructor, field, association, collection, and discriminator mapping. |
| `constructor` | none | Contains `idArg` and `arg`. |
| `id`, `result`, `idArg`, `arg` | `property`, `name`, `column`, `typeHandler` | Defines scalar result binding. |
| `association` | `property`, `type`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | Supports nested mapping, nested selects, named result sets, and lazy loading through explicit types. |
| `collection` | `property`, `ofType`, `type`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | Maps child collections. |
| `discriminator` | `column`, `type`, `typeHandler` | Selects cases by column value. |
| `case` | `value`, `resultMap`, `resultType`, `type` | Defines discriminator branch mapping. |
| `select`, `insert`, `update`, `delete`, `call` | `id`, `resultMap`, `resultType`, `parameterType`, `databaseId`, `affectData`, `useGeneratedKeys`, `keyProperty`, `useCache`, `flushCache`, `statementType`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, `resultSets` | Statement metadata. `resultMap` and `resultType` are mutually exclusive. |
| `selectKey` | `keyProperty`, `resultType`, `order` | `order` accepts `BEFORE` or `AFTER`; default is `AFTER`. |
| `parameter` | `property`, `name`, `mode`, `jdbcType`, `type`, `typeHandler` | Callable parameter metadata. |
| `resultSet` | `name`, `property`, `resultMap`, `resultType` | Callable result-set metadata. |

Dynamic SQL nodes:

- `if`: conditional child rendering through `test`.
- `where`: wraps conditions and removes leading boolean connectors.
- `set`: wraps update assignments and removes trailing commas.
- `trim`: applies `prefix`, `suffix`, `prefixOverrides`, and `suffixOverrides`.
- `foreach`: expands collections with `collection`, `item`, `index`, `open`, `close`, `separator`, and optional `nullable`.
- `choose`, `when`, `otherwise`: renders the first matching branch or fallback.
- `include`: expands a generated-time `sql` fragment by `refid` or `refId`.
- `bind`: creates a named value from a safe expression.

## Dynamic Expression Engine

Expressions are intentionally bounded and deterministic. Supported features:

- boolean logic: `and`, `or`, `not`, `&&`, `||`, `!`
- comparisons: `==`, `!=`, `>`, `>=`, `<`, `<=`, `eq`, `ne`, `neq`, `gt`, `gte`, `ge`, `lt`, `lte`, `le`
- membership: `in`, `not in`
- arithmetic: `+`, `-`, `*`, `/`, `%`
- ternary values: `condition ? trueValue : falseValue`
- literals: `nil`, `null`, booleans, quoted strings, integers, floats, list literals
- collection and string helpers: `len`, `size`, `.size`, `.length`, `.empty`, `.isEmpty()`, `.contains()`, `.containsKey()`, `.containsValue()`
- string helpers: `.startsWith()`, `.endsWith()`, `.toLowerCase()`, `.toUpperCase()`, `.trim()`, `.equals()`, `.equalsIgnoreCase()`

The expression engine cannot call arbitrary functions or mutate values.

## SQL Binding And Safety

| Placeholder | Behavior |
| --- | --- |
| `#{name}` | Bound as a driver parameter after dialect placeholder rewriting. |
| `${name}` | Rendered only when the value implements `RawSQLToken`. |

Safe raw tokens include raw identifiers and raw order-by values created through public constructors. Direct strings are rejected for raw SQL placeholders.

## Dialects

| Database | Factory | Placeholder | Notes |
| --- | --- | --- | --- |
| Question placeholder | `NewQuestionDialect` | `?` | SQL generation only. |
| PostgreSQL | `NewPostgresDialect` | `$1` | Pagination, generated-key returning, upsert, row locks, JSON capability metadata. |
| MySQL | `NewMySQLDialect` | `?` | Pagination, last-insert-id, upsert, row locks, JSON capability metadata. |
| MariaDB | `NewMariaDBDialect` | `?` | MySQL-compatible generation path. |
| SQLite | `NewSQLiteDialect` | `?` | Pagination, last-insert-id, upsert, optional JSON support by driver/extension. |
| SQL Server | `NewSQLServerDialect` | `@p1` | Adds stable fallback ordering for offset pagination when needed. |
| Oracle | `NewOracleDialect` | `:1` | Offset pagination, merge upsert, returning plan metadata. |

## Type Handlers

Built-ins:

- `json`: marshals and unmarshals through the internal Sonic-backed JSON codec.
- `time`: converts common string, byte, and `time.Time` values.
- `decimal`: stores strings, byte slices, and `fmt.Stringer` values without adding a decimal dependency.
- `string`: converts values to strings.
- `bool`: accepts bools, common textual booleans, and numeric values.
- `bytes`: clones byte slices and converts strings to bytes.

Custom handlers implement `TypeHandler` or use `NewTypeHandler`.

## Interceptors, Middleware, And Audit

Built-in statement interceptors include:

- pagination
- block-attack protection
- read-only protection
- tenant field injection
- data permission conditions
- dynamic table name rewriting
- illegal SQL checks
- SQL observation
- entity semantic rewriting
- SQL governance rules

Middleware extension points cover statement execution, statement handling, parameter handling, and result-set handling. The optional `goark.dev/orm/audit` package provides write and `affectData` select auditing.

## Cache

Local cache defaults to session scope. Statement scope is available for one-statement reuse only. Second-level cache is namespace-scoped and supports LRU eviction, TTL, cache references, blocking miss coalescing, stats, and transaction-aware publication.

## Schema Tools

`ormgen` includes SQL schema introspection, reverse engineering, custom template rendering, schema drift detection, and schema compatibility validation. These helpers use caller-owned `database/sql` connections and do not manage migrations.

## Real Database Suites

`ormtest` provides reusable compatibility and benchmark harnesses. Concrete drivers are imported only by caller-owned tests or by temporary modules created by scripts. The standard matrix currently covers PostgreSQL, MySQL, MariaDB, SQLite, SQL Server, and Oracle.
