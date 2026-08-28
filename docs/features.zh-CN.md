# Goark ORM 功能参考

本文列出当前代码库已经实现的生产级公共能力。Goark ORM 是独立的 Go 数据映射库，不假设应用框架、部署平台或私有数据库 schema。

## Core Runtime

| 功能 | 公共接口 | 说明 |
| --- | --- | --- |
| API 标识 | `ModulePath`, `APIVersion` | 模块路径是 `goark.dev/orm`；当前 API 版本是 `v1`。 |
| Registry | `Registry`, `RegisterEntity`, `RegisterMapper`, `ValidateRegistry` | 元数据显式注册。校验 Mapper namespace、statement、result map、type handler、provider、nested select 和 cache ref。 |
| Session | `Session`, `StatementSession`, `CallSession`, `SQLSession`, `ManagedSession` | 生成 Mapper 依赖接口，不依赖具体 Session。 |
| Session Factory | `SQLSessionFactory`, `InTx`, `OpenSession` | 在调用方拥有的 `*sql.DB` 上创建 ORM Session。 |
| Transaction Session | `TxSession`, `InTx` | 二级缓存变更在 commit 后发布，rollback 丢弃待发布变更。 |
| Batch Session | `BatchSession`, `Flush` | 写语句按顺序入队并刷新，读之前自动刷新。 |
| Routing Session | `RoutingSession`, `RoutingSessionFactory`, `WithDataSource` | 支持显式数据源选择和 resolver 路由；不创建跨库事务。 |
| Configuration | `Configuration`, `RuntimeConfig`, `RuntimeAssembly` | 同时支持 Go 代码配置和严格 JSON 配置。 |
| Error Classes | `ErrConfiguration`, `ErrRegistry`, `ErrBinding`, `ErrMapping`, `ErrExecutor` | 错误保留类型分类，支持 `errors.Is` 和 `errors.As`。 |

## Mapper 与实体生成

| 功能 | 公共接口 | 说明 |
| --- | --- | --- |
| Entity 注解 | `//goark-orm:entity` | `table` 可显式声明，也可由生成器命名策略推导；支持 `keySequence`。 |
| Struct Tag | `goark-orm` | 严格 tag：未知属性、重复属性、空属性、未加单引号的字符串都会导致生成失败。 |
| Mapper 注解 | `//goark-orm:mapper` | `namespace` 必填且全局唯一；`xml` 绑定 XML Mapper 文件。 |
| Method Statement | `//goark-orm:select`, `insert`, `update`, `delete`, `call` | 每个方法只能绑定一个 statement 来源：注解 SQL、注解 provider 或 XML。 |
| Generated Entries | `RegisterGoarkORMMetadata`, `New<Entity>Mapper`, `New<Entity>BaseMapper`, `New<Entity>Service` | V1 内生成名称稳定。 |
| Row Scanner | 生成 `RowScanner` 函数 | 简单实体映射优先走生成 scanner；复杂 result map 走受控 fallback。 |
| Typed Fields | 生成字段常量和 typed fields | 用于 wrapper、字段值查询、排序和更新构造。 |
| Multi-Package Config | `goark-orm generate orm --config` | 支持共享默认值和 package 局部覆盖。 |
| Check Mode | `--check` | 生成文件过期时失败。 |
| Diff Mode | `--diff` | 打印生成差异，不写文件。 |

## 注解参考

| 注解 | 作用域 | 属性 |
| --- | --- | --- |
| `//goark-orm:entity` | struct 类型 | `table`, `keySequence` 或 `key-sequence` |
| `//goark-orm:mapper` | interface 类型 | `namespace`, `xml` |
| `//goark-orm:select` | mapper 方法 | `sql` 或 `provider`, `statementType`, `affectData`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, callable `parameters`, callable `resultSets` |
| `//goark-orm:insert` | mapper 方法 | `sql` 或 `provider`, `statementType`, `useGeneratedKeys`, `keyProperty`, statement options, callable metadata |
| `//goark-orm:update` | mapper 方法 | `sql` 或 `provider`, `statementType`, statement options, `interceptorIgnore` |
| `//goark-orm:delete` | mapper 方法 | `sql` 或 `provider`, `statementType`, statement options, `interceptorIgnore` |
| `//goark-orm:call` | mapper 方法 | `sql` 或 `provider`, `statementType`, `parameters`, `resultSets`, statement options |

规则：

