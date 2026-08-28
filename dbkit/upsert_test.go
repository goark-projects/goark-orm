package dbkit_test

import (
	"context"
	"reflect"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/dbkit"
)

func TestKit_Upsert_whenRowsAffected_shouldDelegateToService(t *testing.T) {
	session := &testStatementSession{
		dialect:    nil,
		execResult: orm.Result{RowsAffected: 1},
	}
	kit, err := dbkit.NewWithEntity[dbkitUser, int64](session, dbkitUserEntity())
	if err != nil {
		t.Fatalf("new kit failed: %v", err)
	}

	ok, err := kit.Upsert(
		context.Background(),
		&dbkitUser{ID: 7, Name: "Alice", Status: "ACTIVE"},
		[]orm.Field[dbkitUser]{dbkitUserID},
		[]orm.Field[dbkitUser]{dbkitUserName},
	)
	if err != nil {
		t.Fatalf("kit upsert failed: %v", err)
	}

	if !ok {
		t.Fatalf("expected kit upsert to report success")
	}
	expectedSQL := `INSERT INTO "sys_user" ("id", "name", "status") VALUES (#{ID}, #{Name}, #{Status}) ON CONFLICT ("id") DO UPDATE SET "name" = #{Name}`
	if !reflect.DeepEqual(statementSQL(session.execs), []string{expectedSQL}) {
		t.Fatalf("unexpected upsert SQL %#v", statementSQL(session.execs))
	}
}
