package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"
)

func TestSQLSession_whenInterceptorsRegistered_shouldRunInOrderAndRewriteStatement(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id", "name"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id, name from sys_user where status = #{status}`,
	})
	order := make([]string, 0, 4)
	first := StatementInterceptorFunc(func(ctx context.Context, invocation *StatementInvocation) error {
		order = append(order, "first-before")
		invocation.Statement().SQL += " and owner_id = #{owner_id}"
		invocation.Statement().Args["owner_id"] = int64(9)
		err := invocation.Proceed(ctx)
		order = append(order, "first-after")
		return err
	})
	second := StatementInterceptorFunc(func(ctx context.Context, invocation *StatementInvocation) error {
		order = append(order, "second-before")
		err := invocation.Proceed(ctx)
		order = append(order, "second-after")
		return err
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(first, second))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"status": "ACTIVE"}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if !reflect.DeepEqual(order, []string{"first-before", "second-before", "second-after", "first-after"}) {
		t.Fatalf("unexpected interceptor order %#v", order)
	}
	if state.query != `select id, name from sys_user where status = $1 and owner_id = $2` {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}, {Ordinal: 2, Value: int64(9)}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
}

func TestBlockAttackInterceptor_whenUpdateWithoutWhere_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UnsafeUpdate",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UnsafeUpdate",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       `update sys_user set name = #{name}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewBlockAttackInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.UnsafeUpdate", NamedArgs{"name": "Alice"})
	if err == nil || !strings.Contains(err.Error(), "blocked full-table update") {
		t.Fatalf("expected block attack error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("unsafe SQL should not execute, got %q", state.exec)
	}
}

func TestBlockAttackInterceptor_whenWhereKeywordIsQuotedIdentifier_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UnsafeUpdate",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UnsafeUpdate",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       `update sys_user set [where] = #{name}`,
	})
	session, err := NewSQLSession(registry, state.db, NewSQLServerDialect(), WithInterceptors(NewBlockAttackInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.UnsafeUpdate", NamedArgs{"name": "Alice"})
	if err == nil || !strings.Contains(err.Error(), "blocked full-table update") {
		t.Fatalf("expected block attack error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("unsafe SQL should not execute, got %q", state.exec)
	}
}

func TestSQLObserverInterceptor_whenStatementRewritten_shouldObserveFinalTemplate(t *testing.T) {
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
	var observed SQLObservation
	observer := NewSQLObserverInterceptor(func(ctx context.Context, item SQLObservation) error {
		observed = item
		return nil
	})
	rewrite := StatementInterceptorFunc(func(ctx context.Context, invocation *StatementInvocation) error {
		invocation.Statement().SQL += " where id = #{id}"
		invocation.Statement().Args["id"] = int64(7)
		return invocation.Proceed(ctx)
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(observer, rewrite))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", nil, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if observed.SQL != `select id from sys_user where id = #{id}` {
		t.Fatalf("unexpected observed SQL %q", observed.SQL)
	}
	if observed.Args["id"] != int64(7) {
		t.Fatalf("unexpected observed args %#v", observed.Args)
	}
}

func TestTenantInterceptor_whenSelectHasWhere_shouldAppendTenantCondition(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from sys_user where status = #{status}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewTenantInterceptor("tenant_id", int64(1001))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"status": "ACTIVE"}, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != `select id from sys_user where status = $1 AND "tenant_id" = $2` {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}, {Ordinal: 2, Value: int64(1001)}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
}

func TestTenantInterceptor_whenInsertHasColumns_shouldAppendTenantColumnAndValue(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "Insert",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.Insert",
		Command:   StatementCommandInsert,
		Source:    StatementSourceAnnotation,
		SQL:       `insert into sys_user(name, status) values(#{name}, #{status})`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewTenantInterceptor("tenant_id", int64(1001))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{"name": "Alice", "status": "ACTIVE"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if state.exec != `insert into sys_user(name, status, "tenant_id") values($1, $2, $3)` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: "ACTIVE"},
		{Ordinal: 3, Value: int64(1001)},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}

func TestTenantInterceptor_whenInsertAlreadyHasTenantColumn_shouldNotDuplicateTenant(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "Insert",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.Insert",
		Command:   StatementCommandInsert,
		Source:    StatementSourceAnnotation,
		SQL:       `insert into sys_user(name, tenant_id) values(#{name}, #{tenantID})`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewTenantInterceptor("tenant_id", int64(1001))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{"name": "Alice", "tenantID": int64(2002)})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if state.exec != `insert into sys_user(name, tenant_id) values($1, $2)` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "Alice"}, {Ordinal: 2, Value: int64(2002)}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}

func TestTenantInterceptor_whenInsertHasMultipleValueRows_shouldAppendTenantValuePerRow(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 2}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "InsertBatch",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.InsertBatch",
		Command:   StatementCommandInsert,
		Source:    StatementSourceAnnotation,
		SQL:       `insert into sys_user(name) values(#{name1}), (#{name2}) returning id`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewTenantInterceptor("tenant_id", int64(1001))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.InsertBatch", NamedArgs{"name1": "Alice", "name2": "Bob"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if state.exec != `insert into sys_user(name, "tenant_id") values($1, $2), ($3, $4) returning id` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: int64(1001)},
		{Ordinal: 3, Value: "Bob"},
		{Ordinal: 4, Value: int64(1001)},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}

func TestTenantInterceptor_whenInsertWithoutColumnList_shouldReturnError(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "Insert",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.Insert",
		Command:   StatementCommandInsert,
		Source:    StatementSourceAnnotation,
		SQL:       `insert into sys_user values(#{id}, #{name})`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewTenantInterceptor("tenant_id", int64(1001))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{"id": int64(7), "name": "Alice"})
	if err == nil || !strings.Contains(err.Error(), "tenant insert requires explicit column list") {
		t.Fatalf("expected tenant insert error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("unsafe insert should not execute, got %q", state.exec)
	}
}

func TestDataPermissionInterceptor_whenProviderReturnsCondition_shouldAppendCondition(t *testing.T) {
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
	interceptor := NewDataPermissionInterceptor(func(ctx context.Context, statement StatementMeta) (SQLCondition, error) {
		return SQLCondition{SQL: `"owner_id" = #{owner_id}`, Args: NamedArgs{"owner_id": int64(7)}}, nil
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(interceptor))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", nil, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != `select id from sys_user WHERE "owner_id" = $1` {
		t.Fatalf("unexpected query %q", state.query)
	}
	if !reflect.DeepEqual(state.queryArgs, []driver.NamedValue{{Ordinal: 1, Value: int64(7)}}) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
}

func TestDynamicTableInterceptor_whenTableMapped_shouldRewriteTableName(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from "sys_user" where id = #{id}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewDynamicTableInterceptor(map[string]string{"sys_user": "sys_user_2026"})))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"id": int64(7)}, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != `select id from "sys_user_2026" where id = $1` {
		t.Fatalf("unexpected query %q", state.query)
	}
}

func TestPaginationInterceptor_whenContextHasPageRequest_shouldAppendLimitOffset(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from sys_user where status = #{status}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewPaginationInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	ctx := WithPageRequest(context.Background(), NewPageRequest(3, 20))
	if err := session.Query(ctx, "system.user.UserMapper.List", NamedArgs{"status": "ACTIVE"}, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != `select id from sys_user where status = $1 LIMIT $2 OFFSET $3` {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "ACTIVE"},
		{Ordinal: 2, Value: int64(20)},
		{Ordinal: 3, Value: int64(40)},
	}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
}
