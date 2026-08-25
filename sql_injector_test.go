package orm

import (
	"context"
	"testing"
)

func TestRegisterInjectedStatements_whenLogicDeleteByIDProvided_shouldRegisterExecutableStatement(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	entity := baseMapperAuditUserEntity()
	if err := registry.RegisterEntity(entity); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	err := RegisterInjectedStatements(
		registry,
		"system.user.UserExtraMapper",
		entity,
		LogicDeleteByIDInjector{},
		WithInjectDialect(NewPostgresDialect()),
	)
	if err != nil {
		t.Fatalf("register injected statements failed: %v", err)
	}

	statement, ok := registry.Statement("system.user.UserExtraMapper.LogicDeleteByID")
	if !ok {
		t.Fatalf("expected injected statement")
	}
	if statement.SQL != `UPDATE "sys_user" SET "deleted" = #{deleted} WHERE "id" = #{id} AND "deleted" = #{live}` {
		t.Fatalf("unexpected SQL %q", statement.SQL)
	}

	state := openTestSQLState(t)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	_, err = session.Exec(context.Background(), "system.user.UserExtraMapper.LogicDeleteByID", NamedArgs{
		"id":      int64(7),
		"deleted": true,
		"live":    false,
	})
	if err != nil {
		t.Fatalf("exec injected statement failed: %v", err)
	}
	if state.exec != `UPDATE "sys_user" SET "deleted" = $1 WHERE "id" = $2 AND "deleted" = $3` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
}
