package runtime

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
)

func TestSQLSessionFactory_InTx_whenCallbackSucceeds_shouldCommitAndUseTxSession(t *testing.T) {
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

	err = factory.InTx(context.Background(), &sql.TxOptions{ReadOnly: false}, func(ctx context.Context, session Session) error {
		result, execErr := session.Exec(ctx, "system.user.UserMapper.UpdateName", NamedArgs{
			"id":   int64(7),
			"name": "Alice",
		})
		if execErr != nil {
			return execErr
		}
		if result.RowsAffected != 1 {
			t.Fatalf("unexpected rows affected %d", result.RowsAffected)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	if state.beginCount != 1 || state.commitCount != 1 || state.rollbackCount != 0 {
		t.Fatalf("unexpected transaction lifecycle begin=%d commit=%d rollback=%d", state.beginCount, state.commitCount, state.rollbackCount)
	}
	if state.exec != "update sys_user set name = $1 where id = $2" {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "Alice"}, {Ordinal: 2, Value: int64(7)}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
	if len(state.txOptions) != 1 || state.txOptions[0].ReadOnly {
		t.Fatalf("unexpected tx options %#v", state.txOptions)
	}
}

func TestSQLSessionFactory_InTx_whenCallbackFails_shouldRollback(t *testing.T) {
	state := openTestSQLState(t)
	registry := NewRegistry()
	factory, err := NewSQLSessionFactory(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session factory failed: %v", err)
	}
	expected := errors.New("business failed")

	err = factory.InTx(context.Background(), nil, func(ctx context.Context, session Session) error {
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if state.beginCount != 1 || state.commitCount != 0 || state.rollbackCount != 1 {
		t.Fatalf("unexpected transaction lifecycle begin=%d commit=%d rollback=%d", state.beginCount, state.commitCount, state.rollbackCount)
	}
}

func TestTxSession_Close_whenNotCompleted_shouldRollback(t *testing.T) {
	state := openTestSQLState(t)
	factory, err := NewSQLSessionFactory(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session factory failed: %v", err)
	}
	session, err := factory.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("close tx session failed: %v", err)
	}
	if state.beginCount != 1 || state.commitCount != 0 || state.rollbackCount != 1 {
		t.Fatalf("unexpected transaction lifecycle begin=%d commit=%d rollback=%d", state.beginCount, state.commitCount, state.rollbackCount)
	}

	_, err = session.Exec(context.Background(), "missing.Statement", nil)
	if err == nil || !errors.Is(err, ErrTransactionCompleted) {
		t.Fatalf("expected completed transaction error, got %v", err)
	}
}

func TestSQLSessionFactory_OpenConfiguredSession_whenExecutorTypeBatch_shouldReturnBatchSession(t *testing.T) {
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
	config := DefaultConfiguration()
	config.DefaultExecutorType = ExecutorTypeBatch
	factory, err := NewSQLSessionFactory(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session factory failed: %v", err)
	}

	session, err := factory.OpenConfiguredSession()
	if err != nil {
		t.Fatalf("open configured session failed: %v", err)
	}
	batch, ok := session.(*BatchSession)
	if !ok {
		t.Fatalf("expected BatchSession, got %T", session)
	}
	if _, err := batch.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"}); err != nil {
		t.Fatalf("queue batch exec failed: %v", err)
	}
	if state.exec != "" {
		t.Fatalf("expected batch exec to queue without immediate SQL, got %q", state.exec)
	}
	results, err := batch.Flush(context.Background())
	if err != nil {
		t.Fatalf("flush batch failed: %v", err)
	}
	if len(results) != 1 || results[0].Result.RowsAffected != 1 {
		t.Fatalf("unexpected batch results %#v", results)
	}
	if state.exec != "update sys_user set name = $1 where id = $2" {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
}

func TestTxSession_Commit_whenWriteOccurs_shouldClearSecondLevelCacheAfterCommit(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	state.execResult = testResult{rowsAffected: 1}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16},
		StatementMeta{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where id = #{id}",
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
	factory, err := NewSQLSessionFactory(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session factory failed: %v", err)
	}
	session, err := factory.OpenSession()
	if err != nil {
		t.Fatalf("open session failed: %v", err)
	}
	var before sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &before); err != nil {
		t.Fatalf("warm cache query failed: %v", err)
	}

	tx, err := factory.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}
	if _, err := tx.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Bob"}); err != nil {
		t.Fatalf("tx exec failed: %v", err)
	}
	readBeforeCommit, err := factory.OpenSession()
	if err != nil {
		t.Fatalf("open pre-commit session failed: %v", err)
	}
	var cachedBeforeCommit sqlSessionUser
	if err := readBeforeCommit.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &cachedBeforeCommit); err != nil {
		t.Fatalf("pre-commit query failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	readAfterCommit, err := factory.OpenSession()
	if err != nil {
		t.Fatalf("open post-commit session failed: %v", err)
	}
	var after sqlSessionUser
	if err := readAfterCommit.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &after); err != nil {
		t.Fatalf("post-commit query failed: %v", err)
	}

	if cachedBeforeCommit.Name != "Alice" {
		t.Fatalf("expected cache to remain visible before commit, got %#v", cachedBeforeCommit)
	}
	if after.Name != "Bob" {
		t.Fatalf("expected namespace cache cleared after commit, got %#v", after)
	}
	if len(state.queries) != 2 {
		t.Fatalf("expected one query before commit and one after commit, got %#v", state.queries)
	}
	if state.commitCount != 1 || state.rollbackCount != 0 {
		t.Fatalf("unexpected transaction lifecycle commit=%d rollback=%d", state.commitCount, state.rollbackCount)
	}
}

func TestTxSession_Rollback_whenWriteOccurs_shouldKeepSecondLevelCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	state.execResult = testResult{rowsAffected: 1}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16},
		StatementMeta{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where id = #{id}",
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
	factory, err := NewSQLSessionFactory(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session factory failed: %v", err)
	}
	session, err := factory.OpenSession()
	if err != nil {
		t.Fatalf("open session failed: %v", err)
	}
	var before sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &before); err != nil {
		t.Fatalf("warm cache query failed: %v", err)
	}

	tx, err := factory.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}
	if _, err := tx.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Bob"}); err != nil {
		t.Fatalf("tx exec failed: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	afterRollback, err := factory.OpenSession()
	if err != nil {
		t.Fatalf("open after rollback session failed: %v", err)
	}
	var after sqlSessionUser
	if err := afterRollback.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &after); err != nil {
		t.Fatalf("after rollback query failed: %v", err)
	}

	if before.Name != "Alice" || after.Name != "Alice" {
		t.Fatalf("expected rollback to keep cached value, before=%#v after=%#v", before, after)
	}
	if len(state.queries) != 1 {
		t.Fatalf("expected rollback not to clear namespace cache, got queries %#v", state.queries)
	}
	if state.commitCount != 0 || state.rollbackCount != 1 {
		t.Fatalf("unexpected transaction lifecycle commit=%d rollback=%d", state.commitCount, state.rollbackCount)
	}
}

func TestTxSession_Commit_whenQueryOccurs_shouldPublishSecondLevelCacheAfterCommit(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16}, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	factory, err := NewSQLSessionFactory(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session factory failed: %v", err)
	}
	tx, err := factory.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}
	var inTx sqlSessionUser
	if err := tx.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &inTx); err != nil {
		t.Fatalf("tx query failed: %v", err)
	}
	inTx.Name = "Mutated"
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	afterCommit, err := factory.OpenSession()
	if err != nil {
		t.Fatalf("open session after commit failed: %v", err)
	}
	var cached sqlSessionUser
	if err := afterCommit.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &cached); err != nil {
		t.Fatalf("after commit query failed: %v", err)
	}

	if len(state.queries) != 1 {
		t.Fatalf("expected committed transaction query to populate namespace cache, got queries %#v", state.queries)
	}
	if cached.Name != "Alice" {
		t.Fatalf("expected committed cache to use detached value, got %#v", cached)
	}
}
