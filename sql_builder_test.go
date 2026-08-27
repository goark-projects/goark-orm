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
