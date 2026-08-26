package orm_test

import (
	"context"
	"database/sql"
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
	var _ orm.StatementInterceptor = orm.StatementInterceptorFunc(func(ctx context.Context, invocation *orm.StatementInvocation) error {
		return invocation.Proceed(ctx)
	})
	var _ orm.SQLGuardRule = orm.SQLGuardRuleFunc(func(context.Context, orm.StatementMeta, string) error { return nil })
	var _ orm.StatementExecutorMiddleware = orm.StatementExecutorMiddlewareFunc(func(next orm.StatementExecutor) orm.StatementExecutor { return next })
	var _ orm.StatementHandlerMiddleware = orm.StatementHandlerMiddlewareFunc(func(next orm.StatementHandler) orm.StatementHandler { return next })
	var _ orm.ParameterHandlerMiddleware = orm.ParameterHandlerMiddlewareFunc(func(next orm.ParameterHandler) orm.ParameterHandler { return next })
	var _ orm.ResultSetHandlerMiddleware = orm.ResultSetHandlerMiddlewareFunc(func(next orm.ResultSetHandler) orm.ResultSetHandler { return next })

	var _ func(*orm.Registry, orm.SQLExecutor, orm.Dialect, ...orm.SQLSessionOption) (*orm.SQLSession, error) = orm.NewSQLSession
	var _ func(*orm.Registry, *sql.DB, orm.Dialect, ...orm.SQLSessionOption) (*orm.SQLSessionFactory, error) = orm.NewSQLSessionFactory
	var _ func(orm.Session) (*orm.BatchSession, error) = orm.NewBatchSession
	var _ func(orm.StatementSession, orm.EntityMeta, ...orm.BaseMapperOption) (*orm.BaseMapper[contractUser, int64], error) = orm.NewBaseMapper[contractUser, int64]
	var _ func(*orm.BaseMapper[contractUser, int64]) (*orm.Service[contractUser, int64], error) = orm.NewService[contractUser, int64]

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
}
