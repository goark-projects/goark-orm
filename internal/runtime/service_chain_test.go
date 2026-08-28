package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestQueryChain_whenMyBatisPlusHelpersProvided_shouldRenderQuery(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	service := newBaseMapperUserService(t, state, NewPostgresDialect())

	records, err := service.ChainQuery().
		Select(baseMapperUserID, baseMapperUserName).
		Ne(baseMapperUserStatus, "DELETED").
		Gt(baseMapperUserID, int64(1)).
		Ge(baseMapperUserID, int64(7)).
		Lt(baseMapperUserID, int64(100)).
		Le(baseMapperUserID, int64(99)).
		In(baseMapperUserID, []int64{7, 8}).
		Between(baseMapperUserID, int64(7), int64(8)).
		IsNull(baseMapperUserStatus).
		Exists(`select 1 from sys_role r where r.user_id = id and r.code = #{role}`, NamedArgs{"role": "admin"}).
		Apply(`lower(name) = lower(#{name})`, NamedArgs{"name": "ALICE"}).
		GroupBy(baseMapperUserStatus).
		Having(`count(*) > #{min}`, NamedArgs{"min": int64(1)}).
		OrderByAsc(baseMapperUserName).
		OrderByDesc(baseMapperUserID).
		Last("FOR UPDATE").
		List(context.Background())
	if err != nil {
		t.Fatalf("query chain failed: %v", err)
	}

	if len(records) != 1 || records[0].ID != 7 || records[0].Name != "Alice" {
		t.Fatalf("unexpected records %#v", records)
	}
	expectedSQL := `SELECT "id", "name" FROM "sys_user" WHERE "status" <> $1 AND "id" > $2 AND "id" >= $3 AND "id" < $4 AND "id" <= $5 AND "id" IN ($6, $7) AND "id" BETWEEN $8 AND $9 AND "status" IS NULL AND EXISTS (select 1 from sys_role r where r.user_id = id and r.code = $10) AND lower(name) = lower($11) GROUP BY "status" HAVING count(*) > $12 ORDER BY "name" ASC, "id" DESC FOR UPDATE`
	if state.query != expectedSQL {
		t.Fatalf("unexpected query SQL %q", state.query)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "DELETED"},
		{Ordinal: 2, Value: int64(1)},
		{Ordinal: 3, Value: int64(7)},
		{Ordinal: 4, Value: int64(100)},
		{Ordinal: 5, Value: int64(99)},
		{Ordinal: 6, Value: int64(7)},
		{Ordinal: 7, Value: int64(8)},
		{Ordinal: 8, Value: int64(7)},
		{Ordinal: 9, Value: int64(8)},
		{Ordinal: 10, Value: "admin"},
		{Ordinal: 11, Value: "ALICE"},
		{Ordinal: 12, Value: int64(1)},
	}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestUpdateChain_whenMyBatisPlusHelpersProvided_shouldRenderUpdate(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	service := newBaseMapperUserService(t, state, NewPostgresDialect())

	rows, err := service.ChainUpdate().
		Set(baseMapperUserStatus, "LOCKED").
		SetSQL(`name = upper(#{name})`, NamedArgs{"name": "alice"}).
		Ne(baseMapperUserStatus, "DELETED").
		Gt(baseMapperUserID, int64(1)).
		Ge(baseMapperUserID, int64(7)).
		Lt(baseMapperUserID, int64(100)).
		Le(baseMapperUserID, int64(99)).
		In(baseMapperUserID, []int64{7, 8}).
		Between(baseMapperUserID, int64(1), int64(9)).
		IsNull(baseMapperUserName).
		Exists(`select 1 from sys_role r where r.user_id = id and r.code = #{role}`, NamedArgs{"role": "admin"}).
		Apply(`status <> #{archived}`, NamedArgs{"archived": "ARCHIVED"}).
		Last("RETURNING id").
		Update(context.Background())
	if err != nil {
		t.Fatalf("update chain failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("expected one updated row, got %d", rows)
	}
	expectedSQL := `UPDATE "sys_user" SET "status" = $1, name = upper($2) WHERE "status" <> $3 AND "id" > $4 AND "id" >= $5 AND "id" < $6 AND "id" <= $7 AND "id" IN ($8, $9) AND "id" BETWEEN $10 AND $11 AND "name" IS NULL AND EXISTS (select 1 from sys_role r where r.user_id = id and r.code = $12) AND status <> $13 RETURNING id`
	if state.exec != expectedSQL {
		t.Fatalf("unexpected update SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "LOCKED"},
		{Ordinal: 2, Value: "alice"},
		{Ordinal: 3, Value: "DELETED"},
		{Ordinal: 4, Value: int64(1)},
		{Ordinal: 5, Value: int64(7)},
		{Ordinal: 6, Value: int64(100)},
		{Ordinal: 7, Value: int64(99)},
		{Ordinal: 8, Value: int64(7)},
		{Ordinal: 9, Value: int64(8)},
		{Ordinal: 10, Value: int64(1)},
		{Ordinal: 11, Value: int64(9)},
		{Ordinal: 12, Value: "admin"},
		{Ordinal: 13, Value: "ARCHIVED"},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func newBaseMapperUserService(t testing.TB, state *testSQLState, dialect Dialect) *Service[baseMapperUser, int64] {
	t.Helper()
	session, err := NewSQLSession(NewRegistry(), state.db, dialect)
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
	return service
}
