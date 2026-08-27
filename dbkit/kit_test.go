package dbkit_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/dbkit"
)

type dbkitUser struct {
	ID     int64
	Name   string
	Status string
}

var (
	dbkitUserID     = orm.NewField[dbkitUser]("id")
	dbkitUserName   = orm.NewField[dbkitUser]("name")
	dbkitUserStatus = orm.NewField[dbkitUser]("status")
)

func TestKit_whenNewWithEntity_shouldExposeConvenienceCRUD(t *testing.T) {
	session := &testStatementSession{
		dialect:     orm.NewPostgresDialect(),
		queryResult: []dbkitUser{{ID: 7, Name: "Alice", Status: "ACTIVE"}},
		execResult:  orm.Result{RowsAffected: 1},
	}
	kit, err := dbkit.NewWithEntity[dbkitUser, int64](session, dbkitUserEntity())
	if err != nil {
		t.Fatalf("new kit failed: %v", err)
	}

	records, err := kit.List(context.Background(), orm.NewQueryWrapper[dbkitUser]().Eq(dbkitUserStatus, "ACTIVE"))
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	saved, err := kit.Save(context.Background(), &dbkitUser{Name: "Bob", Status: "ACTIVE"})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	removed, err := kit.RemoveByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("remove by id failed: %v", err)
	}

	if len(records) != 1 || records[0].Name != "Alice" {
		t.Fatalf("unexpected records %#v", records)
	}
	if !saved || !removed {
		t.Fatalf("expected save/remove success, saved=%v removed=%v", saved, removed)
	}
	if session.queries[0].SQL != `SELECT "id", "name", "status" FROM "sys_user" WHERE "status" = #{__goark_orm_w_0}` {
		t.Fatalf("unexpected list SQL %q", session.queries[0].SQL)
	}
	expectedExecs := []string{
		`INSERT INTO "sys_user" ("name", "status") VALUES (#{Name}, #{Status})`,
		`DELETE FROM "sys_user" WHERE "id" = #{id}`,
	}
	if !reflect.DeepEqual(statementSQL(session.execs), expectedExecs) {
		t.Fatalf("unexpected exec SQL %#v", statementSQL(session.execs))
	}
}

func TestKit_whenNewFromRegistry_shouldResolveEntityByGoTypeName(t *testing.T) {
	registry := orm.NewRegistry()
	if err := registry.RegisterEntity(dbkitUserEntity()); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	session := &testStatementSession{
		dialect:        orm.NewPostgresDialect(),
		queryOneResult: dbkitUser{ID: 7, Name: "Alice", Status: "ACTIVE"},
	}
	kit, err := dbkit.NewFromRegistry[dbkitUser, int64](session, registry)
	if err != nil {
		t.Fatalf("new kit from registry failed: %v", err)
	}

	user, err := kit.GetByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}

	if user == nil || user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
	if session.queryOnes[0].SQL != `SELECT "id", "name", "status" FROM "sys_user" WHERE "id" = #{id}` {
		t.Fatalf("unexpected query SQL %q", session.queryOnes[0].SQL)
	}
}

func TestKit_whenNewFromRegistryUsesPointerType_shouldReject(t *testing.T) {
	registry := orm.NewRegistry()
	if err := registry.RegisterEntity(dbkitUserEntity()); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}

	_, err := dbkit.NewFromRegistry[*dbkitUser, int64](&testStatementSession{}, registry)

	if err == nil || !strings.Contains(err.Error(), "entity type must be a struct") {
		t.Fatalf("expected pointer entity type to fail, got %v", err)
	}
}

