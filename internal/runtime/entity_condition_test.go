package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestBaseMapper_SelectListByEntity_whenDefaultStrategy_shouldSkipZeroValues(t *testing.T) {
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

	users, err := mapper.SelectListByEntity(context.Background(), &baseMapperUser{Name: "Alice"})
	if err != nil {
		t.Fatalf("select by entity failed: %v", err)
	}

	if len(users) != 1 || users[0].ID != 7 || users[0].Name != "Alice" {
		t.Fatalf("unexpected users %#v", users)
	}
	expectedSQL := `SELECT "id", "name", "status" FROM "sys_user" WHERE "name" = $1`
	if state.query != expectedSQL {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "Alice"}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestService_ListByEntity_whenConditionAndWhereStrategyProvided_shouldApplyMetadata(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name", "status"},
		values:  [][]driver.Value{{int64(7), "Alice", ""}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, entityConditionUserMeta())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	service, err := NewService[baseMapperUser, int64](mapper)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	users, err := service.ListByEntity(context.Background(), &baseMapperUser{Name: "%Al%", Status: ""})
	if err != nil {
		t.Fatalf("service list by entity failed: %v", err)
	}

	if len(users) != 1 || users[0].ID != 7 {
		t.Fatalf("unexpected users %#v", users)
	}
	expectedSQL := `SELECT "id", "name", "status" FROM "sys_user" WHERE "name" LIKE $1 AND "status" = $2`
	if state.query != expectedSQL {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "%Al%"},
		{Ordinal: 2, Value: ""},
	}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func entityConditionUserMeta() EntityMeta {
	return EntityMeta{
		TypeName: "baseMapperUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", FieldType: "int64", ColumnName: "id", PrimaryKey: true, AutoIncrement: true},
			{FieldName: "Name", FieldType: "string", ColumnName: "name", Condition: "%s LIKE #{%s}", WhereStrategy: FieldStrategyNotEmpty},
			{FieldName: "Status", FieldType: "string", ColumnName: "status", WhereStrategy: FieldStrategyAlways},
		},
	}
}
