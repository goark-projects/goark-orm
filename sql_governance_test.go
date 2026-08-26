package orm

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestIllegalSQLInterceptor_whenSelectWildcardProvided_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select * from sys_user where id = #{id}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewIllegalSQLInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"id": int64(7)}, &users)
	if err == nil || !strings.Contains(err.Error(), "SELECT *") {
		t.Fatalf("expected select wildcard error, got %v", err)
	}
	if state.query != "" {
		t.Fatalf("query should not execute, got %q", state.query)
	}
}

func TestIllegalSQLInterceptor_whenAggregateWildcardProvided_shouldAllowCountStar(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"total"},
		values:  [][]driver.Value{{int64(3)}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "Count",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.Count",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select count(*) as total from sys_user`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewIllegalSQLInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var total int64
	err = session.QueryOne(context.Background(), "system.user.UserMapper.Count", nil, &total)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("unexpected total %d", total)
	}
}

func TestIllegalSQLInterceptor_whenMultipleStatementsProvided_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from sys_user; delete from sys_user`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewIllegalSQLInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", nil, &users)
	if err == nil || !strings.Contains(err.Error(), "multiple statements") {
		t.Fatalf("expected multiple statement error, got %v", err)
	}
	if state.query != "" {
		t.Fatalf("query should not execute, got %q", state.query)
	}
}

func TestIllegalSQLInterceptor_whenWriteWithoutWhereProvided_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UpdateAll",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UpdateAll",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       `update sys_user set status = #{status}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewIllegalSQLInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.UpdateAll", NamedArgs{"status": "LOCKED"})
	if err == nil || !strings.Contains(err.Error(), "without WHERE") {
		t.Fatalf("expected write without where error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("exec should not run, got %q", state.exec)
	}
}

func TestIllegalSQLInterceptor_whenRuleDisabled_shouldAllowMatchingSQL(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select * from sys_user where id = #{id}`,
	})
	session, err := NewSQLSession(
		registry,
		state.db,
		NewPostgresDialect(),
		WithInterceptors(NewIllegalSQLInterceptor(WithIllegalSQLDenySelectWildcard(false))),
	)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"id": int64(7)}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(users) != 1 || users[0].ID != 7 {
		t.Fatalf("unexpected users %#v", users)
	}
}

func TestIllegalSQLInterceptor_whenStatementIgnored_shouldSkipGovernanceRules(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:                 "List",
		Namespace:          "system.user.UserMapper",
		FullName:           "system.user.UserMapper.List",
		Command:            StatementCommandSelect,
		Source:             StatementSourceAnnotation,
		SQL:                `select * from sys_user where id = #{id}`,
		InterceptorIgnores: []string{InterceptorNameIllegalSQL},
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewIllegalSQLInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"id": int64(7)}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("unexpected users %#v", users)
	}
}

func TestReadOnlyInterceptor_whenWriteProvided_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "Insert",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.Insert",
		Command:   StatementCommandInsert,
		Source:    StatementSourceAnnotation,
		SQL:       `insert into sys_user(name) values(#{name})`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewReadOnlyInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{"name": "Alice"})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("exec should not run, got %q", state.exec)
	}
}

func TestReadOnlyInterceptor_whenSelectProvided_shouldAllowQuery(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id, name from sys_user where id = #{id}`,
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewReadOnlyInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if !reflect.DeepEqual(state.queryArgs, []driver.NamedValue{{Ordinal: 1, Value: int64(7)}}) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
}

func TestSQLGuardInterceptor_whenCustomRuleRejects_shouldStopExecution(t *testing.T) {
	state := openTestSQLState(t)
	expectedErr := errors.New("custom governance rejection")
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       `select id from sys_user`,
	})
	session, err := NewSQLSession(
		registry,
		state.db,
		NewPostgresDialect(),
		WithInterceptors(NewSQLGuardInterceptor(SQLGuardRuleFunc(func(ctx context.Context, statement StatementMeta, sql string) error {
			return expectedErr
		}))),
	)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", nil, &users)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected custom rule error, got %v", err)
	}
	if state.query != "" {
		t.Fatalf("query should not execute, got %q", state.query)
	}
}
