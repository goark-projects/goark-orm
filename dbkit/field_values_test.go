package dbkit_test

import (
	"context"
	"reflect"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/dbkit"
)

func TestFieldValueHelpers_whenKitProvided_shouldReturnTypedScalars(t *testing.T) {
	session := &testStatementSession{
		dialect:        orm.NewPostgresDialect(),
		queryResult:    []string{"Alice", "Bob"},
		queryOneResult: "ACTIVE",
	}
	kit, err := dbkit.NewWithEntity[dbkitUser, int64](session, dbkitUserEntity())
	if err != nil {
		t.Fatalf("new kit failed: %v", err)
	}

	names, err := dbkit.ListFieldValues(
		context.Background(),
		kit,
		orm.NewTypedField[dbkitUser, string]("name"),
		orm.NewQueryWrapper[dbkitUser]().OrderByAsc(dbkitUserName),
	)
	if err != nil {
		t.Fatalf("list field values failed: %v", err)
	}
	status, err := dbkit.GetFieldValue(
		context.Background(),
		kit,
		orm.NewTypedField[dbkitUser, string]("status"),
		orm.NewQueryWrapper[dbkitUser]().Eq(dbkitUserID, int64(7)),
	)
	if err != nil {
		t.Fatalf("get field value failed: %v", err)
	}

	if !reflect.DeepEqual(names, []string{"Alice", "Bob"}) || status != "ACTIVE" {
		t.Fatalf("unexpected typed values names=%#v status=%q", names, status)
	}
	expectedQueries := []string{`SELECT "name" FROM "sys_user" ORDER BY "name" ASC`}
	if !reflect.DeepEqual(statementSQL(session.queries), expectedQueries) {
		t.Fatalf("unexpected query SQL %#v", statementSQL(session.queries))
	}
	expectedQueryOnes := []string{`SELECT "status" FROM "sys_user" WHERE "id" = #{__goark_orm_w_0}`}
	if !reflect.DeepEqual(statementSQL(session.queryOnes), expectedQueryOnes) {
		t.Fatalf("unexpected query-one SQL %#v", statementSQL(session.queryOnes))
	}
}

func TestKit_ListIDs_whenCalled_shouldReturnPrimaryKeys(t *testing.T) {
	session := &testStatementSession{
		dialect:     orm.NewPostgresDialect(),
		queryResult: []int64{7, 8},
	}
	kit, err := dbkit.NewWithEntity[dbkitUser, int64](session, dbkitUserEntity())
	if err != nil {
		t.Fatalf("new kit failed: %v", err)
	}

	ids, err := kit.ListIDs(context.Background(), orm.NewQueryWrapper[dbkitUser]().Eq(dbkitUserStatus, "ACTIVE"))
	if err != nil {
		t.Fatalf("list ids failed: %v", err)
	}

	if !reflect.DeepEqual(ids, []int64{7, 8}) {
		t.Fatalf("unexpected ids %#v", ids)
	}
	expectedQueries := []string{`SELECT "id" FROM "sys_user" WHERE "status" = #{__goark_orm_w_0}`}
	if !reflect.DeepEqual(statementSQL(session.queries), expectedQueries) {
		t.Fatalf("unexpected query SQL %#v", statementSQL(session.queries))
	}
}
