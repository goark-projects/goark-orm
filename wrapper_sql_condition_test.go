package orm

import (
	"reflect"
	"strings"
	"testing"
)

func TestQueryWrapper_whenSQLConditionHelpersProvided_shouldRenderSafeSubqueries(t *testing.T) {
	wrapper := NewQueryWrapper[baseMapperUser]().
		EqSQL(baseMapperUserID, `select max(id) from sys_user where status = #{status}`, NamedArgs{"status": "ACTIVE"}).
		InSQL(baseMapperUserID, `select user_id from sys_role where code = #{code}`, NamedArgs{"code": "admin"}).
		GtSQL(baseMapperUserID, `select min(id) from sys_user where status = #{status}`, NamedArgs{"status": "LOCKED"})

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build wrapper failed: %v", err)
	}

	expectedWhere := `"id" = (select max(id) from sys_user where status = #{__goark_orm_w_0}) AND "id" IN (select user_id from sys_role where code = #{__goark_orm_w_1}) AND "id" > (select min(id) from sys_user where status = #{__goark_orm_w_2})`
	if rendered.WhereSQL != expectedWhere {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_w_0": "ACTIVE",
		"__goark_orm_w_1": "admin",
		"__goark_orm_w_2": "LOCKED",
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestUpdateWrapper_whenSQLConditionHelpersProvided_shouldRenderSafeSubqueryConditions(t *testing.T) {
	wrapper := NewUpdateWrapper[baseMapperUser]().
		Set(baseMapperUserName, "Alice").
		LeSQL(baseMapperUserID, `select max(id) from sys_user where status = #{status}`, NamedArgs{"status": "ACTIVE"})

	rendered, err := wrapper.build(NewPostgresDialect(), 0)
	if err != nil {
		t.Fatalf("build update wrapper failed: %v", err)
	}

	if rendered.SetSQL != `"name" = #{__goark_orm_u_0}` {
		t.Fatalf("unexpected set SQL %q", rendered.SetSQL)
	}
	if rendered.WhereSQL != `"id" <= (select max(id) from sys_user where status = #{__goark_orm_w_1})` {
		t.Fatalf("unexpected where SQL %q", rendered.WhereSQL)
	}
	expectedArgs := NamedArgs{
		"__goark_orm_u_0": "Alice",
		"__goark_orm_w_1": "ACTIVE",
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestQueryWrapper_whenSQLConditionContainsUnsafeFragment_shouldReject(t *testing.T) {
	for _, sqlText := range []string{
		`select ${column} from sys_user`,
		`select id from sys_user; drop table sys_user`,
	} {
		_, err := NewQueryWrapper[baseMapperUser]().
			EqSQL(baseMapperUserID, sqlText, nil).
			build(NewPostgresDialect(), 0)
		if err == nil || (!strings.Contains(err.Error(), "forbidden") && !strings.Contains(err.Error(), "semicolon")) {
			t.Fatalf("expected unsafe SQL fragment %q to fail, got %v", sqlText, err)
		}
	}
}