func TestSimpleQueryHelpers_whenRecordsReturned_shouldProjectMapAndGroup(t *testing.T) {
	session := &testStatementSession{
		dialect: orm.NewPostgresDialect(),
		queryResult: []dbkitUser{
			{ID: 7, Name: "Alice", Status: "ACTIVE"},
			{ID: 8, Name: "Bob", Status: "ACTIVE"},
			{ID: 9, Name: "Carol", Status: "LOCKED"},
		},
	}
	mapper, err := orm.NewBaseMapper[dbkitUser, int64](session, dbkitUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	names, err := dbkit.List(context.Background(), mapper, orm.NewQueryWrapper[dbkitUser]().OrderByAsc(dbkitUserID), func(user dbkitUser) string {
		return user.Name
	})
	if err != nil {
		t.Fatalf("list projection failed: %v", err)
	}
	keyed, err := dbkit.KeyMap(context.Background(), mapper, nil, func(user dbkitUser) int64 {
		return user.ID
	})
	if err != nil {
		t.Fatalf("key map failed: %v", err)
	}
	grouped, err := dbkit.GroupValues(context.Background(), mapper, nil, func(user dbkitUser) string {
		return user.Status
	}, func(user dbkitUser) string {
		return user.Name
	})
	if err != nil {
		t.Fatalf("group values failed: %v", err)
	}

	if !reflect.DeepEqual(names, []string{"Alice", "Bob", "Carol"}) {
		t.Fatalf("unexpected names %#v", names)
	}
	if keyed[8].Name != "Bob" {
		t.Fatalf("unexpected keyed map %#v", keyed)
	}
	if !reflect.DeepEqual(grouped["ACTIVE"], []string{"Alice", "Bob"}) || !reflect.DeepEqual(grouped["LOCKED"], []string{"Carol"}) {
		t.Fatalf("unexpected grouped values %#v", grouped)
	}
	if len(session.queries) != 3 {
		t.Fatalf("expected three helper queries, got %d", len(session.queries))
	}
}

func TestSimpleQueryHelpers_whenRequiredInputsMissing_shouldReject(t *testing.T) {
	_, err := dbkit.List[dbkitUser, int64, string](context.Background(), nil, nil, func(user dbkitUser) string {
		return user.Name
	})
	if err == nil || !strings.Contains(err.Error(), "mapper is nil") {
		t.Fatalf("expected nil mapper to fail, got %v", err)
	}

	_, err = dbkit.ListFrom[dbkitUser, string]([]dbkitUser{{ID: 7}}, nil)
	if err == nil || !strings.Contains(err.Error(), "projector is nil") {
		t.Fatalf("expected nil projector to fail, got %v", err)
	}
}

func dbkitUserEntity() orm.EntityMeta {
	return orm.EntityMeta{
		TypeName: "dbkitUser",
		Table:    "sys_user",
		Columns: []orm.ColumnMeta{
			{FieldName: "ID", FieldType: "int64", ColumnName: "id", PrimaryKey: true, AutoIncrement: true},
			{FieldName: "Name", FieldType: "string", ColumnName: "name"},
			{FieldName: "Status", FieldType: "string", ColumnName: "status"},
		},
	}
}

type testStatementSession struct {
	dialect        orm.Dialect
	queryResult    any
	queryOneResult any
	execResult     orm.Result
	queries        []orm.StatementMeta
	queryOnes      []orm.StatementMeta
	execs          []orm.StatementMeta
}

func (s *testStatementSession) Dialect() orm.Dialect {
	if s.dialect == nil {
		return orm.NewPostgresDialect()
	}
	return s.dialect
}

func (s *testStatementSession) QueryStatement(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs, dest any) error {
	s.queries = append(s.queries, statement)
	return assignDest(dest, s.queryResult)
}

func (s *testStatementSession) QueryOneStatement(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs, dest any) error {
	s.queryOnes = append(s.queryOnes, statement)
	return assignDest(dest, s.queryOneResult)
}

func (s *testStatementSession) ExecStatement(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs) (orm.Result, error) {
	s.execs = append(s.execs, statement)
	return s.execResult, nil
}

func assignDest(dest any, value any) error {
	if value == nil {
		return nil
	}
	target := reflect.ValueOf(dest)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		return fmt.Errorf("dest must be non-nil pointer")
	}
	source := reflect.ValueOf(value)
	if !source.Type().AssignableTo(target.Elem().Type()) {
		return fmt.Errorf("value %s is not assignable to %s", source.Type(), target.Elem().Type())
	}
	target.Elem().Set(source)
	return nil
}

func statementSQL(statements []orm.StatementMeta) []string {
	out := make([]string, 0, len(statements))
	for _, statement := range statements {
		out = append(out, statement.SQL)
	}
	return out
}
