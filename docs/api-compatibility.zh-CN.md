# Goark ORM V1 API 兼容性

## 状态

`goark.dev/orm` 通过 `orm.APIVersion` 暴露当前公共 API 版本。当前值为 `v1`。

## 稳定范围

V1 兼容性范围包括：

- Module path：`goark.dev/orm`。
- 运行期包：`goark.dev/orm`。
- 可选审计包：`goark.dev/orm/audit`。
- 可选 DbKit 便捷包：`goark.dev/orm/dbkit`。
- 启动装配适配包：`goark.dev/orm/ormboot`。
- 生成器包：`goark.dev/orm/ormgen`。
- 测试辅助包：`goark.dev/orm/ormtest`。
- CLI 入口：`goark-orm generate orm`。
- 注解前缀：`//goark-orm:entity`、`//goark-orm:mapper`、`//goark-orm:select`、`//goark-orm:insert`、`//goark-orm:update`、`//goark-orm:delete`、`//goark-orm:call`。
- Struct tag key：`goark-orm`。

## 运行期契约

以下导出运行期概念受 V1 兼容性策略保护：

- `Session`、`ManagedSession`、`StatementSession`、`CallSession`、`StatementCallSession`。
- `SQLSession`、`SQLSessionFactory`、`TxSession`、`BatchSession`、`RoutingSession`、`RoutingSessionFactory`。
- `Configuration`、`GlobalConfig`、`DbConfig`、`RuntimeSettings`、`RuntimeEnvironment`、`RuntimeConfig`、`RuntimeConfigFile`、`RuntimeSettingsFile`、`RuntimeEnvironmentFile`、`RuntimeAssembly`、`RuntimeAssemblyResult`。
- `ormboot.Config`、`ormboot.Assembler`、`ormboot.Runtime`、`ormboot.MetadataRegistrar`、`ormboot.BeanNames`、`ormboot.BeanRegistration`。
- `BaseMapper`、`Service`、`QueryChain`、`UpdateChain`、`QueryWrapper`、`UpdateWrapper`、类型化字段值查询辅助函数、`Page`、`PageRequest`、`Cursor`、`Lazy`、`LazySlice`。
- `SQLInjector`、`SQLInjectorFunc`、`DefaultSQLInjector`、`InjectOption`、`InjectNamespaceResolver`、`RegisterInjectedStatements`、`RegisterDefaultInjectedStatementsForRegistry`。
- `Dialect`、`DialectCapabilities`、`DbType`、`UpsertSpec`、`RowLockOptions`、`GeneratedKeyPlan`。
- `TypeHandler`、`RowScanner`、`IdentifierGenerator`、`MetaObjectHandler`、`EnumValuer`。
- `StatementInterceptor`、`StatementHandler`、`ParameterHandler`、`ResultSetHandler` 及其 middleware 类型。
- `Cache`、`CacheStatsProvider`、`SQLProvider`、`SQLProviderDescriptor` 和 SQL builder 类型。
- `ResultSetType`、`ParseResultSetType`、`AutoMappingBehavior`、`ParseAutoMappingBehavior`、`AutoMappingUnknownColumnBehavior`、`ParseAutoMappingUnknownColumnBehavior`。
- 结构化错误值和上下文类型，例如 `ErrConfiguration`、`ErrRegistry`、`ErrBinding`、`ErrMapping`、`ErrExecutor`。
- `goark.dev/orm/audit` 中的审计契约：`Event`、`Operation`、`Recorder`、`RecorderFunc`、`NewMiddleware`、`Option`、`WithQueryEvents`、`WithErrorEvents`、`WithIgnoreRecorderError`、`WithSkipFunc`。

## 元数据契约

稳定元数据模型包括：

- `EntityMeta`、`ColumnMeta`、`MapperMeta`、`StatementMeta`、`StatementOptions`。
- `ParameterMeta`、`ResultSetMeta`、`ResultMapMeta`、构造器映射、association 映射、collection 映射、discriminator 映射、cache 元数据和动态 SQL 节点。
- `StatementMeta.AffectData`：用于会修改数据的 select 风格语句，这类语句必须使用类似写操作的缓存和审计默认值。
- `StatementOptions.ResultSetType`、`StatementOptions.ResultOrdered`、`StatementOptions.KeyColumn`。
- `ResultAssociationMeta.ResultSet`、`ResultAssociationMeta.ForeignColumn`、`ResultCollectionMeta.ResultSet`、`ResultCollectionMeta.ForeignColumn`：用于命名多结果集嵌套对象映射。
- 配置和文件设置：`DefaultResultSetType`、`UseColumnLabel`、`NullableOnForEach`、`ShrinkWhitespacesInSQL`、`JDBCTypeForNull`、`AutoMappingBehavior`、`AutoMappingUnknownColumnBehavior`。
- Statement command、source、type、cache policy、parameter mode、result set type、field strategy、field fill、ID type 枚举。

