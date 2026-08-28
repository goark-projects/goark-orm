package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestStatementInterceptorIgnored_whenAliasesProvided_shouldMatchCanonicalName(t *testing.T) {
	statement := StatementMeta{InterceptorIgnores: []string{"blockAttack", "data_permission"}}

	if !StatementInterceptorIgnored(statement, InterceptorNameBlockAttack) {
		t.Fatalf("expected block attack to be ignored")
	}
	if !StatementInterceptorIgnored(statement, InterceptorNameDataPermission) {
		t.Fatalf("expected data permission to be ignored")
	}
	if StatementInterceptorIgnored(statement, InterceptorNameTenant) {
		t.Fatalf("tenant should not be ignored")
	}
}

func TestStatementInterceptorIgnored_whenAllProvided_shouldMatchEveryNamedInterceptor(t *testing.T) {
	statement := StatementMeta{InterceptorIgnores: []string{InterceptorNameAll}}

	if !StatementInterceptorIgnored(statement, InterceptorNameTenant) {
		t.Fatalf("expected tenant to be ignored")
	}
	if !StatementInterceptorIgnored(statement, InterceptorNamePagination) {
		t.Fatalf("expected pagination to be ignored")
	}
}

func TestBlockAttackInterceptor_whenStatementIgnored_shouldAllowUnsafeUpdate(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:                 "UnsafeUpdate",
		Namespace:          "system.user.UserMapper",
		FullName:           "system.user.UserMapper.UnsafeUpdate",
		Command:            StatementCommandUpdate,
		Source:             StatementSourceAnnotation,
		SQL:                `update sys_user set name = #{name}`,
		InterceptorIgnores: []string{InterceptorNameBlockAttack},
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewBlockAttackInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	result, err := session.Exec(context.Background(), "system.user.UserMapper.UnsafeUpdate", NamedArgs{"name": "Alice"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	if state.exec != `update sys_user set name = $1` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
}

func TestTenantAndPaginationInterceptors_whenStatementIgnored_shouldNotRewriteSQL(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from sys_user where status = #{status}`,
		InterceptorIgnores: []string{
			InterceptorNameTenant,
			InterceptorNamePagination,
		},
	})
	session, err := NewSQLSession(
		registry,
		state.db,
		NewPostgresDialect(),
		WithInterceptors(
			NewTenantInterceptor("tenant_id", int64(1001)),
			NewPaginationInterceptor(),
		),
	)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	ctx := WithPageRequest(context.Background(), NewPageRequest(2, 10))
	if err := session.Query(ctx, "system.user.UserMapper.List", NamedArgs{"status": "ACTIVE"}, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != `select id from sys_user where status = $1` {
		t.Fatalf("unexpected SQL %q", state.query)
	}
	if !reflect.DeepEqual(state.queryArgs, []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}}) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
}
