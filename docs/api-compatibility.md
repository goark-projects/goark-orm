# Goark ORM V1 公共 API 兼容策略

## 状态

`goark.dev/orm` 当前公共 API 主版本为 `v1`，运行时通过 `orm.APIVersion` 暴露。

## 稳定范围

以下内容进入 V1 兼容范围：

- `goark.dev/orm` module path。
- `goark.dev/orm/ormtest` 真实数据库测试辅助包；该包仅依赖 `database/sql` 和 core ORM，不引入具体数据库驱动。
- `Session`、`ManagedSession`、`StatementSession`、`CallSession`、`StatementCallSession`、`SQLSession`、`SQLSessionFactory`、`TxSession`、`BatchSession`、`BaseMapper`、`Service`、`QueryWrapper`、`UpdateWrapper`、`Page`、`Lazy`、`Cache`、`TypeHandler`、`RowScanner`、`StatementInterceptor`、`StatementHandler`、`ParameterHandler`、`ResultSetHandler`、`IdentifierGenerator`、`MetaObjectHandler`、`DialectCapabilities`、`SQLProviderDescriptor`、`SelectSQLBuilder`、`InsertSQLBuilder`、`UpdateSQLBuilder`、`DeleteSQLBuilder`、`UpsertSpec`、`RowLockOptions`、`GeneratedKeyPlan` 等导出运行时接口和类型。
- `CallResult`、`ResultSetRows`、`RowScannerRow`、`RowScannerFunc`、`SQLSource`、`EntityMeta`、`ColumnMeta`、`MapperMeta`、`StatementMeta`、`StatementOptions`、`StatementType`、`ResultSetType`、`ParameterMode`、`ParameterMeta`、`ResultSetMeta`、`ResultMapMeta`、`DynamicSQLNode` 等导出运行时和元数据结构。
- `ormgen.GenerateSpec`、`ormgen.PackageModel`、`ormgen.EntityModel`、`ormgen.MapperModel`、`ormgen.StatementModel`、`ormgen.SchemaIntrospector`、`ormgen.SchemaNamingStrategy`、`ormgen.SchemaColumnFilter`、`ormgen.SchemaColumnOverride`、`ormgen.SQLSchemaIntrospector`、`ormgen.SQLSchemaDialect`、`ormgen.TemplateRenderer`、`ormgen.ReverseEngineerWithRenderer` 和 `goark-orm generate orm` 主命令。
- `//goark-orm:entity`、`//goark-orm:mapper`、`//goark-orm:select`、`//goark-orm:insert`、`//goark-orm:update`、`//goark-orm:delete`、`//goark-orm:call` 注解前缀和 `goark-orm` struct tag key。

## 演进规则

- 只做向后兼容扩展：新增导出类型、方法、可选字段、可选配置和新适配层。
- 不删除 V1 已导出标识符。
- 不改变 V1 已有函数签名、接口方法签名和错误分类语义。
- 不改变已生成代码的主要入口名称，包括 `RegisterGoarkORMMetadata`、`New<Entity>Mapper`、`New<Entity>BaseMapper` 和 `New<Entity>Service`。
- 不把 Goark core、boot、CLI 或具体数据库驱动加入 `goark.dev/orm` core 的强依赖。

## 允许变化

- 新增 struct 字段，但必须保持零值安全。
- 新增可选配置，但默认值必须保持既有行为。
- 新增错误上下文字段，但 `errors.Is` 和 `errors.As` 分类语义不能破坏。
- 新增生成代码内容，但已存在入口和语义保持兼容。

## 兼容门禁

- `api_contract_external_test.go` 从外部包视角编译运行时公共 API，覆盖缓存 SPI、Wrapper 类型安全 helper、SQL token、拦截器、中间件、BaseMapper、Service、事务和配置解析入口。
- `ormgen/api_contract_external_test.go` 从外部包视角编译生成器公共 API，覆盖模型、反向工程、schema drift、模板渲染和 schema SQL 方言入口。
- `ormtest/api_contract_external_test.go` 从外部包视角编译真实数据库兼容套件 API，覆盖环境变量加载、SQL 列表解析、标准兼容矩阵和可复用 DatabaseCase 构造器。
- `scripts/verify-release.sh` 本地发布门禁执行 Go 格式检查、示例生成一致性检查、`go test -count=1 ./...`、`go vet ./...`、`git diff --check` 和固定 `-benchtime=100x` 的核心 benchmark smoke，保证公共 API、示例和 benchmark 持续可编译和可运行。

## 非兼容变化处理

确实需要破坏 V1 契约时，必须进入新的主版本或新增并行 API，不能直接修改 V1 行为。
