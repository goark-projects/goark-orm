# Goark ORM 配置参考

Goark ORM 有两层配置：

- 生成器配置控制源码扫描和生成文件输出。
- 运行期配置控制 Session、方言选择、缓存行为、TypeHandler、Mapper 校验、插件和全局实体行为。

两个 JSON 解码器都是严格模式，并统一经过内部 Sonic-backed JSON codec。未知字段会直接失败。

## 生成器配置文件

使用方式：

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json
```

顶层字段：

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `dir` | string | `.` | `packages` 为空时的源码 package 目录，也可作为共享默认值。 |
| `package` | string | 从 `dir` 发现 | 显式 Go package 名称。 |
| `output` | string | 生成器默认路径 | 单 package 输出文件。多 package 配置时不能在顶层使用。 |
| `databaseId` | string | empty | 生成期选择 database-specific XML statement。 |
| `typeHandlers` | string array | empty | 增加扫描校验可接受的 TypeHandler 名称。生成器始终接受内置 `json`、`time`、`decimal`。 |
| `buildTags` | string array | empty | 传给 package loader 的 build tags。 |
| `naming` | object | 显式名称 | 共享表名和列名推导规则。 |
| `packages` | object array | 用顶层字段生成一个 package | 每个 package 的生成目标。 |

`packages[]` 字段：

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `dir` | string | 顶层 `dir`，再 fallback 到 `.` | 源码 package 目录。相对路径基于配置文件所在目录解析。 |
| `package` | string | 顶层 `package`，再从 `dir` 发现 | 要扫描的 Go package 名称。 |
| `output` | string | 生成器默认路径 | 当前 package 输出文件。相对路径基于配置文件所在目录解析。 |
| `databaseId` | string | 顶层 `databaseId` | Database-specific XML statement selector。 |
| `typeHandlers` | string array | 顶层值加局部值 | 扫描校验可接受的 TypeHandler 名称。 |
| `buildTags` | string array | 顶层值加局部值 | 当前 package 的 build tags。 |
| `naming` | object | 顶层 `naming` | 当前 package 的命名覆盖。 |

`naming` 字段：

| 字段 | 可选值 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `table` | `explicit`, `same`, `snake_case`, `snake`, `underline` | `explicit` | 当 `//goark-orm:entity` 省略 `table` 时推导表名。 |
| `column` | `explicit`, `same`, `snake_case`, `snake`, `underline` | `explicit` | 当字段 tag 省略 `column` 时推导列名。 |
| `tablePrefix` | identifier prefix | empty | 推导表名缺少前缀时追加前缀。 |

示例：

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

## 运行期 JSON 配置文件

使用方式：

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

顶层字段：

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `properties` | object string map | empty | 使用 `${name}` 在支持的字符串配置中解析占位符。循环和缺失名称会失败。 |
| `settings` | object | `DefaultConfiguration` 值 | 运行期行为开关。 |
| `environment` | object | question-placeholder dialect | 环境 id、数据库类型和 database id。 |
| `databaseIdProvider` | object | none | 可选的 database id 推导规则。 |
| `typeAliases` | array | empty | 用于校验和上层集成的显式别名。运行期不会根据别名扫描 package。 |
| `typeHandlers` | array | empty | 必须在 `RuntimeAssembly.TypeHandlers` 或内置处理器中存在的命名 handler。 |
| `mappers` | array | empty | 设置 `namespace` 时会对 Registry 中的 Mapper 做校验。 |
| `plugins` | array | empty | 内置或调用方提供的 statement interceptor。 |
| `global` | object | `DefaultGlobalConfig` | 全局数据库/实体默认值。 |
| `globalConfig` | object | none | `global` 的别名；不能和 `global` 同时出现。 |

### Properties

`properties` 是 string map：

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

解析规则：

- 占位符格式是 `${name}`。
- 嵌套占位符会先解析再使用。
- 缺失 property 会导致配置加载失败。
- 循环引用会导致配置加载失败。
- 支持字段包括字符串 settings、environment 字符串、database id provider 字符串和 properties、type aliases、type handlers、mapper refs、plugin options、global db 字符串字段。

### Settings

