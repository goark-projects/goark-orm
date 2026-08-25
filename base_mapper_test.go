package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
	"time"
)

type baseMapperUser struct {
	ID     int64
	Name   string
	Status string
}

type baseMapperAuditUser struct {
	ID        int64
	Name      string
	Version   int64
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	baseMapperUserID     = Field[baseMapperUser]{Column: "id"}
	baseMapperUserName   = Field[baseMapperUser]{Column: "name"}
	baseMapperUserStatus = Field[baseMapperUser]{Column: "status"}
)

func TestQueryWrapper_whenConditionsAndOrders_shouldRenderSafeSQL(t *testing.T) {
	wrapper := NewQueryWrapper[baseMapperUser]().
		Eq(baseMapperUserStatus, "ACTIVE").
		Like(baseMapperUserName, "%Alice%").
		In(baseMapperUserID, []int64{7, 8}).
		OrderByDesc(baseMapperUserID)

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build wrapper failed: %v", err)
	}

	if rendered.WhereSQL != `"status" = #{__goark_orm_w_0} AND "name" LIKE #{__goark_orm_w_1} AND "id" IN (#{__goark_orm_w_2}, #{__goark_orm_w_3})` {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	if rendered.OrderSQL != `ORDER BY "id" DESC` {
		t.Fatalf("unexpected order SQL %q", rendered.OrderSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_w_0": "ACTIVE",
		"__goark_orm_w_1": "%Alice%",
		"__goark_orm_w_2": int64(7),
		"__goark_orm_w_3": int64(8),
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestQueryWrapper_whenNestedRawAndGroupingProvided_shouldRenderSQL(t *testing.T) {
	wrapper := NewQueryWrapper[baseMapperUser]().
		Eq(baseMapperUserStatus, "ACTIVE").
		Or(func(child *QueryWrapper[baseMapperUser]) {
			child.Eq(baseMapperUserName, "Alice").Eq(baseMapperUserID, int64(7))
		}).
		Apply(`lower(name) = lower(#{name})`, NamedArgs{"name": "ALICE"}).
		Exists(`select 1 from sys_role r where r.user_id = id and r.code = #{role}`, NamedArgs{"role": "admin"}).
		GroupBy(baseMapperUserStatus).
		Having(`count(*) > #{min}`, NamedArgs{"min": int64(1)}).
		OrderByDesc(baseMapperUserID).
		Last("FOR UPDATE")

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build wrapper failed: %v", err)
	}

	expectedWhere := `"status" = #{__goark_orm_w_0} OR ("name" = #{__goark_orm_w_1} AND "id" = #{__goark_orm_w_2}) AND lower(name) = lower(#{__goark_orm_w_3}) AND EXISTS (select 1 from sys_role r where r.user_id = id and r.code = #{__goark_orm_w_4})`
	if rendered.WhereSQL != expectedWhere {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	if rendered.GroupSQL != `GROUP BY "status"` {
		t.Fatalf("unexpected group SQL %q", rendered.GroupSQL)
	}
	if rendered.HavingSQL != `HAVING count(*) > #{__goark_orm_w_5}` {
		t.Fatalf("unexpected having SQL %q", rendered.HavingSQL)
	}
	if rendered.LastSQL != "FOR UPDATE" {
		t.Fatalf("unexpected last SQL %q", rendered.LastSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_w_0": "ACTIVE",
		"__goark_orm_w_1": "Alice",
		"__goark_orm_w_2": int64(7),
		"__goark_orm_w_3": "ALICE",
		"__goark_orm_w_4": "admin",
		"__goark_orm_w_5": int64(1),
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestQueryWrapper_whenPlusOperatorsProvided_shouldRenderSQL(t *testing.T) {
	wrapper := NewQueryWrapper[baseMapperUser]().
		NotLike(baseMapperUserName, "Root%").
		LikeLeft(baseMapperUserName, "ice").
		LikeRight(baseMapperUserStatus, "ACT").
		NotIn(baseMapperUserID, []int64{9, 10}).
		Between(baseMapperUserID, int64(1), int64(8)).
		NotBetween(baseMapperUserID, int64(100), int64(200)).
		AllEq(map[Field[baseMapperUser]]any{
			baseMapperUserStatus: nil,
			baseMapperUserName:   "Alice",
		}).
		OrderBy(false, true, baseMapperUserID).
		OrderBy(true, true, baseMapperUserName, baseMapperUserStatus)

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build wrapper failed: %v", err)
	}

	expectedWhere := `"name" NOT LIKE #{__goark_orm_w_0} AND "name" LIKE #{__goark_orm_w_1} AND "status" LIKE #{__goark_orm_w_2} AND "id" NOT IN (#{__goark_orm_w_3}, #{__goark_orm_w_4}) AND "id" BETWEEN #{__goark_orm_w_5} AND #{__goark_orm_w_6} AND "id" NOT BETWEEN #{__goark_orm_w_7} AND #{__goark_orm_w_8} AND "name" = #{__goark_orm_w_9} AND "status" IS NULL`
	if rendered.WhereSQL != expectedWhere {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	if rendered.OrderSQL != `ORDER BY "name" ASC, "status" ASC` {
		t.Fatalf("unexpected order SQL %q", rendered.OrderSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_w_0": "Root%",
		"__goark_orm_w_1": "%ice",
		"__goark_orm_w_2": "ACT%",
		"__goark_orm_w_3": int64(9),
		"__goark_orm_w_4": int64(10),
		"__goark_orm_w_5": int64(1),
		"__goark_orm_w_6": int64(8),
		"__goark_orm_w_7": int64(100),
		"__goark_orm_w_8": int64(200),
		"__goark_orm_w_9": "Alice",
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestBaseMapper_SelectPage_whenWrapperProvided_shouldCountAndQueryRecords(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"count"}, values: [][]driver.Value{{int64(2)}}},
		{
			columns: []string{"id", "name", "status"},
			values: [][]driver.Value{
				{int64(7), "Alice", "ACTIVE"},
				{int64(8), "Bob", "ACTIVE"},
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

	page, err := mapper.SelectPage(
		context.Background(),
		NewPageRequest(2, 10),
		NewQueryWrapper[baseMapperUser]().
			Eq(baseMapperUserStatus, "ACTIVE").
			OrderByDesc(baseMapperUserID),
	)
	if err != nil {
		t.Fatalf("select page failed: %v", err)
	}

	if page.Total != 2 || page.Current != 2 || page.Size != 10 {
		t.Fatalf("unexpected page metadata %#v", page)
	}
	if len(page.Records) != 2 || page.Records[0].ID != 7 || page.Records[1].Name != "Bob" {
		t.Fatalf("unexpected records %#v", page.Records)
	}
	expectedQueries := []string{
		`SELECT COUNT(*) FROM (SELECT "id", "name", "status" FROM "sys_user" WHERE "status" = $1) goark_orm_count`,
		`SELECT "id", "name", "status" FROM "sys_user" WHERE "status" = $1 ORDER BY "id" DESC LIMIT $2 OFFSET $3`,
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
	expectedArgs := [][]driver.NamedValue{
		{{Ordinal: 1, Value: "ACTIVE"}},
		{{Ordinal: 1, Value: "ACTIVE"}, {Ordinal: 2, Value: int64(10)}, {Ordinal: 3, Value: int64(10)}},
	}
	if !reflect.DeepEqual(state.queryArgsList, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgsList)
	}
}

func TestBaseMapper_SelectMapsAndObjs_whenWrapperProvided_shouldScanMapAndFirstColumn(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{
			columns: []string{"id", "name", "status"},
			values:  [][]driver.Value{{int64(7), "Alice", "ACTIVE"}},
		},
		{
			columns: []string{"id"},
			values:  [][]driver.Value{{int64(7)}, {int64(8)}},
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

	maps, err := mapper.SelectMaps(context.Background(), NewQueryWrapper[baseMapperUser]().Eq(baseMapperUserStatus, "ACTIVE"))
	if err != nil {
		t.Fatalf("select maps failed: %v", err)
	}
	objs, err := mapper.SelectObjs(context.Background(), NewQueryWrapper[baseMapperUser]().Eq(baseMapperUserStatus, "ACTIVE"))
	if err != nil {
		t.Fatalf("select objs failed: %v", err)
	}

	if len(maps) != 1 || maps[0]["id"] != int64(7) || maps[0]["name"] != "Alice" {
		t.Fatalf("unexpected maps %#v", maps)
	}
	if !reflect.DeepEqual(objs, []any{int64(7), int64(8)}) {
		t.Fatalf("unexpected objs %#v", objs)
	}
	expectedQueries := []string{
		`SELECT "id", "name", "status" FROM "sys_user" WHERE "status" = $1`,
		`SELECT "id" FROM "sys_user" WHERE "status" = $1`,
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
}

func TestBaseMapper_SelectList_whenWrapperSelectProvided_shouldUseProjection(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"name", "status"},
		values:  [][]driver.Value{{"Alice", "ACTIVE"}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	users, err := mapper.SelectList(
		context.Background(),
		NewQueryWrapper[baseMapperUser]().
			Select(baseMapperUserName, baseMapperUserStatus).
			Eq(baseMapperUserStatus, "ACTIVE"),
	)
	if err != nil {
		t.Fatalf("select list failed: %v", err)
	}

	if len(users) != 1 || users[0].ID != 0 || users[0].Name != "Alice" || users[0].Status != "ACTIVE" {
		t.Fatalf("unexpected users %#v", users)
	}
	if state.query != `SELECT "name", "status" FROM "sys_user" WHERE "status" = $1` {
		t.Fatalf("unexpected query %q", state.query)
	}
}

func TestBaseMapper_SelectByID_whenRecordExists_shouldQueryPrimaryKey(t *testing.T) {
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

	user, err := mapper.SelectByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("select by id failed: %v", err)
	}

	if user == nil || user.ID != 7 || user.Name != "Alice" || user.Status != "ACTIVE" {
		t.Fatalf("unexpected user %#v", user)
	}
	if state.query != `SELECT "id", "name", "status" FROM "sys_user" WHERE "id" = $1` {
		t.Fatalf("unexpected query %q", state.query)
	}
	if !reflect.DeepEqual(state.queryArgs, []driver.NamedValue{{Ordinal: 1, Value: int64(7)}}) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestBaseMapper_SelectByID_whenSoftDeleteFieldPresent_shouldFilterLiveRows(t *testing.T) {
	state := openTestSQLState(t)
	now := time.Date(2026, 8, 21, 10, 30, 0, 0, time.UTC)
	state.queryRows = testRowsData{
		columns: []string{"id", "name", "version", "deleted", "created_at", "updated_at"},
		values:  [][]driver.Value{{int64(7), "Alice", int64(3), false, now, now}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperAuditUser, int64](session, baseMapperAuditUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	user, err := mapper.SelectByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("select by id failed: %v", err)
	}

	if user == nil || user.ID != 7 || user.Deleted {
		t.Fatalf("unexpected user %#v", user)
	}
	if state.query != `SELECT "id", "name", "version", "deleted", "created_at", "updated_at" FROM "sys_user" WHERE "id" = $1 AND "deleted" = $2` {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: int64(7)}, {Ordinal: 2, Value: false}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestBaseMapper_SelectList_whenSoftDeleteFieldPresent_shouldAppendLiveCondition(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name", "version", "deleted", "created_at", "updated_at"},
		values:  nil,
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperAuditUser, int64](session, baseMapperAuditUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.SelectList(context.Background(), NewQueryWrapper[baseMapperAuditUser]().Eq(NewField[baseMapperAuditUser]("name"), "Alice"))
	if err != nil {
		t.Fatalf("select list failed: %v", err)
	}

	if state.query != `SELECT "id", "name", "version", "deleted", "created_at", "updated_at" FROM "sys_user" WHERE "name" = $1 AND "deleted" = $2` {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "Alice"}, {Ordinal: 2, Value: false}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestBaseMapper_Insert_whenAutoIncrementPrimaryKey_shouldSkipPrimaryKey(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1, lastInsertID: 42}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	result, err := mapper.Insert(context.Background(), &baseMapperUser{Name: "Alice", Status: "ACTIVE"})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if result.LastInsertID != 42 || result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	if state.exec != `INSERT INTO "sys_user" ("name", "status") VALUES ($1, $2)` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "Alice"}, {Ordinal: 2, Value: "ACTIVE"}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_Insert_whenAssignIDPrimaryKey_shouldGenerateAndInsertPrimaryKey(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](
		session,
		EntityMeta{
			TypeName: "baseMapperUser",
			Table:    "sys_user",
			Columns: []ColumnMeta{
				{FieldName: "ID", FieldType: "int64", ColumnName: "id", PrimaryKey: true, IDType: IDTypeAssignID},
				{FieldName: "Name", FieldType: "string", ColumnName: "name"},
				{FieldName: "Status", FieldType: "string", ColumnName: "status"},
			},
		},
		WithBaseMapperIdentifierGenerator(fixedIdentifierGenerator{id: int64(9001)}),
	)
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	user := &baseMapperUser{Name: "Alice", Status: "ACTIVE"}

	result, err := mapper.Insert(context.Background(), user)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if user.ID != 9001 {
		t.Fatalf("expected generated id to be assigned, got %#v", user)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	if state.exec != `INSERT INTO "sys_user" ("id", "name", "status") VALUES ($1, $2, $3)` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: int64(9001)}, {Ordinal: 2, Value: "Alice"}, {Ordinal: 3, Value: "ACTIVE"}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_Insert_whenAssignUUIDPrimaryKey_shouldGenerateAndInsertPrimaryKey(t *testing.T) {
	type uuidUser struct {
		ID   string
		Name string
	}
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[uuidUser, string](
		session,
		EntityMeta{
			TypeName: "uuidUser",
			Table:    "sys_user",
			Columns: []ColumnMeta{
				{FieldName: "ID", FieldType: "string", ColumnName: "id", PrimaryKey: true, IDType: IDTypeAssignUUID},
				{FieldName: "Name", FieldType: "string", ColumnName: "name"},
			},
		},
		WithBaseMapperIdentifierGenerator(fixedIdentifierGenerator{uuid: "uuid-1"}),
	)
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	user := &uuidUser{Name: "Alice"}

	_, err = mapper.Insert(context.Background(), user)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if user.ID != "uuid-1" {
		t.Fatalf("expected generated uuid to be assigned, got %#v", user)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "uuid-1"}, {Ordinal: 2, Value: "Alice"}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_Insert_whenAutoTimeFieldsPresent_shouldFillCreatedAndUpdatedAt(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1, lastInsertID: 42}
	now := time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC)
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperAuditUser, int64](session, baseMapperAuditUserEntity(), WithBaseMapperClock(func() time.Time {
		return now
	}))
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	user := &baseMapperAuditUser{Name: "Alice"}

	_, err = mapper.Insert(context.Background(), user)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if !user.CreatedAt.Equal(now) || !user.UpdatedAt.Equal(now) {
		t.Fatalf("expected auto time fields to be filled, got %#v", user)
	}
	if state.exec != `INSERT INTO "sys_user" ("name", "version", "deleted", "created_at", "updated_at") VALUES ($1, $2, $3, $4, $5)` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: int64(0)},
		{Ordinal: 3, Value: false},
		{Ordinal: 4, Value: now},
		{Ordinal: 5, Value: now},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_UpdateByID_whenEntityProvided_shouldUpdateNonPrimaryColumns(t *testing.T) {
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

	rows, err := mapper.UpdateByID(context.Background(), &baseMapperUser{ID: 7, Name: "Alice", Status: "LOCKED"})
	if err != nil {
		t.Fatalf("update by id failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if state.exec != `UPDATE "sys_user" SET "name" = $1, "status" = $2 WHERE "id" = $3` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: "LOCKED"},
		{Ordinal: 3, Value: int64(7)},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_UpdateByID_whenVersionAndAutoTimeFieldsPresent_shouldUseOptimisticLock(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperAuditUser, int64](session, baseMapperAuditUserEntity(), WithBaseMapperClock(func() time.Time {
		return now
	}))
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	user := &baseMapperAuditUser{ID: 7, Name: "Alice", Version: 3}

	rows, err := mapper.UpdateByID(context.Background(), user)
	if err != nil {
		t.Fatalf("update by id failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if !user.UpdatedAt.Equal(now) {
		t.Fatalf("expected updated-at field to be filled, got %#v", user)
	}
	if state.exec != `UPDATE "sys_user" SET "name" = $1, "updated_at" = $2, "version" = "version" + 1 WHERE "id" = $3 AND "version" = $4 AND "deleted" = $5` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: now},
		{Ordinal: 3, Value: int64(7)},
		{Ordinal: 4, Value: int64(3)},
		{Ordinal: 5, Value: false},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_SaveOrUpdate_whenPrimaryKeyZero_shouldInsert(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1, lastInsertID: 42}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	result, err := mapper.SaveOrUpdate(context.Background(), &baseMapperUser{Name: "Alice", Status: "ACTIVE"})
	if err != nil {
		t.Fatalf("save or update failed: %v", err)
	}

	if result.LastInsertID != 42 || result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	if state.exec != `INSERT INTO "sys_user" ("name", "status") VALUES ($1, $2)` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
}

func TestBaseMapper_SaveOrUpdate_whenPrimaryKeyPresent_shouldUpdateByID(t *testing.T) {
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

	result, err := mapper.SaveOrUpdate(context.Background(), &baseMapperUser{ID: 7, Name: "Alice", Status: "ACTIVE"})
	if err != nil {
		t.Fatalf("save or update failed: %v", err)
	}

	if result.RowsAffected != 1 || result.LastInsertID != 0 {
		t.Fatalf("unexpected result %#v", result)
	}
	if state.exec != `UPDATE "sys_user" SET "name" = $1, "status" = $2 WHERE "id" = $3` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
}

func TestBaseMapper_DeleteByID_whenPrimaryKeyProvided_shouldDeleteOneRecord(t *testing.T) {
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

	rows, err := mapper.DeleteByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("delete by id failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if state.exec != `DELETE FROM "sys_user" WHERE "id" = $1` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: int64(7)}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_DeleteByID_whenSoftDeleteFieldPresent_shouldUpdateDeletedFlag(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperAuditUser, int64](session, baseMapperAuditUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	rows, err := mapper.DeleteByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("delete by id failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if state.exec != `UPDATE "sys_user" SET "deleted" = $1 WHERE "id" = $2 AND "deleted" = $3` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: true},
		{Ordinal: 2, Value: int64(7)},
		{Ordinal: 3, Value: false},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_Delete_whenSoftDeleteFieldPresent_shouldUpdateMatchedLiveRows(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperAuditUser, int64](session, baseMapperAuditUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	rows, err := mapper.Delete(context.Background(), NewQueryWrapper[baseMapperAuditUser]().Eq(NewField[baseMapperAuditUser]("name"), "Alice"))
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if state.exec != `UPDATE "sys_user" SET "deleted" = $1 WHERE "name" = $2 AND "deleted" = $3` {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: true},
		{Ordinal: 2, Value: "Alice"},
		{Ordinal: 3, Value: false},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestBaseMapper_Delete_whenWrapperEmpty_shouldRejectFullTableDelete(t *testing.T) {
	state := openTestSQLState(t)
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.Delete(context.Background(), NewQueryWrapper[baseMapperUser]())
	if err == nil {
		t.Fatalf("expected empty wrapper delete to fail")
	}
	if state.exec != "" {
		t.Fatalf("delete should not execute SQL, got %q", state.exec)
	}
}

func baseMapperAuditUserEntity() EntityMeta {
	return EntityMeta{
		TypeName: "baseMapperAuditUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", FieldType: "int64", ColumnName: "id", PrimaryKey: true, AutoIncrement: true},
			{FieldName: "Name", FieldType: "string", ColumnName: "name"},
			{FieldName: "Version", FieldType: "int64", ColumnName: "version", Version: true},
			{FieldName: "Deleted", FieldType: "bool", ColumnName: "deleted", SoftDelete: true},
			{FieldName: "CreatedAt", FieldType: "time.Time", ColumnName: "created_at", CreatedAt: true},
			{FieldName: "UpdatedAt", FieldType: "time.Time", ColumnName: "updated_at", UpdatedAt: true},
		},
	}
}

func baseMapperUserEntity() EntityMeta {
	return EntityMeta{
		TypeName: "baseMapperUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", FieldType: "int64", ColumnName: "id", PrimaryKey: true, AutoIncrement: true},
			{FieldName: "Name", FieldType: "string", ColumnName: "name"},
			{FieldName: "Status", FieldType: "string", ColumnName: "status"},
		},
	}
}

type fixedIdentifierGenerator struct {
	id   any
	uuid string
}

func (g fixedIdentifierGenerator) NextID(ctx context.Context, entity EntityMeta, column ColumnMeta) (any, error) {
	return g.id, nil
}

func (g fixedIdentifierGenerator) NextUUID(ctx context.Context, entity EntityMeta, column ColumnMeta) (string, error) {
	return g.uuid, nil
}
