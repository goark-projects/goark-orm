package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestAssembleMyBatisConfig_whenTenantAndDynamicTablePluginsConfigured_shouldApplyOrderedPlugins(t *testing.T) {
	t.Parallel()

	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from "sys_user" where status = #{status}`,
	})
	if err := registry.RegisterTypeHandler("profile", profileTypeHandler{}); err != nil {
		t.Fatalf("register type handler failed: %v", err)
	}
	assembled, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config: MyBatisConfig{
			Environment: MyBatisEnvironment{DbType: DbTypePostgres},
			Plugins: []PluginRef{
				{Name: "tenant", Order: 20, Options: map[string]string{"column": "tenant_id", "value": "1001"}},
				{Name: "dynamicTable", Order: 10, Options: map[string]string{"sys_user": "sys_user_2026"}},
			},
		},
		Registry: registry,
		DB:       state.db,
	})
	if err != nil {
		t.Fatalf("assemble config failed: %v", err)
	}
	defer assembled.Session.Close()

	var users []sqlSessionUser
	if err := assembled.Session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"status": "ACTIVE"}, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	expectedSQL := `select id from "sys_user_2026" where status = $1 AND "tenant_id" = $2`
	if state.query != expectedSQL {
		t.Fatalf("unexpected SQL %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}, {Ordinal: 2, Value: "1001"}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
}

func TestPluginRegistryBuilder_whenCustomPluginsRegistered_shouldBuildValidatedRegistry(t *testing.T) {
	t.Parallel()

	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from sys_user`,
	})
	if err := registry.RegisterTypeHandler("profile", profileTypeHandler{}); err != nil {
		t.Fatalf("register type handler failed: %v", err)
	}
	var observed SQLObservation
	plugins, err := NewPluginRegistryBuilder().
		WithDataPermission(func(ctx context.Context, statement StatementMeta) (SQLCondition, error) {
			return SQLCondition{SQL: `"owner_id" = #{ownerID}`, Args: NamedArgs{"ownerID": int64(7)}}, nil
		}).
		WithSQLObserver(func(ctx context.Context, item SQLObservation) error {
			observed = item
			return nil
		}).
		Build()
	if err != nil {
		t.Fatalf("build plugin registry failed: %v", err)
	}

	assembled, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config: MyBatisConfig{
			Environment: MyBatisEnvironment{DbType: DbTypePostgres},
			Plugins: []PluginRef{
				{Name: "sqlObserver", Order: 10},
				{Name: "dataPermission", Order: 20},
			},
		},
		Registry: registry,
		DB:       state.db,
		Plugins:  plugins,
	})
	if err != nil {
		t.Fatalf("assemble config failed: %v", err)
	}
	defer assembled.Session.Close()

	var users []sqlSessionUser
	if err := assembled.Session.Query(context.Background(), "system.user.UserMapper.List", nil, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != `select id from sys_user WHERE "owner_id" = $1` {
		t.Fatalf("unexpected SQL %q", state.query)
	}
	if observed.SQL != `select id from sys_user WHERE "owner_id" = #{ownerID}` {
		t.Fatalf("observer should see final template, got %q", observed.SQL)
	}
}

func TestPluginRegistryBuilder_whenDuplicateRegistered_shouldReject(t *testing.T) {
	t.Parallel()

	_, err := NewPluginRegistryBuilder().
		With("custom", StatementInterceptorFunc(func(ctx context.Context, invocation *StatementInvocation) error {
			return invocation.Proceed(ctx)
		})).
		With("custom", StatementInterceptorFunc(func(ctx context.Context, invocation *StatementInvocation) error {
			return invocation.Proceed(ctx)
		})).
		Build()
	if err == nil {
		t.Fatalf("expected duplicate plugin error")
	}
}