| 字段 | 类型 | 默认值 | 可选值 / 行为 |
| --- | --- | --- | --- |
| `cacheEnabled` | bool | `true` | 全局启用 namespace 二级缓存行为。 |
| `localCacheEnabled` | bool | `true` | 启用 Session local cache。 |
| `useColumnLabel` | bool | `true` | 在结果路径支持时使用返回列 label。 |
| `localCacheScope` | string | `SESSION` | `SESSION` 或 `STATEMENT`。 |
| `mapUnderscoreToCamelCase` | bool | `false` | 启用 underscore-to-camel fallback auto-mapping。 |
| `useGeneratedKeys` | bool | `false` | 对显式 opt-in 的生成 statement 使用生成主键偏好。 |
| `lazyLoadingEnabled` | bool | `false` | 在元数据声明处启用显式 lazy mapping 支持。 |
| `defaultExecutorType` | string | `SIMPLE` | `SIMPLE`, `REUSE`, `BATCH`。 |
| `preparedStatementCacheSize` | int | `256` | `REUSE` 预编译语句缓存容量。`0` 使用默认值，负数失败。 |
| `defaultStatementTimeout` | string | `0` | Go duration，例如 `2s`，或整数秒。负数失败。 |
| `defaultFetchSize` | int | `0` | Statement fetch hint。负数失败。 |
| `defaultResultSetType` | string | empty | `DEFAULT`, `FORWARD_ONLY`, `SCROLL_INSENSITIVE`, `SCROLL_SENSITIVE`。 |
| `nullableOnForEach` | bool | `true` | Dynamic `foreach` 对空集合或 nil 集合的默认行为。 |
| `shrinkWhitespacesInSql` | bool | `false` | 压缩动态 SQL 渲染后的空白。 |
| `jdbcTypeForNull` | string | `OTHER` | 作为元数据保留的 null 类型名称。 |
| `autoMappingBehavior` | string | `FULL` | `NONE`, `PARTIAL`, `FULL`。 |
| `autoMappingUnknownColumnBehavior` | string | `NONE` | `NONE`, `WARNING`, `FAILING`。 |
| `databaseId` | string | empty | 显式 statement selection id，优先级最高。 |
| `safeRowBoundsEnabled` | bool | `false` | 预留的校验兼容配置。 |
| `safeResultHandlerEnabled` | bool | `true` | 预留的校验兼容配置。 |
| `aggressiveLazyLoading` | bool | `false` | 预留的校验兼容配置。 |
| `lazyLoadTriggerMethods` | string array | `equals`, `clone`, `hashCode`, `toString` | 经校验的 token 列表，保留在配置中。 |
| `defaultScriptingLanguage` | string | empty | 经校验的 token，保留在配置中。 |
| `defaultEnumTypeHandler` | string | empty | 经校验的 token，保留在配置中。 |
| `callSettersOnNulls` | bool | `false` | 预留的校验兼容配置。 |
| `returnInstanceForEmptyRow` | bool | `false` | 预留的校验兼容配置。 |
| `logPrefix` | string | empty | 保留 log prefix metadata。 |
| `logImpl` | string | empty | 经校验的 token，保留在配置中。core 不创建 logger。 |
| `proxyFactory` | string | empty | 经校验的 token，保留在配置中。core 不创建透明代理。 |
| `vfsImpl` | string array | empty | 经校验且允许 slash 的 token 列表，保留在配置中。 |
| `useActualParamName` | bool | `true` | 保留参数命名配置。 |
| `configurationFactory` | string | empty | 经校验的 token，保留在配置中。 |
| `defaultSqlProviderType` | string | empty | 经校验的 token，保留在配置中。 |
| `argNameBasedConstructorAutoMapping` | bool | `false` | 保留 constructor auto-mapping 配置。 |

### Environment

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `id` | string | empty | 调用方定义的环境 id。 |
| `dbType` | string | `question` | `question`, `postgres`, `postgresql`, `pg`, `mysql`, `mariadb`, `sqlite`, `sqlite3`, `sqlserver`, `mssql`, `oracle`。 |
| `databaseId` | string | empty | statement selection id，优先级低于 `settings.databaseId`，高于 `databaseIdProvider`。 |

`environment.dbType` 会创建运行期方言；如果在 Go 代码中显式设置了 `RuntimeEnvironment.Dialect`，则以显式方言为准。

### Database ID Provider

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `type` | string | empty | 只接受 `vendor`。 |
| `properties` | object string map | empty | 将 dialect name 或产品名映射到 database id，匹配大小写不敏感。 |
| `defaultId` | string | empty | fallback database id。 |

Database id 优先级：

1. `settings.databaseId`
2. `environment.databaseId`
3. `databaseIdProvider`

示例：

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

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `alias` | string | yes | 大小写不敏感的 alias key。重复会失败。 |
| `typeName` | string | yes | 全限定或 package-local 类型名。 |

运行期不会根据 alias 扫描 Go package。它们是用于校验和集成层的显式声明元数据。

### Type Handlers

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | yes | Handler 名称。重复会失败。 |

内置运行期名称是 `json`、`time`、`decimal`、`string`、`bool` 和 `bytes`。自定义名称必须通过 `RuntimeAssembly.TypeHandlers` 提供。

### Mappers

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `resource` | string | `resource` 或 `namespace` 二选一 | 供生成器或上层工具使用的资源元数据。运行期不扫描文件。 |
| `namespace` | string | `resource` 或 `namespace` 二选一 | 根据已注册元数据校验 Mapper namespace。重复会失败。 |

