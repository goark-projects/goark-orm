package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestBaseMapper_SelectOneAndByMap_whenConditionsProvided_shouldQueryExpectedSQL(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{
			columns: []string{"id", "name", "status"},
			values:  [][]driver.Value{{int64(7), "Alice", "ACTIVE"}},
		},
		{
			columns: []string{"id", "name", "status"},
			values:  [][]driver.Value{{int64(8), "", "ACTIVE"}},
		},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	user, err := mapper.SelectOne(context.Background(), NewQueryWrapper[baseMapperUser]().Eq(baseMapperUserStatus, "ACTIVE"))
	if err != nil {
		t.Fatalf("select one failed: %v", err)
	}
	records, err := mapper.SelectByMap(context.Background(), map[string]any{"status": "ACTIVE", "name": nil})
	if err != nil {
		t.Fatalf("select by map failed: %v", err)
	}

	if user == nil || user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
	if len(records) != 1 || records[0].ID != 8 {
		t.Fatalf("unexpected records %#v", records)
	}
	expectedQueries := []string{
		`SELECT "id", "name", "status" FROM "sys_user" WHERE "status" = $1`,
		`SELECT "id", "name", "status" FROM "sys_user" WHERE "name" IS NULL AND "status" = $1`,
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
}

func TestBaseMapper_SelectMapsPage_whenWrapperProvided_shouldCountAndQueryMaps(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"count"}, values: [][]driver.Value{{int64(2)}}},
		{
			columns: []string{"id", "name"},
			values: [][]driver.Value{
				{int64(7), "Alice"},
				{int64(8), "Bob"},
			},
		},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	page, err := mapper.SelectMapsPage(
		context.Background(),
		NewPageRequest(1, 2),
		NewQueryWrapper[baseMapperUser]().
			Select(baseMapperUserID, baseMapperUserName).
			Eq(baseMapperUserStatus, "ACTIVE"),
	)
	if err != nil {
		t.Fatalf("select maps page failed: %v", err)
	}

	if page.Total != 2 || len(page.Records) != 2 || page.Records[0]["id"] != int64(7) {
		t.Fatalf("unexpected page %#v", page)
	}
	expectedQueries := []string{
		`SELECT COUNT(*) FROM (SELECT "id", "name" FROM "sys_user" WHERE "status" = $1) goark_orm_count`,
		`SELECT "id", "name" FROM "sys_user" WHERE "status" = $1 LIMIT $2 OFFSET $3`,
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
}

func TestBaseMapper_DeleteByMap_whenConditionsProvided_shouldDeleteRows(t *testing.T) {
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

	rows, err := mapper.DeleteByMap(context.Background(), map[string]any{"status": "LOCKED"})
	if err != nil {
		t.Fatalf("delete by map failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if state.exec != `DELETE FROM "sys_user" WHERE "status" = $1` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
}

func TestBaseMapper_DeleteByMap_whenMapEmpty_shouldRejectFullTableDelete(t *testing.T) {
	state := openTestSQLState(t)
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.DeleteByMap(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected empty column map to fail")
	}
	if state.exec != "" {
		t.Fatalf("delete should not execute SQL, got %q", state.exec)
	}
}
