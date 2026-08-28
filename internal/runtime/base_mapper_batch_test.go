package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestBaseMapper_InsertBatchSize_whenSessionSupportsBatch_shouldFlushAndReturnRows(t *testing.T) {
	state := openTestSQLState(t)
	state.execResults = []driver.Result{
		testResult{rowsAffected: 1},
		testResult{rowsAffected: 1},
		testResult{rowsAffected: 1},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	rows, err := mapper.InsertBatchSize(context.Background(), []baseMapperUser{
		{Name: "Alice", Status: "ACTIVE"},
		{Name: "Bob", Status: "LOCKED"},
		{Name: "Carol", Status: "ACTIVE"},
	}, 2)
	if err != nil {
		t.Fatalf("insert batch failed: %v", err)
	}

	if rows != 3 {
		t.Fatalf("expected three affected rows, got %d", rows)
	}
	expectedExecs := []string{
		`INSERT INTO "sys_user" ("name", "status") VALUES ($1, $2)`,
		`INSERT INTO "sys_user" ("name", "status") VALUES ($1, $2)`,
		`INSERT INTO "sys_user" ("name", "status") VALUES ($1, $2)`,
	}
	if !reflect.DeepEqual(state.execs, expectedExecs) {
		t.Fatalf("unexpected batch insert SQL %#v", state.execs)
	}
	expectedArgs := [][]driver.NamedValue{
		{{Ordinal: 1, Value: "Alice"}, {Ordinal: 2, Value: "ACTIVE"}},
		{{Ordinal: 1, Value: "Bob"}, {Ordinal: 2, Value: "LOCKED"}},
		{{Ordinal: 1, Value: "Carol"}, {Ordinal: 2, Value: "ACTIVE"}},
	}
	if !reflect.DeepEqual(state.execArgsList, expectedArgs) {
		t.Fatalf("unexpected batch insert args %#v", state.execArgsList)
	}
}

func TestService_UpdateBatchByIDSize_whenSessionSupportsBatch_shouldUpdateRows(t *testing.T) {
	state := openTestSQLState(t)
	state.execResults = []driver.Result{
		testResult{rowsAffected: 1},
		testResult{rowsAffected: 2},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	service, err := NewService[baseMapperUser, int64](mapper)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	rows, err := service.UpdateBatchByIDSize(context.Background(), []baseMapperUser{
		{ID: 7, Name: "Alice", Status: "ACTIVE"},
		{ID: 8, Name: "Bob", Status: "LOCKED"},
	}, 1)
	if err != nil {
		t.Fatalf("update batch failed: %v", err)
	}

	if rows != 3 {
		t.Fatalf("expected three affected rows, got %d", rows)
	}
	expectedExecs := []string{
		`UPDATE "sys_user" SET "name" = $1, "status" = $2 WHERE "id" = $3`,
		`UPDATE "sys_user" SET "name" = $1, "status" = $2 WHERE "id" = $3`,
	}
	if !reflect.DeepEqual(state.execs, expectedExecs) {
		t.Fatalf("unexpected batch update SQL %#v", state.execs)
	}
	expectedArgs := [][]driver.NamedValue{
		{{Ordinal: 1, Value: "Alice"}, {Ordinal: 2, Value: "ACTIVE"}, {Ordinal: 3, Value: int64(7)}},
		{{Ordinal: 1, Value: "Bob"}, {Ordinal: 2, Value: "LOCKED"}, {Ordinal: 3, Value: int64(8)}},
	}
	if !reflect.DeepEqual(state.execArgsList, expectedArgs) {
		t.Fatalf("unexpected batch update args %#v", state.execArgsList)
	}
}