- `sql` 和 `provider` 互斥。
- 同一方法多个 SQL 注解会失败。
- XML 已声明的方法不能再声明注解 SQL。
- Mapper 方法必须具名参数，且第一个参数必须是 `context.Context`。
- Select 方法返回 `(T, error)`、`([]T, error)`、`(orm.Page[T], error)`、`(*orm.Cursor[T], error)`，或使用 `orm.ResultHandler[T]` 并返回 `error`。
- Insert、update、delete 方法返回 `(int64, error)`。
- Call 方法返回 `error` 或 `(orm.CallResult, error)`，并校验 OUT/INOUT/result-set 参数是否匹配指针参数。

## Struct Tag 参考

| 属性 | 类型 | 作用 |
| --- | --- | --- |
| `column` | string | 数据库列名。 |
| `type` | string | 数据库类型元数据，用于生成 schema 和兼容性检查。 |
| `default` | string | 列默认值元数据。 |
| `id-type` | string | 主键策略：`auto`, `input`, `assign_id`, `assign_uuid`。 |
| `fill` | string | 严格填充时机：`insert`, `update`, `insert_update`。 |
| `type-handler` | string | 字段使用的命名 TypeHandler。 |
| `key-column` | string | 生成主键回读列。 |
| `update` | string | 自定义 update 表达式。 |
| `update-expression` | string | 自定义 update 表达式别名；不能和 `update` 同时使用。 |
| `condition` | string | Wrapper 自定义条件模板。 |
| `insert-strategy` | string | INSERT 字段参与策略。 |
| `update-strategy` | string | UPDATE 字段参与策略。 |
| `where-strategy` | string | WHERE 字段参与策略。 |
| `primary-key` | bool | 标记主键字段。 |
| `auto-increment` | bool | 标记数据库生成主键；要求 `primary-key=true`。 |
| `nullable` | bool | 列可空元数据。 |
| `select` | bool | `false` 表示从默认 select 列表排除该字段。 |
| `version` | bool | 启用乐观锁元数据。每个实体最多一个。 |
| `soft-delete` | bool | 启用逻辑删除元数据。每个实体最多一个。 |
| `created-at` | bool | 标记创建时间元数据。每个实体最多一个。 |
| `updated-at` | bool | 标记更新时间元数据。每个实体最多一个。 |
| `order-by` | bool | 加入默认排序元数据。 |
| `order-desc` | bool | `order-by=true` 时使用降序。 |
| `transient` | bool | 从持久化元数据中排除字段。 |
| `size` | int | 列长度元数据。 |
| `numeric-scale` | int | Decimal scale 元数据。 |
| `order-priority` | int | 排序优先级元数据。 |

字符串 tag 值必须使用单引号。布尔值使用 `true` 或 `false`。整数值使用十进制整数。

## XML Mapper

| 元素 | 属性 | 说明 |
| --- | --- | --- |
| `mapper` | `namespace` | 必填根元素。 |
| `cache` | `eviction`, `size`, `flushInterval`, `readOnly`, `blocking` | 启用 namespace 二级缓存。 |
| `cache-ref` | `namespace` | 复用其他 namespace 的缓存。 |
| `sql` | `id` | 声明可复用片段，生成期展开。 |
| `resultMap` | `id`, `type`, `extends`, `autoMapping` | 支持 constructor、field、association、collection、discriminator 映射。 |
| `constructor` | none | 包含 `idArg` 和 `arg`。 |
| `id`, `result`, `idArg`, `arg` | `property`, `name`, `column`, `typeHandler` | 定义标量结果绑定。 |
| `association` | `property`, `type`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | 支持嵌套映射、nested select、命名 result set 和显式懒加载类型。 |
| `collection` | `property`, `ofType`, `type`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | 映射子集合。 |
| `discriminator` | `column`, `type`, `typeHandler` | 按列值选择分支。 |
| `case` | `value`, `resultMap`, `resultType`, `type` | 定义 discriminator 分支映射。 |
| `select`, `insert`, `update`, `delete`, `call` | `id`, `resultMap`, `resultType`, `parameterType`, `databaseId`, `affectData`, `useGeneratedKeys`, `keyProperty`, `useCache`, `flushCache`, `statementType`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, `resultSets` | Statement 元数据。`resultMap` 和 `resultType` 互斥。 |
| `selectKey` | `keyProperty`, `resultType`, `order` | `order` 支持 `BEFORE` 或 `AFTER`；默认 `AFTER`。 |
| `parameter` | `property`, `name`, `mode`, `jdbcType`, `type`, `typeHandler` | Callable 参数元数据。 |
| `resultSet` | `name`, `property`, `resultMap`, `resultType` | Callable result-set 元数据。 |

动态 SQL 节点：

