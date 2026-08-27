package orm

import (
	"context"
	"reflect"
	"testing"
)

func TestSelectSQLBuilder_whenConditionsAndPagingProvided_shouldBuildSafeSQLSource(t *testing.T) {
	source, err := NewSelectSQLBuilder().
		Select("id", "name").
		From("sys_user").
		WhereEq("status", "ACTIVE").
		OrderByDesc("id").
		Limit(10).
		Offset(20).
		CacheKey("status-page").
		Build()
	if err != nil {
		t.Fatalf("build select SQL failed: %v", err)
	}

	compiled, err := CompileSQLContext(context.Background(), source.SQL, source.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}

	expectedSQL := `SELECT "id", "name" FROM "sys_user" WHERE "status" = $1 ORDER BY "id" DESC LIMIT $2 OFFSET $3`
	if compiled.SQL != expectedSQL {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{"ACTIVE", 10, 20}) {
		t.Fatalf("unexpected args %#v", compiled.Args)
	}
	if source.CacheKey != "status-page" {
		t.Fatalf("unexpected cache key %q", source.CacheKey)
	}
}

func TestInsertUpdateDeleteSQLBuilders_whenValuesProvided_shouldBuildWriteSQLSource(t *testing.T) {
	insertSource, err := NewInsertSQLBuilder().
		Into("sys_user").
		Value("name", "Alice").
		Value("status", "ACTIVE").
		Build()
	if err != nil {
		t.Fatalf("build insert SQL failed: %v", err)
	}
	insertCompiled, err := CompileSQLContext(context.Background(), insertSource.SQL, insertSource.Args, NewMySQLDialect())
	if err != nil {
		t.Fatalf("compile insert SQL failed: %v", err)
	}
	if insertCompiled.SQL != "INSERT INTO `sys_user` (`name`, `status`) VALUES (?, ?)" {
		t.Fatalf("unexpected insert SQL %q", insertCompiled.SQL)
	}
	if !reflect.DeepEqual(insertCompiled.Args, []any{"Alice", "ACTIVE"}) {
		t.Fatalf("unexpected insert args %#v", insertCompiled.Args)
	}

	updateSource, err := NewUpdateSQLBuilder().
		Table("sys_user").
		Set("status", "DISABLED").
		WhereEq("id", int64(7)).
		Build()
	if err != nil {
		t.Fatalf("build update SQL failed: %v", err)
	}
	updateCompiled, err := CompileSQLContext(context.Background(), updateSource.SQL, updateSource.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile update SQL failed: %v", err)
	}
	if updateCompiled.SQL != `UPDATE "sys_user" SET "status" = $1 WHERE "id" = $2` {
		t.Fatalf("unexpected update SQL %q", updateCompiled.SQL)
	}
	if !reflect.DeepEqual(updateCompiled.Args, []any{"DISABLED", int64(7)}) {
		t.Fatalf("unexpected update args %#v", updateCompiled.Args)
	}

	deleteSource, err := NewDeleteSQLBuilder().
		From("sys_user").
		WhereEq("id", int64(7)).
		Build()
	if err != nil {
		t.Fatalf("build delete SQL failed: %v", err)
	}
	deleteCompiled, err := CompileSQLContext(context.Background(), deleteSource.SQL, deleteSource.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile delete SQL failed: %v", err)
	}
	if deleteCompiled.SQL != `DELETE FROM "sys_user" WHERE "id" = $1` {
		t.Fatalf("unexpected delete SQL %q", deleteCompiled.SQL)
	}
	if !reflect.DeepEqual(deleteCompiled.Args, []any{int64(7)}) {
		t.Fatalf("unexpected delete args %#v", deleteCompiled.Args)
	}
}

func TestSelectSQLBuilder_whenUnsafeIdentifierProvided_shouldReject(t *testing.T) {
	_, err := NewSelectSQLBuilder().
		Select("id").
		From("sys_user; drop table sys_user").
		Build()
	if err == nil {
		t.Fatal("expected unsafe identifier error")
	}
}

