package orm

import (
	"context"
	"testing"
)

func TestDb_whenSessionProvided_shouldExecuteStatements(t *testing.T) {
	t.Parallel()

	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UpdateName",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UpdateName",
		Command:   StatementCommandUpdate,
		SQL:       "update sys_user set name = #{name} where id = #{id}",
	})
	state := openTestSQLState(t)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	db, err := NewDb(session)
	if err != nil {
		t.Fatalf("new db failed: %v", err)
	}
	result, err := db.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if result.RowsAffected != 0 {
		t.Fatalf("unexpected result %#v", result)
	}
	if state.exec != "update sys_user set name = $1 where id = $2" {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
}
