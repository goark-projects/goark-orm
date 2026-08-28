# Goark ORM Configuration Reference

Goark ORM has two configuration layers:

- Generator configuration controls source scanning and generated file output.
- Runtime configuration controls sessions, dialect selection, cache behavior, type handlers, mapper validation, plugins, and global entity behavior.

Both JSON decoders are strict and route through the internal Sonic-backed JSON codec. Unknown fields fail fast.

## Generator Configuration File

Use the file with:

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json
```

Top-level fields:

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `dir` | string | `.` | Source package directory when `packages` is empty or as a shared package default. |
| `package` | string | package found in `dir` | Explicit Go package name to scan. |
| `output` | string | generated default path | Output file for a single package. It cannot be used at top level when multiple packages are configured. |
| `databaseId` | string | empty | Selects database-specific XML statements during generation. |
| `typeHandlers` | string array | empty | Adds accepted type-handler names during scan validation. Built-ins `json`, `time`, and `decimal` are always accepted by the generator. |
| `buildTags` | string array | empty | Build tags passed to the package loader. |
| `naming` | object | explicit names | Shared table and column naming rules. |
| `packages` | object array | one package from top-level fields | Per-package generation targets. |

`packages[]` fields:

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `dir` | string | top-level `dir`, then `.` | Source package directory. Relative paths are resolved from the config file directory. |
| `package` | string | top-level `package`, then package found in `dir` | Go package name to scan. |
| `output` | string | generated default path | Output file for this package. Relative paths are resolved from the config file directory. |
| `databaseId` | string | top-level `databaseId` | Database-specific XML statement selector. |
| `typeHandlers` | string array | top-level values plus local values | Accepted type-handler names for scan validation. |
| `buildTags` | string array | top-level values plus local values | Build tags for this package. |
| `naming` | object | top-level `naming` | Package-specific naming override. |

`naming` fields:

| Field | Accepted values | Default | Description |
| --- | --- | --- | --- |
| `table` | `explicit`, `same`, `snake_case`, `snake`, `underline` | `explicit` | Derives table names when `//goark-orm:entity` omits `table`. |
| `column` | `explicit`, `same`, `snake_case`, `snake`, `underline` | `explicit` | Derives column names when a field tag omits `column`. |
| `tablePrefix` | identifier prefix | empty | Prepends a prefix to derived table names when missing. |

Example:

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
      "dir": "internal/order",
      "databaseId": "mysql"
    }
  ]
}
```

## Runtime JSON Configuration File

Use the file with:

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
defer assembled.Session.Close()
```

Top-level fields:

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `properties` | object string map | empty | Named placeholders resolved in supported string settings using `${name}`. Cycles and missing names fail. |
| `settings` | object | `DefaultConfiguration` values | Runtime behavior switches. |
| `environment` | object | question-placeholder dialect | Environment id, database type, and database id. |
| `databaseIdProvider` | object | none | Optional database id derivation from dialect or db type. |
| `typeAliases` | array | empty | Declared aliases used for validation and higher-level integration. Runtime does not scan packages from aliases. |
| `typeHandlers` | array | empty | Named handlers that must be registered in `RuntimeAssembly.TypeHandlers` or be built in. |
| `mappers` | array | empty | Mapper references validated against the registry when `namespace` is set. |
| `plugins` | array | empty | Built-in or caller-provided statement interceptors. |
| `global` | object | `DefaultGlobalConfig` | Global database/entity defaults. |
| `globalConfig` | object | none | Alias for `global`; `global` and `globalConfig` cannot both be present. |

### Properties

`properties` is a string map:

```json
{
  "properties": {
    "db.type": "postgres",
    "env": "production"
  },
  "environment": {
    "id": "${env}",
    "dbType": "${db.type}"
  }
}
```

Resolution rules:

- Placeholders use `${name}`.
- Nested placeholders are resolved before use.
- Missing properties fail configuration loading.
- Circular references fail configuration loading.
- Supported fields include string settings, environment strings, database id provider strings and properties, type aliases, type handlers, mapper refs, plugin options, and global db string fields.

### Settings