func TestSelectSQLBuilder_whenAdvancedClausesProvided_shouldBuildSafeSQLSource(t *testing.T) {
	source, err := NewSelectSQLBuilder().
		Select("id", "name").
		From("sys_user").
		Join("sys_role", "sys_role.user_id = sys_user.id and sys_role.code = #{code}", NamedArgs{"code": "admin"}).
		WhereIn("status", []string{"ACTIVE", "LOCKED"}).
		WhereBetween("id", int64(1), int64(9)).
		WhereIsNull("deleted_at").
		ForUpdate(NewPostgresDialect(), RowLockOptions{SkipLocked: true}).
		Build()
	if err != nil {
		t.Fatalf("build advanced select SQL failed: %v", err)
	}

	compiled, err := CompileSQLContext(context.Background(), source.SQL, source.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile advanced select SQL failed: %v", err)
	}

	expectedSQL := `SELECT "id", "name" FROM "sys_user" JOIN "sys_role" ON sys_role.user_id = sys_user.id and sys_role.code = $1 WHERE "status" IN ($2, $3) AND "id" BETWEEN $4 AND $5 AND "deleted_at" IS NULL FOR UPDATE SKIP LOCKED`
	if compiled.SQL != expectedSQL {
		t.Fatalf("unexpected advanced select SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{"admin", "ACTIVE", "LOCKED", int64(1), int64(9)}) {
		t.Fatalf("unexpected advanced select args %#v", compiled.Args)
	}
}

func TestWriteSQLBuilders_whenReturningProvided_shouldBuildReturningClauses(t *testing.T) {
	insertSource, err := NewInsertSQLBuilder().
		Into("sys_user").
		Value("name", "Alice").
		Returning("id").
		Build()
	if err != nil {
		t.Fatalf("build insert returning SQL failed: %v", err)
	}
	insertCompiled, err := CompileSQLContext(context.Background(), insertSource.SQL, insertSource.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile insert returning SQL failed: %v", err)
	}
	if insertCompiled.SQL != `INSERT INTO "sys_user" ("name") VALUES ($1) RETURNING "id"` {
		t.Fatalf("unexpected insert returning SQL %q", insertCompiled.SQL)
	}

	updateSource, err := NewUpdateSQLBuilder().
		Table("sys_user").
		Set("name", "Alice").
		WhereIsNotNull("id").
		Returning("id", "name").
		RequireWhere().
		Build()
	if err != nil {
		t.Fatalf("build update returning SQL failed: %v", err)
	}
	updateCompiled, err := CompileSQLContext(context.Background(), updateSource.SQL, updateSource.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile update returning SQL failed: %v", err)
	}
	if updateCompiled.SQL != `UPDATE "sys_user" SET "name" = $1 WHERE "id" IS NOT NULL RETURNING "id", "name"` {
		t.Fatalf("unexpected update returning SQL %q", updateCompiled.SQL)
	}

	deleteSource, err := NewDeleteSQLBuilder().
		From("sys_user").
		WhereBetween("id", int64(1), int64(9)).
		Returning("id").
		RequireWhere().
		Build()
	if err != nil {
		t.Fatalf("build delete returning SQL failed: %v", err)
	}
	deleteCompiled, err := CompileSQLContext(context.Background(), deleteSource.SQL, deleteSource.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile delete returning SQL failed: %v", err)
	}
	if deleteCompiled.SQL != `DELETE FROM "sys_user" WHERE "id" BETWEEN $1 AND $2 RETURNING "id"` {
		t.Fatalf("unexpected delete returning SQL %q", deleteCompiled.SQL)
	}
}

func TestWriteSQLBuilders_whenRequireWhereWithoutCondition_shouldReject(t *testing.T) {
	if _, err := NewUpdateSQLBuilder().
		Table("sys_user").
		Set("name", "Alice").
		RequireWhere().
		Build(); err == nil {
		t.Fatal("expected update require where error")
	}
	if _, err := NewDeleteSQLBuilder().
		From("sys_user").
		RequireWhere().
		Build(); err == nil {
		t.Fatal("expected delete require where error")
	}
}

func TestSelectSQLBuilder_whenWhereInEmpty_shouldReject(t *testing.T) {
	_, err := NewSelectSQLBuilder().
		Select("id").
		From("sys_user").
		WhereIn("id").
		Build()
	if err == nil {
		t.Fatal("expected empty IN values error")
	}
}