- `if`：通过 `test` 条件渲染子节点。
- `where`：包装条件并移除前置布尔连接符。
- `set`：包装更新赋值并移除尾部逗号。
- `trim`：应用 `prefix`、`suffix`、`prefixOverrides` 和 `suffixOverrides`。
- `foreach`：通过 `collection`、`item`、`index`、`open`、`close`、`separator` 和可选 `nullable` 展开集合。
- `choose`、`when`、`otherwise`：渲染第一个命中分支或 fallback。
- `include`：按 `refid` 或 `refId` 展开生成期 `sql` 片段。
- `bind`：通过安全表达式创建命名值。

## 动态表达式引擎

表达式引擎是有界且确定性的。支持：

- 布尔逻辑：`and`, `or`, `not`, `&&`, `||`, `!`
- 比较：`==`, `!=`, `>`, `>=`, `<`, `<=`, `eq`, `ne`, `neq`, `gt`, `gte`, `ge`, `lt`, `lte`, `le`
- 集合包含：`in`, `not in`
- 算术：`+`, `-`, `*`, `/`, `%`
- 三元值：`condition ? trueValue : falseValue`
- 字面量：`nil`, `null`, 布尔值、引号字符串、整数、浮点数、列表字面量
- 集合和字符串 helper：`len`, `size`, `.size`, `.length`, `.empty`, `.isEmpty()`, `.contains()`, `.containsKey()`, `.containsValue()`
- 字符串 helper：`.startsWith()`, `.endsWith()`, `.toLowerCase()`, `.toUpperCase()`, `.trim()`, `.equals()`, `.equalsIgnoreCase()`

表达式引擎不能调用任意函数，也不能修改值。

## SQL 绑定与安全

| 占位符 | 行为 |
| --- | --- |
| `#{name}` | 方言占位符改写后作为 driver 参数绑定。 |
| `${name}` | 仅当值实现 `RawSQLToken` 时渲染。 |

安全 raw token 包括通过公共构造器创建的 raw identifier 和 raw order-by 值。普通字符串不能进入 raw SQL 占位符。

## 方言

| 数据库 | Factory | 占位符 | 说明 |
| --- | --- | --- | --- |
| Question placeholder | `NewQuestionDialect` | `?` | 只作为 SQL 生成方言。 |
| PostgreSQL | `NewPostgresDialect` | `$1` | 分页、生成主键 returning、upsert、行锁、JSON capability metadata。 |
| MySQL | `NewMySQLDialect` | `?` | 分页、last-insert-id、upsert、行锁、JSON capability metadata。 |
| MariaDB | `NewMariaDBDialect` | `?` | MySQL-compatible 生成路径。 |
| SQLite | `NewSQLiteDialect` | `?` | 分页、last-insert-id、upsert，JSON 取决于 driver/extension。 |
| SQL Server | `NewSQLServerDialect` | `@p1` | offset pagination 缺少 order 时追加稳定 fallback ordering。 |
| Oracle | `NewOracleDialect` | `:1` | offset pagination、merge upsert、returning plan metadata。 |

## TypeHandler

内置处理器：

- `json`：通过内部 Sonic-backed JSON codec marshal/unmarshal。
- `time`：转换常见 string、byte 和 `time.Time` 值。
- `decimal`：存储 string、byte slice 和 `fmt.Stringer` 值，不增加 decimal 依赖。
- `string`：把值转成 string。
- `bool`：接受 bool、常见文本布尔值和数字值。
- `bytes`：克隆 byte slice，并把 string 转为 bytes。

自定义处理器实现 `TypeHandler`，或使用 `NewTypeHandler`。

## 拦截器、Middleware 与审计

内置 statement interceptor 包括：

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

Middleware 扩展点覆盖 statement execution、statement handling、parameter handling 和 result-set handling。可选 `goark.dev/orm/audit` 包提供写语句和 `affectData` 查询审计。

## 缓存

Local cache 默认是 session scope。Statement scope 只用于单语句复用。二级缓存以 namespace 为作用域，支持 LRU、TTL、cache ref、并发 miss 合并、统计和事务感知发布。

## Schema 工具

`ormgen` 包含 SQL schema introspection、反向工程、自定义模板渲染、schema drift detection 和 schema compatibility validation。这些 helper 使用调用方拥有的 `database/sql` 连接，不管理 migration。

## 真实数据库套件

`ormtest` 提供可复用兼容性和 benchmark harness。具体驱动只由调用方测试或脚本创建的临时模块导入。标准矩阵当前覆盖 PostgreSQL、MySQL、MariaDB、SQLite、SQL Server 和 Oracle。
