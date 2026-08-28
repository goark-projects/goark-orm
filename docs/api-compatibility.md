# Goark ORM V1 API Compatibility

## Status

`goark.dev/orm` exposes the current public API version through `orm.APIVersion`. The current value is `v1`.

## Stable Surface

The V1 compatibility surface includes:

- Module path: `goark.dev/orm`.
- Public runtime facade package: `goark.dev/orm`.
- Optional audit package: `goark.dev/orm/audit`.
- Optional DbKit convenience package: `goark.dev/orm/dbkit`.
- Boot-style adapter package: `goark.dev/orm/ormboot`.
- Generator package: `goark.dev/orm/ormgen`.
- Test helper package: `goark.dev/orm/ormtest`.
- Internal runtime implementation package: `goark.dev/orm/internal/runtime`; this is not part of the public compatibility surface.
- CLI entrypoint: `goark-orm generate orm`.
- Annotation prefixes: `//goark-orm:entity`, `//goark-orm:mapper`, `//goark-orm:select`, `//goark-orm:insert`, `//goark-orm:update`, `//goark-orm:delete`, and `//goark-orm:call`.
- Struct tag key: `goark-orm`.

## Runtime Contracts

The following exported runtime concepts are covered by the V1 compatibility policy:

- `Session`, `ManagedSession`, `StatementSession`, `CallSession`, and `StatementCallSession`.
- `SQLSession`, `SQLSessionFactory`, `TxSession`, `BatchSession`, `RoutingSession`, and `RoutingSessionFactory`.
- `Configuration`, `GlobalConfig`, `DbConfig`, `RuntimeSettings`, `RuntimeEnvironment`, `RuntimeConfig`, `RuntimeConfigFile`, `RuntimeSettingsFile`, `RuntimeEnvironmentFile`, `RuntimeAssembly`, and `RuntimeAssemblyResult`.
- `ormboot.Config`, `ormboot.Assembler`, `ormboot.Runtime`, `ormboot.MetadataRegistrar`, `ormboot.BeanNames`, and `ormboot.BeanRegistration`.
- `BaseMapper`, `Service`, `QueryChain`, `UpdateChain`, `QueryWrapper`, `UpdateWrapper`, typed field value query helpers, `Page`, `PageRequest`, `Cursor`, `Lazy`, and `LazySlice`.
- `SQLInjector`, `SQLInjectorFunc`, `DefaultSQLInjector`, `InjectOption`, `InjectNamespaceResolver`, `RegisterInjectedStatements`, and `RegisterDefaultInjectedStatementsForRegistry`.
- `Dialect`, `DialectCapabilities`, `DbType`, `UpsertSpec`, `RowLockOptions`, and `GeneratedKeyPlan`.
- `TypeHandler`, `RowScanner`, `IdentifierGenerator`, `MetaObjectHandler`, and `EnumValuer`.
- `StatementInterceptor`, `StatementHandler`, `ParameterHandler`, `ResultSetHandler`, and their middleware types.
- `Cache`, `CacheStatsProvider`, `SQLProvider`, `SQLProviderDescriptor`, and the SQL builder types.
- `ResultSetType`, `ParseResultSetType`, `AutoMappingBehavior`, `ParseAutoMappingBehavior`, `AutoMappingUnknownColumnBehavior`, and `ParseAutoMappingUnknownColumnBehavior`.
- Structured error values and context types such as `ErrConfiguration`, `ErrRegistry`, `ErrBinding`, `ErrMapping`, and `ErrExecutor`.
- Audit contracts in `goark.dev/orm/audit`: `Event`, `Operation`, `Recorder`, `RecorderFunc`, `NewMiddleware`, `Option`, `WithQueryEvents`, `WithErrorEvents`, `WithIgnoreRecorderError`, and `WithSkipFunc`.

## Metadata Contracts

The stable metadata model includes:

- `EntityMeta`, `ColumnMeta`, `MapperMeta`, `StatementMeta`, and `StatementOptions`.
- `ParameterMeta`, `ResultSetMeta`, `ResultMapMeta`, constructor mapping, association mapping, collection mapping, discriminator mapping, cache metadata, and dynamic SQL nodes.
- `StatementMeta.AffectData` for select-style statements that modify data and must use write-like cache and audit defaults.
- `StatementOptions.ResultSetType`, `StatementOptions.ResultOrdered`, and `StatementOptions.KeyColumn`.
- `ResultAssociationMeta.ResultSet`, `ResultAssociationMeta.ForeignColumn`, `ResultCollectionMeta.ResultSet`, and `ResultCollectionMeta.ForeignColumn` for named multi-result-set nested object mapping.
- Configuration and file settings: `DefaultResultSetType`, `UseColumnLabel`, `NullableOnForEach`, `ShrinkWhitespacesInSQL`, `JDBCTypeForNull`, `AutoMappingBehavior`, and `AutoMappingUnknownColumnBehavior`.
- Statement command, source, type, cache policy, parameter mode, result set type, field strategy, field fill, and ID type enums.

