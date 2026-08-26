package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestUpdateWrapper_whenSetAndConditionsProvided_shouldRenderSQL(t *testing.T) {
	wrapper := NewUpdateWrapper[baseMapperUser]().
		Set(baseMapperUserName, "Alice").
		Eq(baseMapperUserID, int64(7))

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build update wrapper failed: %v", err)
	}

	if rendered.SetSQL != `"name" = #{__goark_orm_u_0}` {
		t.Fatalf("unexpected set SQL %q", rendered.SetSQL)
	}
	if rendered.WhereSQL != `"id" = #{__goark_orm_w_1}` {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	expectedArgs := NamedArgs{"__goark_orm_u_0": "Alice", "__goark_orm_w_1": int64(7)}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestUpdateWrapper_whenTypedFieldsUsed_shouldKeepValueTypesAtCompileTime(t *testing.T) {
	name := NewTypedField[baseMapperUser, string]("name")
	id := NewTypedField[baseMapperUser, int64]("id")

	wrapper := NewUpdateWrapper[baseMapperUser]().
		SetTyped(name, "Alice").
		EqTyped(id, int64(7))

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build update wrapper failed: %v", err)
	}
	if rendered.SetSQL == "" || rendered.WhereSQL == "" {
		t.Fatalf("expected typed fields to render update wrapper, got %#v", rendered)
	}

	typed := SetTypedValue(NewUpdateWrapper[baseMapperUser](), name, "Bob")
	typed = EqTypedValue(typed, id, int64(8))
	if typed.SetEmpty() || typed.Empty() {
		t.Fatalf("expected typed helper functions to keep set and where clauses")
	}
}

