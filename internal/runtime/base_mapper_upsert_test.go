package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"
)

func TestBaseMapper_Upsert_whenPostgresConflictFieldsProvided_shouldUseNativeUpsert(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	result, err := mapper.Upsert(
		context.Background(),
		&baseMapperUser{ID: 7, Name: "Alice", Status: "ACTIVE"},
		[]Field[baseMapperUser]{baseMapperUserID},
		[]Field[baseMapperUser]{baseMapperUserName, baseMapperUserStatus},
	)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	if result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	expectedSQL := `INSERT INTO "sys_user" ("id", "name", "status") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "name" = $4, "status" = $5`
	if state.exec != expectedSQL {
		t.Fatalf("unexpected upsert SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: int64(7)},
		{Ordinal: 2, Value: "Alice"},
		{Ordinal: 3, Value: "ACTIVE"},
		{Ordinal: 4, Value: "Alice"},
		{Ordinal: 5, Value: "ACTIVE"},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected upsert args %#v", state.execArgs)
	}
}

func TestBaseMapper_Upsert_whenPostgresUpdateFieldsEmpty_shouldDoNothingOnConflict(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 0}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	result, err := mapper.Upsert(
		context.Background(),
		&baseMapperUser{ID: 7, Name: "Alice", Status: "ACTIVE"},
		[]Field[baseMapperUser]{baseMapperUserID},
		nil,
	)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	if result.RowsAffected != 0 {
		t.Fatalf("unexpected result %#v", result)
	}
	expectedSQL := `INSERT INTO "sys_user" ("id", "name", "status") VALUES ($1, $2, $3) ON CONFLICT ("id") DO NOTHING`
	if state.exec != expectedSQL {
		t.Fatalf("unexpected upsert SQL %q", state.exec)
	}
}

func TestBaseMapper_Upsert_whenSQLServerProvided_shouldUseMerge(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewSQLServerDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	result, err := mapper.Upsert(
		context.Background(),
		&baseMapperUser{ID: 7, Name: "Alice", Status: "ACTIVE"},
		[]Field[baseMapperUser]{baseMapperUserID},
		[]Field[baseMapperUser]{baseMapperUserName},
	)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	if result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	expectedSQL := "MERGE INTO [sys_user] goark_orm_target USING (SELECT @p1 AS [id], @p2 AS [name], @p3 AS [status]) goark_orm_source ON (goark_orm_target.[id] = goark_orm_source.[id]) WHEN MATCHED THEN UPDATE SET goark_orm_target.[name] = goark_orm_source.[name] WHEN NOT MATCHED THEN INSERT ([id], [name], [status]) VALUES (goark_orm_source.[id], goark_orm_source.[name], goark_orm_source.[status]);"
	if state.exec != expectedSQL {
		t.Fatalf("unexpected upsert SQL %q", state.exec)
	}
}

func TestBaseMapper_Upsert_whenOracleProvided_shouldUseMerge(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewOracleDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	result, err := mapper.Upsert(
		context.Background(),
		&baseMapperUser{ID: 7, Name: "Alice", Status: "ACTIVE"},
		[]Field[baseMapperUser]{baseMapperUserID},
		[]Field[baseMapperUser]{baseMapperUserName},
	)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	if result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	expectedSQL := `MERGE INTO "sys_user" goark_orm_target USING (SELECT :1 AS "id", :2 AS "name", :3 AS "status" FROM dual) goark_orm_source ON (goark_orm_target."id" = goark_orm_source."id") WHEN MATCHED THEN UPDATE SET goark_orm_target."name" = goark_orm_source."name" WHEN NOT MATCHED THEN INSERT ("id", "name", "status") VALUES (goark_orm_source."id", goark_orm_source."name", goark_orm_source."status")`
	if state.exec != expectedSQL {
		t.Fatalf("unexpected upsert SQL %q", state.exec)
	}
}

func TestBaseMapper_UpsertBatchSize_whenSessionSupportsBatch_shouldFlushUpserts(t *testing.T) {
	state := openTestSQLState(t)
	state.execResults = []driver.Result{
		testResult{rowsAffected: 1},
		testResult{rowsAffected: 1},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewMySQLDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	rows, err := mapper.UpsertBatchSize(
		context.Background(),
		[]baseMapperUser{
			{ID: 7, Name: "Alice", Status: "ACTIVE"},
			{ID: 8, Name: "Bob", Status: "LOCKED"},
		},
		[]Field[baseMapperUser]{baseMapperUserID},
		[]Field[baseMapperUser]{baseMapperUserName},
		1,
	)
	if err != nil {
		t.Fatalf("upsert batch failed: %v", err)
	}

	if rows != 2 {
		t.Fatalf("expected two affected rows, got %d", rows)
	}
	expectedSQL := "INSERT INTO `sys_user` (`id`, `name`, `status`) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE `name` = ?"
	if !reflect.DeepEqual(state.execs, []string{expectedSQL, expectedSQL}) {
		t.Fatalf("unexpected batch upsert SQL %#v", state.execs)
	}
}

func TestBaseMapper_Upsert_whenUpdateFieldIsPrimaryKey_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.Upsert(
		context.Background(),
		&baseMapperUser{ID: 7, Name: "Alice"},
		[]Field[baseMapperUser]{baseMapperUserID},
		[]Field[baseMapperUser]{baseMapperUserID},
	)

	if err == nil || !strings.Contains(err.Error(), "must not be primary key") {
		t.Fatalf("expected primary key update rejection, got %v", err)
	}
	if state.exec != "" {
		t.Fatalf("upsert should not execute SQL, got %q", state.exec)
	}
}
