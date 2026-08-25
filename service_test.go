package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestService_whenCrudMethodsCalled_shouldDelegateToBaseMapper(t *testing.T) {
	state := openTestSQLState(t)
	state.execResults = []driver.Result{
		testResult{rowsAffected: 1, lastInsertID: 42},
		testResult{rowsAffected: 1},
	}
	state.queryRows = testRowsData{
		columns: []string{"id", "name", "status"},
		values:  [][]driver.Value{{int64(7), "Alice", "ACTIVE"}},
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

	saved, err := service.Save(context.Background(), &baseMapperUser{Name: "Alice", Status: "ACTIVE"})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	user, err := service.GetByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}
	removed, err := service.RemoveByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("remove by id failed: %v", err)
	}

	if !saved || !removed {
		t.Fatalf("expected save/remove to report success, saved=%v removed=%v", saved, removed)
	}
	if user == nil || user.ID != 7 || user.Name != "Alice" || user.Status != "ACTIVE" {
		t.Fatalf("unexpected user %#v", user)
	}
	expectedExecs := []string{
		`INSERT INTO "sys_user" ("name", "status") VALUES ($1, $2)`,
		`DELETE FROM "sys_user" WHERE "id" = $1`,
	}
	if !reflect.DeepEqual(state.execs, expectedExecs) {
		t.Fatalf("unexpected execs %#v", state.execs)
	}
}

func TestService_ChainQuery_whenConditionsProvided_shouldQueryList(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name", "status"},
		values:  [][]driver.Value{{int64(7), "Alice", "ACTIVE"}},
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

	records, err := service.ChainQuery().
		Eq(baseMapperUserStatus, "ACTIVE").
		OrderByDesc(baseMapperUserID).
		List(context.Background())
	if err != nil {
		t.Fatalf("chain query failed: %v", err)
	}

	if len(records) != 1 || records[0].ID != 7 || records[0].Name != "Alice" {
		t.Fatalf("unexpected chain query records %#v", records)
	}
	expectedQuery := `SELECT "id", "name", "status" FROM "sys_user" WHERE "status" = $1 ORDER BY "id" DESC`
	if state.query != expectedQuery {
		t.Fatalf("unexpected chain query SQL %q", state.query)
	}
}

func TestService_ChainUpdate_whenSetAndConditionProvided_shouldUpdateRows(t *testing.T) {
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
	service, err := NewService[baseMapperUser, int64](mapper)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	rows, err := service.ChainUpdate().
		Set(baseMapperUserName, "Alice").
		Eq(baseMapperUserID, int64(7)).
		Update(context.Background())
	if err != nil {
		t.Fatalf("chain update failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("expected one updated row, got %d", rows)
	}
	expectedSQL := `UPDATE "sys_user" SET "name" = $1 WHERE "id" = $2`
	if state.exec != expectedSQL {
		t.Fatalf("unexpected chain update SQL %q", state.exec)
	}
}
