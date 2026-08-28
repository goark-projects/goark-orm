package runtime

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

func TestBlockAttackInterceptor_whenWhereOnlyExistsInSubquery_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UnsafeUpdate",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UnsafeUpdate",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       `update sys_user set role_id = (select id from sys_role where code = #{code})`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewBlockAttackInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.UnsafeUpdate", NamedArgs{"code": "admin"})
	if err == nil || !strings.Contains(err.Error(), "blocked full-table update") {
		t.Fatalf("expected block attack error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("unsafe SQL should not execute, got %q", state.exec)
	}
}

func TestBlockAttackInterceptor_whenWhereOnlyExistsInPlaceholder_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UnsafeUpdate",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UnsafeUpdate",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       `update sys_user set name = #{where}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewBlockAttackInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.UnsafeUpdate", NamedArgs{"where": "Alice"})
	if err == nil || !strings.Contains(err.Error(), "blocked full-table update") {
		t.Fatalf("expected block attack error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("unsafe SQL should not execute, got %q", state.exec)
	}
}

func TestAppendSQLCondition_whenKeywordOnlyExistsInPlaceholder_shouldAppendWhere(t *testing.T) {
	actual := appendSQLCondition(`select #{where} as marker from sys_user`, `"tenant_id" = #{tenantID}`)
	expected := `select #{where} as marker from sys_user WHERE "tenant_id" = #{tenantID}`
	if actual != expected {
		t.Fatalf("unexpected SQL %q", actual)
	}
}

func TestAppendSQLCondition_whenOrderTokenIsPredicateColumn_shouldAppendAnd(t *testing.T) {
	actual := appendSQLCondition(`select id from sys_user where order = #{order}`, `"tenant_id" = #{tenantID}`)
	expected := `select id from sys_user where order = #{order} AND "tenant_id" = #{tenantID}`
	if actual != expected {
		t.Fatalf("unexpected SQL %q", actual)
	}
}

func TestAppendSQLCondition_whenTailKeywordsArePredicateColumns_shouldAppendAnd(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "limit_column",
			query:    `select id from sys_user where limit = #{limit}`,
			expected: `select id from sys_user where limit = #{limit} AND "tenant_id" = #{tenantID}`,
		},
		{
			name:     "offset_column",
			query:    `select id from sys_user where offset between #{begin} and #{end}`,
			expected: `select id from sys_user where offset between #{begin} and #{end} AND "tenant_id" = #{tenantID}`,
		},
		{
			name:     "having_column",
			query:    `select id from sys_user where having = #{having}`,
			expected: `select id from sys_user where having = #{having} AND "tenant_id" = #{tenantID}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := appendSQLCondition(tt.query, `"tenant_id" = #{tenantID}`)
			if actual != tt.expected {
				t.Fatalf("unexpected SQL %q", actual)
			}
		})
	}
}

func TestAppendSQLCondition_whenTailKeywordsAreProjectionColumns_shouldAppendAnd(t *testing.T) {
	actual := appendSQLCondition(`select limit, offset, having from sys_user where status = #{status}`, `"tenant_id" = #{tenantID}`)
	expected := `select limit, offset, having from sys_user where status = #{status} AND "tenant_id" = #{tenantID}`
	if actual != expected {
		t.Fatalf("unexpected SQL %q", actual)
	}
}

func TestAppendSQLCondition_whenOrderByTailExists_shouldAppendBeforeTail(t *testing.T) {
	actual := appendSQLCondition(`select id from sys_user where status = #{status} order by id`, `"tenant_id" = #{tenantID}`)
	expected := `select id from sys_user where status = #{status} AND "tenant_id" = #{tenantID} order by id`
	if actual != expected {
		t.Fatalf("unexpected SQL %q", actual)
	}
}

func TestAppendSQLCondition_whenLimitOffsetTailExists_shouldAppendBeforeTail(t *testing.T) {
	actual := appendSQLCondition(`select id from sys_user where status = #{status} limit #{limit} offset #{offset}`, `"tenant_id" = #{tenantID}`)
	expected := `select id from sys_user where status = #{status} AND "tenant_id" = #{tenantID} limit #{limit} offset #{offset}`
	if actual != expected {
		t.Fatalf("unexpected SQL %q", actual)
	}
}

func TestAppendSQLCondition_whenGroupingTailExists_shouldAppendBeforeGrouping(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "group_by_without_where",
			query:    `select status, count(*) from sys_user group by status`,
			expected: `select status, count(*) from sys_user WHERE "tenant_id" = #{tenantID} group by status`,
		},
		{
			name:     "group_by_with_where",
			query:    `select status, count(*) from sys_user where active = #{active} group by status having count(*) > 1`,
			expected: `select status, count(*) from sys_user where active = #{active} AND "tenant_id" = #{tenantID} group by status having count(*) > 1`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := appendSQLCondition(tt.query, `"tenant_id" = #{tenantID}`)
			if actual != tt.expected {
				t.Fatalf("unexpected SQL %q", actual)
			}
		})
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

func TestTenantInterceptor_whenOnlySubqueryHasWhere_shouldAppendTopLevelWhere(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"role_id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select (select max(id) from sys_role where code = #{code}) as role_id from sys_user`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewTenantInterceptor("tenant_id", int64(1001))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"code": "admin"}, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	expectedSQL := `select (select max(id) from sys_role where code = $1) as role_id from sys_user WHERE "tenant_id" = $2`
	if state.query != expectedSQL {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "admin"}, {Ordinal: 2, Value: int64(1001)}}
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