## Generator Contracts

The V1 generator surface includes:

- `GenerateSpec`, `GenerateConfig`, `GeneratePackageSpec`, `ConfiguredGenerateSpec`, `PackageModel`, `EntityModel`, `MapperModel`, `StatementModel`, and rendering entrypoints.
- Naming contracts: `NamingConfig`, `NamingStrategyExplicit`, `NamingStrategySame`, and `NamingStrategySnakeCase`.
- Schema introspection contracts: `SchemaIntrospector`, `SQLSchemaIntrospector`, `SQLSchemaDialect`, `SchemaQueryer`, and `SQLQueryer`.
- Reverse engineering contracts: `ReverseEngineerSpec`, `ReverseEngineer`, `BuildPackageModelFromSchema`, `TemplateRenderer`, and `ReverseEngineerWithRenderer`.
- Drift and compatibility helpers: `DetectSchemaDrift`, `ValidateSchemaDrift`, `CompareSchemaDrift`, `ValidateSQLSchemaCompatibility`, `SQLSchemaCompatibilityConfig`, and `SQLSchemaCompatibilityReport`.

## Test Helper Contracts

`ormtest` keeps real database verification reusable without linking concrete drivers into the core module. Its stable surface includes:

- Base suite contracts: `DatabaseSuiteConfig`, `DatabaseCase`, `EnvSuiteOption`, `WithEnvPrefix`, `WithEnvRegistry`, `WithEnvCases`, `WithEnvSessionOptions`, `RunDatabaseSuite`, `RunDatabaseSuiteFromEnv`, `LoadDatabaseSuiteConfigFromEnv`, and `ParseSQLList`.
- Reusable cases: `PingCase`, `QueryStatementCase`, `QueryOneStatementCase`, `ExecStatementCase`, `PageStatementCase`, and `CallStatementCase`.
- Standard compatibility matrix contracts: `DefaultCompatibilityTable`, `CompatibilityRecord`, `CompatibilityProfile`, `CompatibilitySuiteOption`, `WithCompatibilityTable`, `WithCompatibilityNamespace`, `WithCompatibilityEnvPrefix`, `NewCompatibilitySuiteConfig`, `RunCompatibilitySuiteFromEnv`, `SupportedCompatibilityDBTypes`, and `IsCompatibilityDBTypeSupported`.
- Standard benchmark matrix contracts: `DefaultBenchmarkTable`, `DatabaseBenchmarkConfig`, `DatabaseBenchmarkCase`, `DatabaseBenchmarkScope`, `NewBenchmarkSuiteConfig`, `RunDatabaseBenchmark`, `RunDatabaseBenchmarkFromEnv`, `LoadDatabaseBenchmarkConfigFromEnv`, `SupportedBenchmarkDBTypes`, and `IsBenchmarkDBTypeSupported`.
- `NewCompatibilitySuiteConfig` returns `DatabaseSuiteConfig`; compatibility cases are ordinary `DatabaseCase` values.
- Environment parsing helpers and SQL list parsing behavior documented in [database-matrix.md](database-matrix.md).

## Evolution Rules

- Additive changes are allowed when zero values keep existing behavior.
- Existing exported identifiers, function signatures, interface method sets, and error classification semantics are preserved within V1.
- Generated entry names remain stable, including `RegisterGoarkORMMetadata`, `New<Entity>Mapper`, `New<Entity>BaseMapper`, and `New<Entity>Service`.
- Core packages must not add concrete database driver imports.
- Core packages must not add dependencies on Goark core, boot, or CLI packages.
- Runtime mapper scanning, runtime XML scanning, runtime entity modeling, and migration generation remain outside the V1 core boundary.

## Compatibility Gates

The repository keeps external-package API contract tests:

- `api_contract_external_test.go` covers runtime APIs.
- `audit/middleware_test.go` covers the optional audit middleware API and behavior.
- `ormgen/api_contract_external_test.go` covers generator APIs.
- `ormtest/api_contract_external_test.go` covers real database test helper APIs.

Local verification:

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
git diff --check
```

The release gate is documented in [release-gates.md](release-gates.md).
