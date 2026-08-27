package provider

import (
	"context"
	"reflect"
	"testing"

	orm "goark.dev/orm"
)

type User struct {
	ID     int64
	Name   string
	Status string
}

func TestProviderSQLBuilderExample_shouldCompileProviderSource(t *testing.T) {
	registry := orm.NewRegistry()
	err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
		"example.UserSQL.ListByStatus",
		func(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs) (orm.SQLSource, error) {
			return orm.NewSelectSQLBuilder().
				Select("id", "name", "status").
				From("sys_user").
				WhereIn("status", args["statuses"]).
				WhereIsNull("deleted_at").
				OrderByAsc("id").
				Limit(args["limit"]).
				CacheKey("tenant:" + args["tenant"].(string)).
				Build()
		},
		orm.WithSQLProviderCommands(orm.StatementCommandSelect),
		orm.WithSQLProviderStatements("example.UserMapper.ListByStatus"),
	))
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	descriptor, ok := registry.SQLProviderDescriptor("example.UserSQL.ListByStatus")
	if !ok {
		t.Fatal("expected provider descriptor")
	}

	source, err := descriptor.Provider(
		context.Background(),
		orm.StatementMeta{FullName: "example.UserMapper.ListByStatus", Command: orm.StatementCommandSelect},
		orm.NamedArgs{"statuses": []string{"ACTIVE", "LOCKED"}, "limit": 20, "tenant": "t01"},
	)
	if err != nil {
		t.Fatalf("provider failed: %v", err)
	}
	compiled, err := orm.CompileSQLContext(context.Background(), source.SQL, source.Args, orm.NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile provider SQL failed: %v", err)
	}

	expectedSQL := `SELECT "id", "name", "status" FROM "sys_user" WHERE "status" IN ($1, $2) AND "deleted_at" IS NULL ORDER BY "id" ASC LIMIT $3`
	if compiled.SQL != expectedSQL {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{"ACTIVE", "LOCKED", 20}) {
		t.Fatalf("unexpected args %#v", compiled.Args)
	}
	if source.CacheKey != "tenant:t01" {
		t.Fatalf("unexpected cache key %q", source.CacheKey)
	}
}

func TestDialectSQLHelperExample_shouldCompileUpsertAndRowLock(t *testing.T) {
	source, err := orm.BuildUpsertSQL(orm.NewPostgresDialect(), orm.UpsertSpec{
		Table:           "sys_user",
		InsertColumns:   []string{"id", "name", "status"},
		ConflictColumns: []string{"id"},
		UpdateColumns:   []string{"name", "status"},
		Values: orm.NamedArgs{
			"id":     int64(7),
			"name":   "Alice",
			"status": "ACTIVE",
		},
	})
	if err != nil {
		t.Fatalf("build upsert failed: %v", err)
	}
	compiled, err := orm.CompileSQLContext(context.Background(), source.SQL, source.Args, orm.NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile upsert failed: %v", err)
	}
	if compiled.SQL == "" || len(compiled.Args) != 3 {
		t.Fatalf("unexpected upsert result sql=%q args=%#v", compiled.SQL, compiled.Args)
	}

	lockClause, err := orm.RowLockClause(orm.NewPostgresDialect(), orm.RowLockOptions{SkipLocked: true})
	if err != nil {
		t.Fatalf("row lock clause failed: %v", err)
	}
	if lockClause != "FOR UPDATE SKIP LOCKED" {
		t.Fatalf("unexpected row lock clause %q", lockClause)
	}
}