## 生成器契约

V1 生成器范围包括：

- `GenerateSpec`、`GenerateConfig`、`GeneratePackageSpec`、`ConfiguredGenerateSpec`、`PackageModel`、`EntityModel`、`MapperModel`、`StatementModel` 和渲染入口。
- 命名契约：`NamingConfig`、`NamingStrategyExplicit`、`NamingStrategySame`、`NamingStrategySnakeCase`。
- Schema introspection 契约：`SchemaIntrospector`、`SQLSchemaIntrospector`、`SQLSchemaDialect`、`SchemaQueryer`、`SQLQueryer`。
- 反向工程契约：`ReverseEngineerSpec`、`ReverseEngineer`、`BuildPackageModelFromSchema`、`TemplateRenderer`、`ReverseEngineerWithRenderer`。
- Drift 和兼容性辅助函数：`DetectSchemaDrift`、`ValidateSchemaDrift`、`CompareSchemaDrift`、`ValidateSQLSchemaCompatibility`、`SQLSchemaCompatibilityConfig`、`SQLSchemaCompatibilityReport`。

## 测试辅助契约

`ormtest` 让真实数据库验证可复用，同时不把具体驱动链接进 core module。稳定范围包括：

- 基础套件契约：`DatabaseSuiteConfig`、`DatabaseCase`、`EnvSuiteOption`、`WithEnvPrefix`、`WithEnvRegistry`、`WithEnvCases`、`WithEnvSessionOptions`、`RunDatabaseSuite`、`RunDatabaseSuiteFromEnv`、`LoadDatabaseSuiteConfigFromEnv`、`ParseSQLList`。
- 可复用 case：`PingCase`、`QueryStatementCase`、`QueryOneStatementCase`、`ExecStatementCase`、`PageStatementCase`、`CallStatementCase`。
- 标准兼容性矩阵契约：`DefaultCompatibilityTable`、`CompatibilityRecord`、`CompatibilityProfile`、`CompatibilitySuiteOption`、`WithCompatibilityTable`、`WithCompatibilityNamespace`、`WithCompatibilityEnvPrefix`、`NewCompatibilitySuiteConfig`、`RunCompatibilitySuiteFromEnv`、`SupportedCompatibilityDBTypes`、`IsCompatibilityDBTypeSupported`。
- 标准 benchmark 矩阵契约：`DefaultBenchmarkTable`、`DatabaseBenchmarkConfig`、`DatabaseBenchmarkCase`、`DatabaseBenchmarkScope`、`NewBenchmarkSuiteConfig`、`RunDatabaseBenchmark`、`RunDatabaseBenchmarkFromEnv`、`LoadDatabaseBenchmarkConfigFromEnv`、`SupportedBenchmarkDBTypes`、`IsBenchmarkDBTypeSupported`。
- `NewCompatibilitySuiteConfig` 返回 `DatabaseSuiteConfig`；兼容性 cases 是普通 `DatabaseCase` 值。
- 环境变量解析辅助函数和 SQL 列表解析行为见 [database-matrix.zh-CN.md](database-matrix.zh-CN.md)。

## 演进规则

- 允许新增能力，前提是零值保持现有行为。
- V1 内保留既有导出标识符、函数签名、接口方法集和错误分类语义。
- 生成入口名称保持稳定，包括 `RegisterGoarkORMMetadata`、`New<Entity>Mapper`、`New<Entity>BaseMapper`、`New<Entity>Service`。
- Core packages 不得新增具体数据库驱动导入。
- Core packages 不得新增对 Goark core、boot 或 CLI 包的依赖。
- 运行期 Mapper 扫描、运行期 XML 扫描、运行期实体建模和 migration 生成仍在 V1 core 边界之外。

## 兼容性门禁

仓库保留外部 package API 契约测试：

- `api_contract_external_test.go` 覆盖运行期 API。
- `audit/middleware_test.go` 覆盖可选审计 middleware API 和行为。
- `ormgen/api_contract_external_test.go` 覆盖生成器 API。
- `ormtest/api_contract_external_test.go` 覆盖真实数据库测试辅助 API。

本地验证：

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
git diff --check
```

发布门禁见 [release-gates.zh-CN.md](release-gates.zh-CN.md)。
