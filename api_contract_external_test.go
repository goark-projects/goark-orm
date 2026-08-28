package orm_test

import (
	"context"
	"database/sql"
	"io"
	"testing"
	"time"

	orm "goark.dev/orm"
)

type contractUser struct {
	ID     int64
	Name   string
	Status string
}

func TestV1RuntimePublicAPIContract_shouldCompileExternalUsage(t *testing.T) {
	ctx := context.Background()

	if orm.ModulePath != "goark.dev/orm" || orm.APIVersion != "v1" {
		t.Fatalf("unexpected API identity module=%q version=%q", orm.ModulePath, orm.APIVersion)
	}

	memoryCache := orm.NewMemoryCache("contract.UserMapper", orm.WithMemoryCacheMaxEntries(16), orm.WithMemoryCacheTTL(time.Minute))
	blockingCache := orm.NewBlockingCache(memoryCache)
	var _ orm.Cache = memoryCache
	var _ orm.Cache = blockingCache
	var _ orm.CacheStatsProvider = memoryCache
	var _ orm.CacheStatsProvider = blockingCache
	var _ orm.CacheMissReleaser = blockingCache
	_ = memoryCache.Stats().Hits

	registry := orm.NewRegistry()
	if err := orm.ValidateRegistry(orm.NewRegistry()); err != nil {
		t.Fatalf("validate empty registry failed: %v", err)
	}
	if err := orm.NewRegistry().Validate(); err != nil {
		t.Fatalf("validate empty registry method failed: %v", err)
	}
	handler := orm.NewTypeHandler(
		func(context.Context, any) (any, error) { return nil, nil },
		func(context.Context, any, any) error { return nil },
	)
	if err := registry.RegisterTypeHandlers(map[string]orm.TypeHandler{
		"contract": handler,
		"json":     orm.NewJSONTypeHandler(),
		"time":     orm.NewTimeTypeHandler(),
		"decimal":  orm.NewDecimalTypeHandler(),
		"string":   orm.NewStringTypeHandler(),
		"bool":     orm.NewBoolTypeHandler(),
		"bytes":    orm.NewBytesTypeHandler(),
	}); err != nil {
		t.Fatalf("register type handlers failed: %v", err)
	}
	if err := registry.RegisterCache("contract.UserMapper", blockingCache); err != nil {
		t.Fatalf("register cache failed: %v", err)
	}
	provider := func(context.Context, orm.StatementMeta, orm.NamedArgs) (orm.SQLSource, error) {
		return orm.SQLSource{SQL: "select id from sys_user"}, nil
	}
	if err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
		"contract.UserSQL.List",
		provider,
		orm.WithSQLProviderCommands(orm.StatementCommandSelect),
		orm.WithSQLProviderStatements("contract.UserMapper.List"),
	)); err != nil {
		t.Fatalf("register provider descriptor failed: %v", err)
	}
	if descriptor, ok := registry.SQLProviderDescriptor("contract.UserSQL.List"); !ok || descriptor.Provider == nil {
		t.Fatalf("expected provider descriptor")
	}

	idField := orm.NewTypedField[contractUser, int64]("id")
	nameField := orm.NewTypedField[contractUser, string]("name")
	statusField := orm.NewTypedField[contractUser, string]("status")
	query := orm.EqTypedValue(orm.NewQueryWrapper[contractUser](), idField, int64(1))
	query = orm.LikeRightTypedValue(query, nameField, "Ali")
	query = orm.InTypedValues(query, idField, int64(1), int64(2))
	query = orm.BetweenTypedValues(query, idField, int64(1), int64(9))
	query = query.SelectTyped(idField, nameField, statusField).OrderByDescTyped(idField)
	update := orm.SetTypedValue(orm.NewUpdateWrapper[contractUser](), nameField, "Alice")
	update = orm.SetIncrByTypedValue(update, idField, int64(1))
	update = orm.EqTypedValue(update, statusField, "ACTIVE")
	_ = query
	_ = update

	table, err := orm.NewRawIdentifier("sys_user")
	if err != nil {
		t.Fatalf("new raw identifier failed: %v", err)
	}
	order, err := orm.NewRawOrderItem("id", true)
	if err != nil {
		t.Fatalf("new raw order item failed: %v", err)
	}
	compiled, err := orm.CompileSQL(
		"select * from ${table} where id = #{id} order by ${order}",
		orm.NamedArgs{"table": table, "order": orm.NewRawOrderBy(order), "id": int64(1)},
		orm.NewPostgresDialect(),
	)
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	if compiled.SQL != `select * from "sys_user" where id = $1 order by "id" DESC` {
		t.Fatalf("unexpected compiled SQL %q", compiled.SQL)
	}
	source, err := orm.NewSelectSQLBuilder().
		Select("id", "name").
		From("sys_user").
		LeftJoin("sys_role", "sys_role.user_id = sys_user.id", nil).
		WhereEq("status", "ACTIVE").
		WhereIn("kind", "admin", "operator").
		WhereBetween("id", int64(1), int64(9)).
		WhereIsNotNull("name").
		OrderByAsc("id").
		ForUpdate(orm.NewPostgresDialect(), orm.RowLockOptions{}).
		CacheKey("contract").
		Build()
	if err != nil {
		t.Fatalf("build provider SQL source failed: %v", err)
	}
	if source.CacheKey != "contract" || len(source.Args) == 0 {
		t.Fatalf("unexpected SQL source %#v", source)
	}
	if _, err := orm.NewInsertSQLBuilder().Into("sys_user").Value("name", "Alice").Build(); err != nil {
		t.Fatalf("build insert source failed: %v", err)
	}
	if _, err := orm.NewMultiRowInsertSQLBuilder().
		Into("sys_user").
		Columns("id", "name").
		Rows(
			orm.NamedArgs{"id": int64(1), "name": "Alice"},
			orm.NamedArgs{"id": int64(2), "name": "Bob"},
		).
		Build(orm.NewPostgresDialect()); err != nil {
		t.Fatalf("build multi-row insert source failed: %v", err)
	}
	if _, err := orm.NewInsertSQLBuilder().Into("sys_user").Value("name", "Alice").Returning("id").Build(); err != nil {
		t.Fatalf("build insert returning source failed: %v", err)
	}
	if _, err := orm.NewUpdateSQLBuilder().Table("sys_user").Set("name", "Alice").WhereEq("id", int64(1)).Returning("id").RequireWhere().Build(); err != nil {
		t.Fatalf("build update source failed: %v", err)
	}
	if _, err := orm.NewDeleteSQLBuilder().From("sys_user").WhereIsNull("deleted_at").Returning("id").RequireWhere().Build(); err != nil {
		t.Fatalf("build delete source failed: %v", err)
	}
	upsert, err := orm.BuildUpsertSQL(orm.NewPostgresDialect(), orm.UpsertSpec{
		Table:           "sys_user",
		InsertColumns:   []string{"id", "name"},
		ConflictColumns: []string{"id"},
		UpdateColumns:   []string{"name"},
		Values:          orm.NamedArgs{"id": int64(1), "name": "Alice"},
	})
	if err != nil {
		t.Fatalf("build upsert source failed: %v", err)
	}
	if upsert.SQL == "" || len(upsert.Args) == 0 {
		t.Fatalf("unexpected upsert source %#v", upsert)
	}
	lockClause, err := orm.RowLockClause(orm.NewPostgresDialect(), orm.RowLockOptions{SkipLocked: true})
	if err != nil || lockClause != "FOR UPDATE SKIP LOCKED" {
		t.Fatalf("unexpected row lock clause %q err=%v", lockClause, err)
	}
	keyPlan, err := orm.NewGeneratedKeyPlan(orm.NewPostgresDialect(), "id")
	if err != nil || keyPlan.Style != orm.DialectGeneratedKeyReturning {
		t.Fatalf("unexpected generated key plan %#v err=%v", keyPlan, err)
	}
	capabilities, err := orm.NewDialectCapabilities(orm.DbTypePostgres)
	if err != nil {
		t.Fatalf("new dialect capabilities failed: %v", err)
	}
	if capabilities.Placeholder != orm.DialectPlaceholderDollarNumber || !capabilities.SupportsUpsert() || !capabilities.SupportsJSON() {
		t.Fatalf("unexpected postgres capabilities %#v", capabilities)
	}
	if actual := orm.DialectCapabilitiesOf(orm.NewSQLServerDialect()); !actual.LimitOffsetRequiresOrderBy || actual.GeneratedKey != orm.DialectGeneratedKeyOutput {
		t.Fatalf("unexpected sqlserver capabilities %#v", actual)
	}

	page := orm.NewPageRequest(1, 20)
	_ = orm.WithPageRequest(ctx, page)
	if _, ok := orm.PageRequestFromContext(orm.WithPageRequest(ctx, page)); !ok {
		t.Fatalf("expected page request in context")
	}

	var _ orm.TypeHandler = handler
	var _ orm.RowScanner = orm.RowScannerFunc(func(context.Context, []string, orm.RowScannerRow, any) error { return nil })
	var _ orm.IdentifierGenerator = orm.NewDefaultIdentifierGenerator()
	var _ orm.MetaObjectHandler = orm.MetaObjectHandlerFuncs{}
	var _ = orm.MyBatisGlobalConfigFile{DbConfig: orm.MyBatisDbConfigFile{IDType: "assign_id"}}
	var _ orm.StatementInterceptor = orm.StatementInterceptorFunc(func(ctx context.Context, invocation *orm.StatementInvocation) error {
		return invocation.Proceed(ctx)
	})
	var _ orm.SQLGuardRule = orm.SQLGuardRuleFunc(func(context.Context, orm.StatementMeta, string) error { return nil })
	var _ orm.StatementExecutorMiddleware = orm.StatementExecutorMiddlewareFunc(func(next orm.StatementExecutor) orm.StatementExecutor { return next })
	var _ orm.StatementHandlerMiddleware = orm.StatementHandlerMiddlewareFunc(func(next orm.StatementHandler) orm.StatementHandler { return next })
	var _ orm.ParameterHandlerMiddleware = orm.ParameterHandlerMiddlewareFunc(func(next orm.ParameterHandler) orm.ParameterHandler { return next })
	var _ orm.ResultSetHandlerMiddleware = orm.ResultSetHandlerMiddlewareFunc(func(next orm.ResultSetHandler) orm.ResultSetHandler { return next })
	var _ orm.DynamicSQLRenderOptions = orm.DynamicSQLRenderOptions{NullableOnForEach: true, ShrinkWhitespacesInSQL: true}
	var _ orm.AutoMappingBehavior = orm.AutoMappingBehaviorFull
	var _ orm.AutoMappingUnknownColumnBehavior = orm.AutoMappingUnknownColumnBehaviorFailing

	var _ func(*orm.Registry, orm.SQLExecutor, orm.Dialect, ...orm.SQLSessionOption) (*orm.SQLSession, error) = orm.NewSQLSession
	var _ func(*orm.Registry, *sql.DB, orm.Dialect, ...orm.SQLSessionOption) (*orm.SQLSessionFactory, error) = orm.NewSQLSessionFactory
	var _ func(orm.Session) (*orm.BatchSession, error) = orm.NewBatchSession
	var _ func(io.Reader) (orm.MyBatisConfig, error) = orm.DecodeMyBatisConfig
	var _ func(string) (orm.MyBatisConfig, error) = orm.LoadMyBatisConfig
	var _ func(string, orm.MyBatisAssembly) (orm.MyBatisAssemblyResult, error) = orm.LoadAndAssembleMyBatisConfig
	var _ func(orm.StatementSession, orm.EntityMeta, ...orm.BaseMapperOption) (*orm.BaseMapper[contractUser, int64], error) = orm.NewBaseMapper[contractUser, int64]
	var _ func(*orm.BaseMapper[contractUser, int64]) (*orm.Service[contractUser, int64], error) = orm.NewService[contractUser, int64]
	var _ func(context.Context, *orm.BaseMapper[contractUser, int64], orm.TypedField[contractUser, string], *orm.QueryWrapper[contractUser]) ([]string, error) = orm.SelectFieldValues[contractUser, int64, string]
	var _ func(context.Context, *orm.BaseMapper[contractUser, int64], orm.TypedField[contractUser, string], *orm.QueryWrapper[contractUser]) (string, error) = orm.SelectFieldValue[contractUser, int64, string]
	var _ func(context.Context, *orm.BaseMapper[contractUser, int64], orm.TypedField[contractUser, string], *orm.QueryWrapper[contractUser]) (string, error) = orm.SelectFirstFieldValue[contractUser, int64, string]
	var _ func(context.Context, *orm.Service[contractUser, int64], orm.TypedField[contractUser, string], *orm.QueryWrapper[contractUser]) ([]string, error) = orm.ListFieldValues[contractUser, int64, string]
	var _ func(context.Context, *orm.Service[contractUser, int64], orm.TypedField[contractUser, string], *orm.QueryWrapper[contractUser]) (string, error) = orm.GetFieldValue[contractUser, int64, string]
	var _ func(context.Context, *orm.Service[contractUser, int64], orm.TypedField[contractUser, string], *orm.QueryWrapper[contractUser]) (string, error) = orm.GetFirstFieldValue[contractUser, int64, string]
	var _ func(*orm.BaseMapper[contractUser, int64], context.Context, *orm.QueryWrapper[contractUser]) ([]int64, error) = (*orm.BaseMapper[contractUser, int64]).SelectIDs
	var _ func(*orm.Service[contractUser, int64], context.Context, *orm.QueryWrapper[contractUser]) ([]int64, error) = (*orm.Service[contractUser, int64]).ListIDs
	var _ func(*orm.QueryChain[contractUser, int64], context.Context) ([]int64, error) = (*orm.QueryChain[contractUser, int64]).IDs
	var _ func(*orm.Registry) error = orm.ValidateRegistry
	var _ orm.SQLInjector = orm.DefaultSQLInjector{}
	var _ orm.SQLInjector = orm.SQLInjectorFunc(func(orm.EntityMeta, orm.Dialect, orm.GlobalConfig) ([]orm.StatementMeta, error) { return nil, nil })
	var _ orm.InjectNamespaceResolver = func(orm.EntityMeta) string { return "" }
	var _ orm.InjectOption = orm.WithInjectDialect(orm.NewPostgresDialect())
	var _ orm.InjectOption = orm.WithInjectGlobalConfig(orm.DefaultGlobalConfig())
	var _ func(*orm.Registry, string, orm.EntityMeta, orm.SQLInjector, ...orm.InjectOption) error = orm.RegisterInjectedStatements
	var _ func(*orm.Registry, orm.InjectNamespaceResolver, ...orm.InjectOption) error = orm.RegisterDefaultInjectedStatementsForRegistry
	var _ func(int) orm.SQLSessionOption = orm.WithPreparedStatementCacheSize
	var _ func([]orm.DynamicSQLNode, orm.NamedArgs, orm.DynamicSQLRenderOptions) (orm.RenderedSQL, error) = orm.RenderDynamicSQLWithOptions
	var _ func(string) (orm.AutoMappingBehavior, error) = orm.ParseAutoMappingBehavior
	var _ func(string) (orm.AutoMappingUnknownColumnBehavior, error) = orm.ParseAutoMappingUnknownColumnBehavior
	var _ int = orm.DefaultPreparedStatementCacheSize

	if _, err := orm.ParseDbType("postgres"); err != nil {
		t.Fatalf("parse db type failed: %v", err)
	}
	if _, err := orm.ParseIDType("assign_id"); err != nil {
		t.Fatalf("parse id type failed: %v", err)
	}
	if _, err := orm.ParseFieldStrategy("not_empty"); err != nil {
		t.Fatalf("parse field strategy failed: %v", err)
	}
	if _, err := orm.ParseFieldFill("insert_update"); err != nil {
		t.Fatalf("parse field fill failed: %v", err)
	}
	if _, err := orm.ParseResultSetType("FORWARD_ONLY"); err != nil {
		t.Fatalf("parse result set type failed: %v", err)
	}
	if _, err := orm.ParseAutoMappingBehavior("FULL"); err != nil {
		t.Fatalf("parse auto mapping behavior failed: %v", err)
	}
	if _, err := orm.ParseAutoMappingUnknownColumnBehavior("FAILING"); err != nil {
		t.Fatalf("parse unknown column behavior failed: %v", err)
	}

	_, err = orm.AssembleMyBatisConfig(orm.MyBatisAssembly{
		Config: orm.MyBatisConfig{
			Settings:     orm.MyBatisSettings{DatabaseID: "postgres"},
			Environment:  orm.MyBatisEnvironment{DbType: orm.DbTypePostgres},
			TypeAliases:  []orm.TypeAlias{{Alias: "User", TypeName: "contract.User"}},
			TypeHandlers: []orm.TypeHandlerRef{{Name: "json"}},
			Mappers:      []orm.MapperRef{{Namespace: "contract.UserMapper"}},
		},
		Registry: registry,
	})
	if err == nil {
		t.Fatalf("expected missing mapper validation error")
	}
}
