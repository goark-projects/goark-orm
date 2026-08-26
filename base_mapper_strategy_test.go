package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"
)

func TestBaseMapper_Insert_whenColumnStrategyNotEmpty_shouldSkipEmptyColumns(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, EntityMeta{
		TypeName: "baseMapperUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", FieldType: "int64", ColumnName: "id", PrimaryKey: true, AutoIncrement: true},
			{FieldName: "Name", FieldType: "string", ColumnName: "name", InsertStrategy: FieldStrategyNotEmpty},
			{FieldName: "Status", FieldType: "string", ColumnName: "status", InsertStrategy: FieldStrategyNotEmpty},
		},
	})
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.Insert(context.Background(), &baseMapperUser{Name: "Alice"})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if state.exec != `INSERT INTO "sys_user" ("name") VALUES ($1)` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: "Alice"}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_Insert_whenGlobalStrategyNotEmpty_shouldSkipEmptyColumns(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	config := DefaultConfiguration()
	config.GlobalConfig.DbConfig.InsertStrategy = FieldStrategyNotEmpty
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.Insert(context.Background(), &baseMapperUser{Status: "ACTIVE"})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if state.exec != `INSERT INTO "sys_user" ("status") VALUES ($1)` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_UpdateByID_whenGlobalStrategyNotEmpty_shouldSkipEmptyColumns(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	config := DefaultConfiguration()
	config.GlobalConfig.DbConfig.UpdateStrategy = FieldStrategyNotEmpty
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	rows, err := mapper.UpdateByID(context.Background(), &baseMapperUser{ID: 7, Status: "LOCKED"})
	if err != nil {
		t.Fatalf("update by id failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if state.exec != `UPDATE "sys_user" SET "status" = $1 WHERE "id" = $2` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "LOCKED"}, {Ordinal: 2, Value: int64(7)}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_UpdateByID_whenStrategySkipsEveryColumn_shouldRejectEmptySet(t *testing.T) {
	state := openTestSQLState(t)
	config := DefaultConfiguration()
	config.GlobalConfig.DbConfig.UpdateStrategy = FieldStrategyNotEmpty
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.UpdateByID(context.Background(), &baseMapperUser{ID: 7})
	if err == nil || !strings.Contains(err.Error(), "no updatable columns") {
		t.Fatalf("expected empty update error, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("update should not execute SQL, got %q", state.exec)
	}
}

func TestBaseMapper_SelectByID_whenColumnSelectDisabled_shouldUseDefaultProjection(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, EntityMeta{
		TypeName: "baseMapperUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", FieldType: "int64", ColumnName: "id", PrimaryKey: true, AutoIncrement: true},
			{FieldName: "Name", FieldType: "string", ColumnName: "name"},
			{FieldName: "Status", FieldType: "string", ColumnName: "status", SelectDisabled: true},
		},
	})
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	user, err := mapper.SelectByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("select by id failed: %v", err)
	}

	if user == nil || user.ID != 7 || user.Name != "Alice" || user.Status != "" {
		t.Fatalf("unexpected user %#v", user)
	}
	if state.query != `SELECT "id", "name" FROM "sys_user" WHERE "id" = $1` {
		t.Fatalf("unexpected SQL %q", state.query)
	}
}