| Field | Type | Default | Accepted values / behavior |
| --- | --- | --- | --- |
| `cacheEnabled` | bool | `true` | Enables namespace second-level cache behavior globally. |
| `localCacheEnabled` | bool | `true` | Enables local session cache. |
| `useColumnLabel` | bool | `true` | Uses returned column labels where supported by the result path. |
| `localCacheScope` | string | `SESSION` | `SESSION` or `STATEMENT`. |
| `mapUnderscoreToCamelCase` | bool | `false` | Enables underscore-to-camel fallback auto-mapping. |
| `useGeneratedKeys` | bool | `false` | Default generated-key preference for generated statements that opt into it. |
| `lazyLoadingEnabled` | bool | `false` | Enables explicit lazy mapping support where metadata declares it. |
| `defaultExecutorType` | string | `SIMPLE` | `SIMPLE`, `REUSE`, or `BATCH`. |
| `preparedStatementCacheSize` | int | `256` | `REUSE` prepared statement cache capacity. `0` uses the default; negative values fail. |
| `defaultStatementTimeout` | string | `0` | Go duration such as `2s` or integer seconds. Negative values fail. |
| `defaultFetchSize` | int | `0` | Statement fetch hint. Negative values fail. |
| `defaultResultSetType` | string | empty | `DEFAULT`, `FORWARD_ONLY`, `SCROLL_INSENSITIVE`, or `SCROLL_SENSITIVE`. |
| `nullableOnForEach` | bool | `true` | Default empty or nil collection behavior for dynamic `foreach`. |
| `shrinkWhitespacesInSql` | bool | `false` | Shrinks rendered dynamic SQL whitespace. |
| `jdbcTypeForNull` | string | `OTHER` | Normalized JDBC-style null type name carried as metadata. |
| `autoMappingBehavior` | string | `FULL` | `NONE`, `PARTIAL`, or `FULL`. |
| `autoMappingUnknownColumnBehavior` | string | `NONE` | `NONE`, `WARNING`, or `FAILING`. |
| `databaseId` | string | empty | Explicit statement selection id; highest priority when set. |
| `safeRowBoundsEnabled` | bool | `false` | Reserved validation-compatible setting. |
| `safeResultHandlerEnabled` | bool | `true` | Reserved validation-compatible setting. |
| `aggressiveLazyLoading` | bool | `false` | Reserved validation-compatible setting. |
| `lazyLoadTriggerMethods` | string array | `equals`, `clone`, `hashCode`, `toString` | Validated token list retained in configuration. |
| `defaultScriptingLanguage` | string | empty | Validated token retained in configuration. |
| `defaultEnumTypeHandler` | string | empty | Validated token retained in configuration. |
| `callSettersOnNulls` | bool | `false` | Reserved validation-compatible setting. |
| `returnInstanceForEmptyRow` | bool | `false` | Reserved validation-compatible setting. |
| `logPrefix` | string | empty | Retained log prefix metadata. |
| `logImpl` | string | empty | Validated token retained in configuration. Core does not instantiate loggers. |
| `proxyFactory` | string | empty | Validated token retained in configuration. Core does not create transparent proxies. |
| `vfsImpl` | string array | empty | Validated slash-capable token list retained in configuration. |
| `useActualParamName` | bool | `true` | Retained parameter naming setting. |
| `configurationFactory` | string | empty | Validated token retained in configuration. |
| `defaultSqlProviderType` | string | empty | Validated token retained in configuration. |
| `argNameBasedConstructorAutoMapping` | bool | `false` | Retained constructor auto-mapping setting. |

### Environment

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `id` | string | empty | Caller-defined environment id. |
| `dbType` | string | `question` | `question`, `postgres`, `postgresql`, `pg`, `mysql`, `mariadb`, `sqlite`, `sqlite3`, `sqlserver`, `mssql`, or `oracle`. |
| `databaseId` | string | empty | Statement selection id used after `settings.databaseId` and before `databaseIdProvider`. |

`environment.dbType` creates the runtime dialect unless `RuntimeEnvironment.Dialect` is set directly in Go.

### Database ID Provider

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `type` | string | empty | Only `vendor` is accepted. |
| `properties` | object string map | empty | Maps dialect names or product names to a database id. Matching is case-insensitive. |
| `defaultId` | string | empty | Fallback database id. |

Database id priority:

1. `settings.databaseId`
2. `environment.databaseId`
3. `databaseIdProvider`

Example:

```json
{
  "environment": {
    "dbType": "postgres"
  },
  "databaseIdProvider": {
    "type": "vendor",
    "properties": {
      "PostgreSQL": "postgres",
      "postgres": "postgres",
      "MySQL": "mysql"
    },
    "defaultId": "default"
  }
}
```

### Type Aliases

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `alias` | string | yes | Case-insensitive alias key. Duplicates are rejected. |
| `typeName` | string | yes | Fully qualified or package-local type name. |

Runtime does not scan Go packages from aliases. They are explicit declaration metadata for validation and integration layers.

### Type Handlers

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes | Handler name. Duplicates are rejected. |

Built-in runtime names are `json`, `time`, `decimal`, `string`, `bool`, and `bytes`. Custom names must be supplied in `RuntimeAssembly.TypeHandlers`.

### Mappers

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `resource` | string | one of `resource` or `namespace` | Resource metadata for generator or higher-level tooling. Runtime does not scan the file. |
| `namespace` | string | one of `resource` or `namespace` | Mapper namespace validated against the registered metadata. Duplicates are rejected. |

### Plugins

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | required | Built-in or caller-registered plugin name. Name matching ignores case, dashes, underscores, and spaces. |
| `order` | int | declaration order | Optional non-negative ordering key. Plugins with `order=0` keep declaration order after ordered plugins. |
| `enabled` | bool | `true` | Disabled plugin refs are ignored during assembly. |
| `options` | object string map | empty | Plugin-specific options. Unknown options fail. |

Built-in plugins:

| Name | Options | Behavior |
| --- | --- | --- |
| `pagination` | none | Adds page request SQL rewriting. |
| `blockAttack` | none | Rejects dangerous full-table writes. |
| `readOnly` | none | Rejects data-affecting statements. |
| `tenant` | `column`, `value` | Adds tenant predicates and insert-field values. |
| `dynamicTable` | table mapping object | Rewrites logical table names to physical names. |
| `illegalSQL` | `denySelectWildcard`, `denyMultipleStatements`, `denyWriteWithoutWhere` | Enables configured SQL governance checks. Values are booleans encoded as strings. |

Custom plugins are looked up from `RuntimeAssembly.Plugins`.

### Global Database Config

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `idType` | string | none | Global primary key strategy: `auto`, `input`, `assign_id`, `assign_uuid`, or empty. |
| `tablePrefix` | string | empty | Prefix applied to generated common CRUD table names when missing. Must be a valid identifier prefix. |
| `schema` | string | empty | Schema prefix applied when a table name is not already qualified. |
| `logicDeleteField` | string | empty | Field or column used for generated logical delete behavior. |
| `logicDeleteValue` | any | `true` | Value written for logical delete. |
| `logicNotDeleteValue` | any | `false` | Value used for active rows. |
| `insertStrategy` | string | default | Global insert field strategy. |
| `updateStrategy` | string | default | Global update field strategy. |
| `whereStrategy` | string | default | Global where field strategy. |

Field strategy values:

- `always`
- `not_null`
- `not_empty`
- `not_zero`
- `never`
- empty default

## Direct Go Configuration

Applications can skip JSON and build `Configuration` directly:

```go
config := orm.DefaultConfiguration().
	WithLocalCache(true).
	WithSecondLevelCache(true).
	WithMapUnderscoreToCamelCase(true).
	WithDefaultResultSetType(orm.ResultSetTypeForwardOnly).
	WithNullableOnForEach(true)

config.Dialect = orm.NewPostgresDialect()
config.DatabaseID = "postgres"
config.LocalCacheScope = orm.LocalCacheScopeSession
config.DefaultExecutorType = orm.ExecutorTypeReuse
config.PreparedStatementCacheSize = orm.DefaultPreparedStatementCacheSize
config.DefaultStatementTimeout = 2 * time.Second
config.DefaultFetchSize = 512
config.ShrinkWhitespacesInSQL = true
config.GlobalConfig.DbConfig.IDType = orm.IDTypeAssignID
config.GlobalConfig.DbConfig.TablePrefix = "sys_"
config.GlobalConfig.DbConfig.InsertStrategy = orm.FieldStrategyNotEmpty
config.GlobalConfig.DbConfig.UpdateStrategy = orm.FieldStrategyNotEmpty

session, err := orm.NewSQLSession(registry, db, nil, orm.WithConfiguration(config))
if err != nil {
	return err
}
```

## Runtime Assembly

`RuntimeAssembly` fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `Config` | `RuntimeConfig` | no | Runtime declaration model. |
| `Registry` | `*Registry` | yes | Explicit metadata registry. |
| `DB` | `*sql.DB` | no | Creates `SessionFactory` and default `Session` when present. |
| `TypeHandlers` | `map[string]TypeHandler` | no | Custom named handlers. |
| `Plugins` | `PluginRegistry` | no | Custom statement interceptors. |
| `SessionOptions` | `[]SQLSessionOption` | no | Extra options applied after configuration-derived options. |

`AssembleRuntimeConfig` validates configuration, registers type handlers, validates mapper namespaces, builds runtime `Configuration`, creates session options, and creates a session factory/session when `DB` is present.

`RuntimeAssemblyResult` fields:

| Field | Type | Description |
| --- | --- | --- |
| `Configuration` | `Configuration` | Normalized runtime configuration. |
| `Registry` | `*Registry` | Validated registry. |
| `SessionFactory` | `*SQLSessionFactory` | Present when `DB` is provided. |
| `Session` | `*SQLSession` | Present when `DB` is provided. Caller must close it. |

## Boot Adapter Configuration

`ormboot.Config` fields:

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `Name` | string | `goarkORM` | Assembly unit name. |
| `Order` | int | `0` | Caller-owned ordering metadata. |
| `BeanNames` | object | default bean names | Output bean names for caller-owned containers. Duplicate names fail. |
| `Registry` | `*orm.Registry` | new registry | Metadata registry. |
| `DB` | `*sql.DB` | required | Caller-owned database pool. |
| `RuntimeConfig` | `orm.RuntimeConfig` | empty | Runtime declaration model. |
| `TypeHandlers` | map | empty | Custom type handlers. |
| `Plugins` | map | empty | Custom plugins. |
| `SessionOptions` | slice | empty | Extra session options. |
| `MetadataRegistrars` | slice | empty | Generated metadata registrar functions. |

Default bean names:

| Field | Default |
| --- | --- |
| `Runtime` | `goarkORMRuntime` |
| `Registry` | `goarkORMRegistry` |
| `Configuration` | `goarkORMConfiguration` |
| `SessionFactory` | `goarkORMSessionFactory` |

The adapter returns bean registrations for the caller's container and owns only ORM sessions it creates.
