package runtime

import (
	"context"
	"reflect"
	"testing"
)

func TestSQLSession_whenStatementExecutorProvided_shouldDelegateStatementCalls(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t,
		StatementMeta{
			ID:        "List",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.List",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where name = #{name}",
		},
		StatementMeta{
			ID:        "Insert",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.Insert",
			Command:   StatementCommandInsert,
			Source:    StatementSourceAnnotation,
			SQL:       "insert into sys_user(name) values(#{name})",
		},
	)
	executor := &recordingStatementExecutor{}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithStatementExecutor(executor))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", NamedArgs{"name": "Alice"}, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	var one sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.List", NamedArgs{"name": "Bob"}, &one); err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	result, err := session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{"name": "Carol"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if executor.queryCalls != 1 || executor.queryOneCalls != 1 || executor.execCalls != 1 {
		t.Fatalf("unexpected executor calls: query=%d queryOne=%d exec=%d", executor.queryCalls, executor.queryOneCalls, executor.execCalls)
	}
	if len(users) != 1 || users[0].ID != 11 || one.ID != 12 {
		t.Fatalf("custom executor did not populate destinations, users=%#v one=%#v", users, one)
	}
	if result.RowsAffected != 3 || result.LastInsertID != 44 {
		t.Fatalf("unexpected custom exec result %#v", result)
	}
	if !reflect.DeepEqual(executor.execArgs, NamedArgs{"name": "Carol"}) {
		t.Fatalf("unexpected observed exec args %#v", executor.execArgs)
	}
	if state.query != "" || state.exec != "" {
		t.Fatalf("custom executor should short-circuit SQL executor, query=%q exec=%q", state.query, state.exec)
	}
}

type recordingStatementExecutor struct {
	queryCalls    int
	queryOneCalls int
	execCalls     int
	execArgs      NamedArgs
}

func (e *recordingStatementExecutor) Query(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error {
	e.queryCalls++
	users, ok := dest.(*[]sqlSessionUser)
	if ok {
		*users = []sqlSessionUser{{ID: 11, Name: meta.ID}}
	}
	return nil
}

func (e *recordingStatementExecutor) QueryOne(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error {
	e.queryOneCalls++
	user, ok := dest.(*sqlSessionUser)
	if ok {
		user.ID = 12
		user.Name = meta.FullName
	}
	return nil
}

func (e *recordingStatementExecutor) Exec(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs) (Result, error) {
	e.execCalls++
	e.execArgs = copyNamedArgs(args)
	return Result{RowsAffected: 3, LastInsertID: 44}, nil
}
