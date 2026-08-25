package orm

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
)

func TestBatchSession_Flush_whenStatementsQueued_shouldExecuteInOrder(t *testing.T) {
	state := openTestSQLState(t)
	state.execResults = []driver.Result{
		testResult{rowsAffected: 1, lastInsertID: 11},
		testResult{rowsAffected: 2},
	}
	registry := newSQLSessionRegistry(t,
		StatementMeta{
			ID:               "Insert",
			Namespace:        "system.user.UserMapper",
			FullName:         "system.user.UserMapper.Insert",
			Command:          StatementCommandInsert,
			Source:           StatementSourceAnnotation,
			SQL:              "insert into sys_user(name) values(#{name})",
			UseGeneratedKeys: true,
		},
		StatementMeta{
			ID:        "UpdateName",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateName",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = #{id}",
		},
	)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	batch, err := NewBatchSession(session)
	if err != nil {
		t.Fatalf("new batch session failed: %v", err)
	}

	first, err := batch.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{"name": "Alice"})
	if err != nil {
		t.Fatalf("queue insert failed: %v", err)
	}
	second, err := batch.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Bob"})
	if err != nil {
		t.Fatalf("queue update failed: %v", err)
	}
	if first != (Result{}) || second != (Result{}) {
		t.Fatalf("queued batch exec should return zero result, got %#v %#v", first, second)
	}
	if len(state.execs) != 0 {
		t.Fatalf("batch should not execute before flush, got %#v", state.execs)
	}

	results, err := batch.Flush(context.Background())
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	if len(results) != 2 || results[0].Result.LastInsertID != 11 || results[1].Result.RowsAffected != 2 {
		t.Fatalf("unexpected batch results %#v", results)
	}
	expectedExecs := []string{
		"insert into sys_user(name) values($1)",
		"update sys_user set name = $1 where id = $2",
	}
	if !reflect.DeepEqual(state.execs, expectedExecs) {
		t.Fatalf("unexpected exec order %#v", state.execs)
	}
	expectedArgs := [][]driver.NamedValue{
		{{Ordinal: 1, Value: "Alice"}},
		{{Ordinal: 1, Value: "Bob"}, {Ordinal: 2, Value: int64(7)}},
	}
	if !reflect.DeepEqual(state.execArgsList, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgsList)
	}
}

func TestBatchSession_Clear_whenStatementsQueued_shouldDropPendingStatements(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UpdateName",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UpdateName",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       "update sys_user set name = #{name} where id = #{id}",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	batch, err := NewBatchSession(session)
	if err != nil {
		t.Fatalf("new batch session failed: %v", err)
	}

	_, err = batch.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"})
	if err != nil {
		t.Fatalf("queue update failed: %v", err)
	}
	batch.Clear()
	results, err := batch.Flush(context.Background())
	if err != nil {
		t.Fatalf("flush after clear failed: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected no batch results after clear, got %#v", results)
	}
	if len(state.execs) != 0 {
		t.Fatalf("clear should prevent execution, got %#v", state.execs)
	}
}

func TestBatchSession_Flush_whenStatementFails_shouldReturnPartialResultsAndBatchError(t *testing.T) {
	state := openTestSQLState(t)
	expected := errors.New("driver failed")
	state.execResults = []driver.Result{testResult{rowsAffected: 1}}
	state.execErrors = []error{nil, expected}
	registry := newSQLSessionRegistry(t,
		StatementMeta{
			ID:        "UpdateName",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateName",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = #{id}",
		},
		StatementMeta{
			ID:        "Delete",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.Delete",
			Command:   StatementCommandDelete,
			Source:    StatementSourceAnnotation,
			SQL:       "delete from sys_user where id = #{id}",
		},
	)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	batch, err := NewBatchSession(session)
	if err != nil {
		t.Fatalf("new batch session failed: %v", err)
	}
	_, _ = batch.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"})
	_, _ = batch.Exec(context.Background(), "system.user.UserMapper.Delete", NamedArgs{"id": int64(8)})

	results, err := batch.Flush(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected driver error, got %v", err)
	}
	var batchErr *BatchError
	if !errors.As(err, &batchErr) {
		t.Fatalf("expected BatchError, got %T", err)
	}
	if batchErr.Index != 1 || batchErr.Statement != "system.user.UserMapper.Delete" {
		t.Fatalf("unexpected batch error %#v", batchErr)
	}
	if len(results) != 1 || results[0].Result.RowsAffected != 1 {
		t.Fatalf("unexpected partial results %#v", results)
	}
}

func TestBatchSession_Query_whenPendingStatementsExist_shouldFlushBeforeQuery(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t,
		StatementMeta{
			ID:        "UpdateName",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateName",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = #{id}",
		},
		StatementMeta{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where id = #{id}",
		},
	)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	batch, err := NewBatchSession(session)
	if err != nil {
		t.Fatalf("new batch session failed: %v", err)
	}
	_, _ = batch.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"})

	var user sqlSessionUser
	err = batch.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &user)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if len(state.execs) != 1 || state.query == "" {
		t.Fatalf("expected query to flush pending exec first, execs=%#v query=%q", state.execs, state.query)
	}
	if user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
}

func TestSQLSessionFactory_BeginBatchTx_whenCommitted_shouldFlushAndCommit(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UpdateName",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UpdateName",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       "update sys_user set name = #{name} where id = #{id}",
	})
	factory, err := NewSQLSessionFactory(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session factory failed: %v", err)
	}
	batch, err := factory.BeginBatchTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin batch tx failed: %v", err)
	}
	_, _ = batch.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"})

	if err := batch.CommitContext(context.Background()); err != nil {
		t.Fatalf("commit batch tx failed: %v", err)
	}

	if state.beginCount != 1 || state.commitCount != 1 || state.rollbackCount != 0 {
		t.Fatalf("unexpected transaction lifecycle begin=%d commit=%d rollback=%d", state.beginCount, state.commitCount, state.rollbackCount)
	}
	if len(state.execs) != 1 {
		t.Fatalf("expected one flushed exec, got %#v", state.execs)
	}
}
