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

func TestDefaultSQLInjector_whenSoftDeleteEntityProvided_shouldRegisterDefaultMethodStatements(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	entity := baseMapperAuditUserEntity()
	if err := registry.RegisterEntity(entity); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	err := RegisterInjectedStatements(
		registry,
		"system.user.UserDefaultMapper",
		entity,
		DefaultSQLInjector{},
		WithInjectDialect(NewPostgresDialect()),
	)
	if err != nil {
		t.Fatalf("register default injected statements failed: %v", err)
	}

	cases := map[string]string{
		"SelectByID":         `SELECT "id", "name", "version", "deleted", "created_at", "updated_at" FROM "sys_user" WHERE "id" = #{id} AND "deleted" = #{live}`,
		"SelectCount":        `SELECT COUNT(1) FROM "sys_user" WHERE "deleted" = #{live}`,
		"PhysicalDeleteByID": `DELETE FROM "sys_user" WHERE "id" = #{id}`,
		"LogicDeleteByID":    `UPDATE "sys_user" SET "deleted" = #{deleted} WHERE "id" = #{id} AND "deleted" = #{live}`,
	}
	for id, expectedSQL := range cases {
		statement, ok := registry.Statement("system.user.UserDefaultMapper." + id)
		if !ok {
			t.Fatalf("expected statement %s", id)
		}
		if statement.SQL != expectedSQL {
			t.Fatalf("unexpected %s SQL %q", id, statement.SQL)
		}
	}
}

func TestRegisterDefaultInjectedStatementsForRegistry_whenEntitiesRegistered_shouldUseNamespaceResolver(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	entity := baseMapperUserEntity()
	if err := registry.RegisterEntity(entity); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}

	err := RegisterDefaultInjectedStatementsForRegistry(
		registry,
		func(entity EntityMeta) string { return "system.injected." + entity.TypeName + "Mapper" },
		WithInjectDialect(NewMySQLDialect()),
	)
	if err != nil {
		t.Fatalf("register default statements for registry failed: %v", err)
	}

	statement, ok := registry.Statement("system.injected.baseMapperUserMapper.SelectByID")
	if !ok {
		t.Fatalf("expected registry injected statement")
	}
	if statement.SQL != "SELECT `id`, `name`, `status` FROM `sys_user` WHERE `id` = #{id}" {
		t.Fatalf("unexpected SQL %q", statement.SQL)
	}
}
