package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestSQLSession_WithLayerMiddlewares_shouldWrapExecutionLayers(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:         "FindByID",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.FindByID",
		Command:    StatementCommandSelect,
		Source:     StatementSourceAnnotation,
		SQL:        "select id, name from sys_user where id = #{id}",
		ResultType: "sqlSessionUser",
	})
	calls := make([]string, 0)
	session, err := NewSQLSession(
		registry,
		state.db,
		NewPostgresDialect(),
		WithStatementExecutorMiddleware(StatementExecutorMiddlewareFunc(func(next StatementExecutor) StatementExecutor {
			return recordingExecutor{next: next, calls: &calls}
		})),
		WithStatementHandlerMiddleware(StatementHandlerMiddlewareFunc(func(next StatementHandler) StatementHandler {
			return recordingStatementHandler{next: next, calls: &calls}
		})),
		WithParameterHandlerMiddleware(ParameterHandlerMiddlewareFunc(func(next ParameterHandler) ParameterHandler {
			return recordingParameterHandler{next: next, calls: &calls}
		})),
		WithResultSetHandlerMiddleware(ResultSetHandlerMiddlewareFunc(func(next ResultSetHandler) ResultSetHandler {
			return recordingResultSetHandler{next: next, calls: &calls}
		})),
	)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &user)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
	expectedCalls := []string{
		"executor:query-one",
		"statement:prepare",
		"statement:compile",
		"parameter:bind",
		"result:scan-one",
	}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("unexpected middleware calls %#v", calls)
	}
}

func TestSQLSession_WithLayerMiddleware_whenWrapperReturnsNil_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	_, err := NewSQLSession(
		NewRegistry(),
		state.db,
		NewPostgresDialect(),
		WithResultSetHandlerMiddleware(ResultSetHandlerMiddlewareFunc(func(next ResultSetHandler) ResultSetHandler {
			return nil
		})),
	)
	if err == nil {
		t.Fatalf("expected nil middleware result to be rejected")
	}
}

type recordingExecutor struct {
	next  StatementExecutor
	calls *[]string
}

func (e recordingExecutor) Query(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error {
	*e.calls = append(*e.calls, "executor:query")
	return e.next.Query(ctx, session, meta, args, dest)
}

func (e recordingExecutor) QueryOne(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error {
	*e.calls = append(*e.calls, "executor:query-one")
	return e.next.QueryOne(ctx, session, meta, args, dest)
}

func (e recordingExecutor) Exec(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs) (Result, error) {
	*e.calls = append(*e.calls, "executor:exec")
	return e.next.Exec(ctx, session, meta, args)
}

type recordingStatementHandler struct {
	next  StatementHandler
	calls *[]string
}

func (h recordingStatementHandler) Prepare(ctx context.Context, meta StatementMeta, args NamedArgs) (*StatementRuntime, error) {
	*h.calls = append(*h.calls, "statement:prepare")
	return h.next.Prepare(ctx, meta, args)
}

func (h recordingStatementHandler) Compile(ctx context.Context, runtime *StatementRuntime) (CompiledSQL, error) {
	*h.calls = append(*h.calls, "statement:compile")
	return h.next.Compile(ctx, runtime)
}

func (h recordingStatementHandler) CompileText(ctx context.Context, meta StatementMeta, dialect Dialect, sqlText string, args NamedArgs) (CompiledSQL, error) {
	*h.calls = append(*h.calls, "statement:compile-text")
	return h.next.CompileText(ctx, meta, dialect, sqlText, args)
}

type recordingParameterHandler struct {
	next  ParameterHandler
	calls *[]string
}

func (h recordingParameterHandler) Bind(ctx context.Context, statement StatementMeta, args NamedArgs) (NamedArgs, error) {
	*h.calls = append(*h.calls, "parameter:bind")
	return h.next.Bind(ctx, statement, args)
}

type recordingResultSetHandler struct {
	next  ResultSetHandler
	calls *[]string
}

func (h recordingResultSetHandler) ScanRows(ctx context.Context, rows Rows, statement StatementMeta, dest any) error {
	*h.calls = append(*h.calls, "result:scan-rows")
	return h.next.ScanRows(ctx, rows, statement, dest)
}

func (h recordingResultSetHandler) ScanOne(ctx context.Context, rows Rows, statement StatementMeta, dest any) error {
	*h.calls = append(*h.calls, "result:scan-one")
	return h.next.ScanOne(ctx, rows, statement, dest)
}