func TestUpdateWrapper_whenTypedValueHelpersUsed_shouldRenderSQL(t *testing.T) {
	id := NewTypedField[baseMapperUser, int64]("id")
	name := NewTypedField[baseMapperUser, string]("name")
	status := NewTypedField[baseMapperUser, string]("status")

	wrapper := SetTypedValue(NewUpdateWrapper[baseMapperUser](), name, "Alice")
	wrapper = SetTypedValueIf(false, wrapper, name, "Ignored")
	wrapper = SetIncrByTypedValue(wrapper, id, int64(1))
	wrapper = SetDecrByTypedValueIf(false, wrapper, id, int64(2))
	wrapper = NeTypedValue(wrapper, status, "DELETED")
	wrapper = NotInTypedValues(wrapper, id, int64(100), int64(200))
	wrapper = LeTypedValueIf(true, wrapper, id, int64(999))

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build update wrapper failed: %v", err)
	}

	expectedSet := `"name" = #{__goark_orm_u_0}, "id" = "id" + #{__goark_orm_u_1}`
	if rendered.SetSQL != expectedSet {
		t.Fatalf("unexpected set SQL %q", rendered.SetSQL)
	}
	expectedWhere := `"status" <> #{__goark_orm_w_2} AND "id" NOT IN (#{__goark_orm_w_3}, #{__goark_orm_w_4}) AND "id" <= #{__goark_orm_w_5}`
	if rendered.WhereSQL != expectedWhere {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_u_0": "Alice",
		"__goark_orm_u_1": int64(1),
		"__goark_orm_w_2": "DELETED",
		"__goark_orm_w_3": int64(100),
		"__goark_orm_w_4": int64(200),
		"__goark_orm_w_5": int64(999),
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestUpdateWrapper_whenRichConditionsProvided_shouldRenderOperators(t *testing.T) {
	wrapper := NewUpdateWrapper[baseMapperUser]().
		SetIf(false, baseMapperUserName, "ignored").
		Set(baseMapperUserStatus, "LOCKED").
		Ne(baseMapperUserName, "Root").
		In(baseMapperUserID, []int64{7, 8}).
		IsNotNull(baseMapperUserStatus)

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build update wrapper failed: %v", err)
	}

	if rendered.SetSQL != `"status" = #{__goark_orm_u_0}` {
		t.Fatalf("unexpected set SQL %q", rendered.SetSQL)
	}
	expectedWhere := `"name" <> #{__goark_orm_w_1} AND "id" IN (#{__goark_orm_w_2}, #{__goark_orm_w_3}) AND "status" IS NOT NULL`
	if rendered.WhereSQL != expectedWhere {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_u_0": "LOCKED",
		"__goark_orm_w_1": "Root",
		"__goark_orm_w_2": int64(7),
		"__goark_orm_w_3": int64(8),
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestUpdateWrapper_whenNestedExistsAndLastProvided_shouldRenderSQL(t *testing.T) {
	wrapper := NewUpdateWrapper[baseMapperUser]().
		Set(baseMapperUserStatus, "LOCKED").
		And(func(child *UpdateWrapper[baseMapperUser]) {
			child.Eq(baseMapperUserName, "Alice").Or(func(or *UpdateWrapper[baseMapperUser]) {
				or.Eq(baseMapperUserID, int64(7))
			})
		}).
		NotExists(`select 1 from sys_role r where r.user_id = id and r.code = #{role}`, NamedArgs{"role": "admin"}).
		Last("RETURNING id")

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build update wrapper failed: %v", err)
	}

	if rendered.SetSQL != `"status" = #{__goark_orm_u_0}` {
		t.Fatalf("unexpected set SQL %q", rendered.SetSQL)
	}
	expectedWhere := `("name" = #{__goark_orm_w_1} OR ("id" = #{__goark_orm_w_2})) AND NOT EXISTS (select 1 from sys_role r where r.user_id = id and r.code = #{__goark_orm_w_3})`
	if rendered.WhereSQL != expectedWhere {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	if rendered.LastSQL != "RETURNING id" {
		t.Fatalf("unexpected last SQL %q", rendered.LastSQL)
	}
}

func TestUpdateWrapper_whenRawSetAndArithmeticProvided_shouldRenderSQL(t *testing.T) {
	wrapper := NewUpdateWrapper[baseMapperUser]().
		SetSQL(`status = coalesce(#{status}, status)`, NamedArgs{"status": "LOCKED"}).
		SetIncrBy(baseMapperUserID, int64(2)).
		SetDecrBy(baseMapperUserID, int64(1)).
		NotLike(baseMapperUserName, "Root%").
		Between(baseMapperUserID, int64(1), int64(9)).
		NotIn(baseMapperUserStatus, []string{"DELETED"})

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build update wrapper failed: %v", err)
	}

	expectedSet := `status = coalesce(#{__goark_orm_w_0}, status), "id" = "id" + #{__goark_orm_u_1}, "id" = "id" - #{__goark_orm_u_2}`
	if rendered.SetSQL != expectedSet {
		t.Fatalf("unexpected set SQL %q", rendered.SetSQL)
	}
	expectedWhere := `"name" NOT LIKE #{__goark_orm_w_3} AND "id" BETWEEN #{__goark_orm_w_4} AND #{__goark_orm_w_5} AND "status" NOT IN (#{__goark_orm_w_6})`
	if rendered.WhereSQL != expectedWhere {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_w_0": "LOCKED",
		"__goark_orm_u_1": int64(2),
		"__goark_orm_u_2": int64(1),
		"__goark_orm_w_3": "Root%",
		"__goark_orm_w_4": int64(1),
		"__goark_orm_w_5": int64(9),
		"__goark_orm_w_6": "DELETED",
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestBaseMapper_UpdateWithWrapper_whenWrapperProvided_shouldExecutePartialUpdate(t *testing.T) {
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

	rows, err := mapper.UpdateWithWrapper(
		context.Background(),
		NewUpdateWrapper[baseMapperUser]().
			Set(baseMapperUserName, "Alice").
			Eq(baseMapperUserID, int64(7)),
	)
	if err != nil {
		t.Fatalf("update with wrapper failed: %v", err)
	}

	if rows != 1 {
		t.Fatalf("unexpected rows affected %d", rows)
	}
	if state.exec != `UPDATE "sys_user" SET "name" = $1 WHERE "id" = $2` {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "Alice"}, {Ordinal: 2, Value: int64(7)}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}

func TestBaseMapper_UpdateWithWrapper_whenWrapperEmpty_shouldRejectFullTableUpdate(t *testing.T) {
	state := openTestSQLState(t)
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = mapper.UpdateWithWrapper(context.Background(), NewUpdateWrapper[baseMapperUser]().Set(baseMapperUserName, "Alice"))
	if err == nil {
		t.Fatalf("expected empty condition update wrapper to fail")
	}
	if state.exec != "" {
		t.Fatalf("update should not execute SQL, got %q", state.exec)
	}
}
