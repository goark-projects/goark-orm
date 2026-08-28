package orm

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestBuildUpsertSQL_whenPostgresProvided_shouldBuildOnConflictSource(t *testing.T) {
	source, err := BuildUpsertSQL(NewPostgresDialect(), UpsertSpec{
		Table:            "sys_user",
		InsertColumns:    []string{"id", "name", "status"},
		ConflictColumns:  []string{"id"},
		UpdateColumns:    []string{"name", "status"},
		ReturningColumns: []string{"id"},
		Values: NamedArgs{
			"id":     int64(7),
			"name":   "Alice",
			"status": "ACTIVE",
		},
	})
	if err != nil {
		t.Fatalf("build postgres upsert SQL failed: %v", err)
	}

	compiled, err := CompileSQLContext(context.Background(), source.SQL, source.Args, NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile postgres upsert SQL failed: %v", err)
	}

	expected := `INSERT INTO "sys_user" ("id", "name", "status") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "name" = EXCLUDED."name", "status" = EXCLUDED."status" RETURNING "id"`
	if compiled.SQL != expected {
		t.Fatalf("unexpected postgres upsert SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{int64(7), "Alice", "ACTIVE"}) {
		t.Fatalf("unexpected postgres upsert args %#v", compiled.Args)
	}
}

func TestBuildUpsertSQL_whenMySQLProvided_shouldBuildOnDuplicateKeySource(t *testing.T) {
	source, err := BuildUpsertSQL(NewMySQLDialect(), UpsertSpec{
		Table:         "sys_user",
		InsertColumns: []string{"id", "name"},
		UpdateColumns: []string{"name"},
		Values: NamedArgs{
			"id":   int64(7),
			"name": "Alice",
		},
	})
	if err != nil {
		t.Fatalf("build mysql upsert SQL failed: %v", err)
	}

	compiled, err := CompileSQLContext(context.Background(), source.SQL, source.Args, NewMySQLDialect())
	if err != nil {
		t.Fatalf("compile mysql upsert SQL failed: %v", err)
	}

	expected := "INSERT INTO `sys_user` (`id`, `name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `name` = VALUES(`name`)"
	if compiled.SQL != expected {
		t.Fatalf("unexpected mysql upsert SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{int64(7), "Alice"}) {
		t.Fatalf("unexpected mysql upsert args %#v", compiled.Args)
	}
}

func TestBuildUpsertSQL_whenSQLServerProvided_shouldBuildMergeSource(t *testing.T) {
	source, err := BuildUpsertSQL(NewSQLServerDialect(), UpsertSpec{
		Table:           "sys_user",
		InsertColumns:   []string{"id", "name", "status"},
		ConflictColumns: []string{"id"},
		UpdateColumns:   []string{"name", "status"},
		Values: NamedArgs{
			"id":     int64(7),
			"name":   "Alice",
			"status": "ACTIVE",
		},
	})
	if err != nil {
		t.Fatalf("build sqlserver upsert SQL failed: %v", err)
	}

	compiled, err := CompileSQLContext(context.Background(), source.SQL, source.Args, NewSQLServerDialect())
	if err != nil {
		t.Fatalf("compile sqlserver upsert SQL failed: %v", err)
	}

	expected := "MERGE INTO [sys_user] goark_orm_target USING (SELECT @p1 AS [id], @p2 AS [name], @p3 AS [status]) goark_orm_source ON (goark_orm_target.[id] = goark_orm_source.[id]) WHEN MATCHED THEN UPDATE SET goark_orm_target.[name] = goark_orm_source.[name], goark_orm_target.[status] = goark_orm_source.[status] WHEN NOT MATCHED THEN INSERT ([id], [name], [status]) VALUES (goark_orm_source.[id], goark_orm_source.[name], goark_orm_source.[status]);"
	if compiled.SQL != expected {
		t.Fatalf("unexpected sqlserver upsert SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{int64(7), "Alice", "ACTIVE"}) {
		t.Fatalf("unexpected sqlserver upsert args %#v", compiled.Args)
	}
}

func TestBuildUpsertSQL_whenOracleProvided_shouldBuildMergeSource(t *testing.T) {
	source, err := BuildUpsertSQL(NewOracleDialect(), UpsertSpec{
		Table:           "sys_user",
		InsertColumns:   []string{"id", "name"},
		ConflictColumns: []string{"id"},
		UpdateColumns:   []string{"name"},
		Values: NamedArgs{
			"id":   int64(7),
			"name": "Alice",
		},
	})
	if err != nil {
		t.Fatalf("build oracle upsert SQL failed: %v", err)
	}

	compiled, err := CompileSQLContext(context.Background(), source.SQL, source.Args, NewOracleDialect())
	if err != nil {
		t.Fatalf("compile oracle upsert SQL failed: %v", err)
	}

	expected := `MERGE INTO "sys_user" goark_orm_target USING (SELECT :1 AS "id", :2 AS "name" FROM dual) goark_orm_source ON (goark_orm_target."id" = goark_orm_source."id") WHEN MATCHED THEN UPDATE SET goark_orm_target."name" = goark_orm_source."name" WHEN NOT MATCHED THEN INSERT ("id", "name") VALUES (goark_orm_source."id", goark_orm_source."name")`
	if compiled.SQL != expected {
		t.Fatalf("unexpected oracle upsert SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{int64(7), "Alice"}) {
		t.Fatalf("unexpected oracle upsert args %#v", compiled.Args)
	}
}

func TestBuildUpsertSQL_whenUnsupportedDialectProvided_shouldReturnConfigurationError(t *testing.T) {
	_, err := BuildUpsertSQL(NewQuestionDialect(), UpsertSpec{
		Table:         "sys_user",
		InsertColumns: []string{"id"},
		UpdateColumns: []string{"id"},
		Values:        NamedArgs{"id": int64(7)},
	})
	if err == nil || !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected unsupported upsert configuration error, got %v", err)
	}
}

func TestRowLockClause_whenOptionsProvided_shouldUseDialectCapabilities(t *testing.T) {
	postgres, err := RowLockClause(NewPostgresDialect(), RowLockOptions{SkipLocked: true})
	if err != nil {
		t.Fatalf("postgres row lock clause failed: %v", err)
	}
	if postgres != "FOR UPDATE SKIP LOCKED" {
		t.Fatalf("unexpected postgres row lock clause %q", postgres)
	}

	oracle, err := RowLockClause(NewOracleDialect(), RowLockOptions{NoWait: true})
	if err != nil {
		t.Fatalf("oracle row lock clause failed: %v", err)
	}
	if oracle != "FOR UPDATE NOWAIT" {
		t.Fatalf("unexpected oracle row lock clause %q", oracle)
	}

	_, err = RowLockClause(NewSQLiteDialect(), RowLockOptions{})
	if err == nil || !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected sqlite row lock configuration error, got %v", err)
	}
}

func TestNewGeneratedKeyPlan_whenDialectProvided_shouldReturnExecutablePlan(t *testing.T) {
	postgres, err := NewGeneratedKeyPlan(NewPostgresDialect(), "id")
	if err != nil {
		t.Fatalf("postgres generated key plan failed: %v", err)
	}
	if postgres.Style != DialectGeneratedKeyReturning || postgres.SQLClause != `RETURNING "id"` || postgres.UsesLastInsertID {
		t.Fatalf("unexpected postgres generated key plan %#v", postgres)
	}

	mysql, err := NewGeneratedKeyPlan(NewMySQLDialect(), "id")
	if err != nil {
		t.Fatalf("mysql generated key plan failed: %v", err)
	}
	if mysql.Style != DialectGeneratedKeyLastInsertID || !mysql.UsesLastInsertID || mysql.SQLClause != "" {
		t.Fatalf("unexpected mysql generated key plan %#v", mysql)
	}
}