### Plugins

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `name` | string | required | 内置或调用方注册的 plugin 名称。匹配时忽略大小写、短横线、下划线和空格。 |
| `order` | int | 声明顺序 | 可选非负排序值。`order=0` 的插件在有序插件之后保留声明顺序。 |
| `enabled` | bool | `true` | 禁用的 plugin ref 在装配时跳过。 |
| `options` | object string map | empty | Plugin 特定选项。未知选项会失败。 |

内置插件：

| 名称 | 选项 | 行为 |
| --- | --- | --- |
| `pagination` | none | 添加分页 SQL 改写。 |
| `blockAttack` | none | 拒绝危险全表写语句。 |
| `readOnly` | none | 拒绝会影响数据的语句。 |
| `tenant` | `column`, `value` | 添加租户谓词和 insert 字段值。 |
| `dynamicTable` | table mapping object | 将逻辑表名改写为物理表名。 |
| `illegalSQL` | `denySelectWildcard`, `denyMultipleStatements`, `denyWriteWithoutWhere` | 启用对应 SQL 治理检查。值是字符串编码的布尔值。 |

自定义 plugin 从 `RuntimeAssembly.Plugins` 查找。

### Global Database Config

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `idType` | string | none | 全局主键策略：`auto`, `input`, `assign_id`, `assign_uuid` 或空。 |
| `tablePrefix` | string | empty | 生成通用 CRUD 表名缺少前缀时追加。必须是有效 identifier prefix。 |
| `schema` | string | empty | 表名未带 schema 时追加 schema 前缀。 |
| `logicDeleteField` | string | empty | 生成逻辑删除行为使用的字段或列。 |
| `logicDeleteValue` | any | `true` | 逻辑删除写入值。 |
| `logicNotDeleteValue` | any | `false` | 有效行使用值。 |
| `insertStrategy` | string | default | 全局 INSERT 字段策略。 |
| `updateStrategy` | string | default | 全局 UPDATE 字段策略。 |
| `whereStrategy` | string | default | 全局 WHERE 字段策略。 |

字段策略值：

- `always`
- `not_null`
- `not_empty`
- `not_zero`
- `never`
- 空默认值

## Go 代码直接配置

应用可以跳过 JSON，直接构造 `Configuration`：

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

## 运行期装配

`RuntimeAssembly` 字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `Config` | `RuntimeConfig` | no | 运行期声明模型。 |
| `Registry` | `*Registry` | yes | 显式元数据 registry。 |
| `DB` | `*sql.DB` | no | 存在时创建 `SessionFactory` 和默认 `Session`。 |
| `TypeHandlers` | `map[string]TypeHandler` | no | 自定义命名 handler。 |
| `Plugins` | `PluginRegistry` | no | 自定义 statement interceptor。 |
| `SessionOptions` | `[]SQLSessionOption` | no | 配置派生 option 之后追加的额外 option。 |

`AssembleRuntimeConfig` 会校验配置、注册 TypeHandler、校验 Mapper namespace、构建运行期 `Configuration`、创建 session options，并在提供 `DB` 时创建 session factory/session。

`RuntimeAssemblyResult` 字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `Configuration` | `Configuration` | 规范化后的运行期配置。 |
| `Registry` | `*Registry` | 已校验的 registry。 |
| `SessionFactory` | `*SQLSessionFactory` | 提供 `DB` 时存在。 |
| `Session` | `*SQLSession` | 提供 `DB` 时存在。调用方需要关闭。 |

## Boot Adapter 配置

`ormboot.Config` 字段：

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `Name` | string | `goarkORM` | 装配单元名称。 |
| `Order` | int | `0` | 调用方拥有的排序元数据。 |
| `BeanNames` | object | 默认 bean names | 输出给调用方容器的 bean 名称。重复会失败。 |
| `Registry` | `*orm.Registry` | new registry | 元数据 registry。 |
| `DB` | `*sql.DB` | required | 调用方拥有的数据库连接池。 |
| `RuntimeConfig` | `orm.RuntimeConfig` | empty | 运行期声明模型。 |
| `TypeHandlers` | map | empty | 自定义 TypeHandler。 |
| `Plugins` | map | empty | 自定义 plugin。 |
| `SessionOptions` | slice | empty | 额外 session options。 |
| `MetadataRegistrars` | slice | empty | 生成元数据注册函数。 |

默认 bean names：

| 字段 | 默认值 |
| --- | --- |
| `Runtime` | `goarkORMRuntime` |
| `Registry` | `goarkORMRegistry` |
| `Configuration` | `goarkORMConfiguration` |
| `SessionFactory` | `goarkORMSessionFactory` |

适配器返回可交给调用方容器注册的 bean registrations，只管理自己创建的 ORM Session。
