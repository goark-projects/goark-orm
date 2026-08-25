package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
	"time"
)

type entitySemanticUser struct {
	ID        int64
	Name      string
	Version   int64
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

func TestEntitySemanticInterceptor_whenInsertEntity_shouldAutoFillTimeFields(t *testing.T) {
	fixed := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newEntitySemanticRegistry(t, StatementMeta{
		ID:            "Insert",
		Namespace:     "system.semantic.UserMapper",
		FullName:      "system.semantic.UserMapper.Insert",
		Command:       StatementCommandInsert,
		Source:        StatementSourceAnnotation,
		SQL:           "insert into sys_user(name, created_at, updated_at) values(#{Name}, #{CreatedAt}, #{UpdatedAt})",
		ParameterType: "entitySemanticUser",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewEntitySemanticInterceptor(registry, WithEntitySemanticClock(func() time.Time {
		return fixed
	}))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	user := &entitySemanticUser{Name: "Alice"}

	_, err = session.Exec(context.Background(), "system.semantic.UserMapper.Insert", NamedArgs{
		"user": user,
		"Name": user.Name,
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if !user.CreatedAt.Equal(fixed) || !user.UpdatedAt.Equal(fixed) {
		t.Fatalf("expected auto-filled time, got %#v", user)
	}
	if state.exec != "insert into sys_user(name, created_at, updated_at) values($1, $2, $3)" {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: fixed},
		{Ordinal: 3, Value: fixed},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}

func TestEntitySemanticInterceptor_whenSelectEntityHasSoftDelete_shouldAppendLiveCondition(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newEntitySemanticRegistry(t, StatementMeta{
		ID:         "FindByID",
		Namespace:  "system.semantic.UserMapper",
		FullName:   "system.semantic.UserMapper.FindByID",
		Command:    StatementCommandSelect,
		Source:     StatementSourceAnnotation,
		SQL:        "select id, name from sys_user where id = #{ID}",
		ResultType: "entitySemanticUser",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewEntitySemanticInterceptor(registry)))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user entitySemanticUser
	if err := session.QueryOne(context.Background(), "system.semantic.UserMapper.FindByID", NamedArgs{"ID": int64(7)}, &user); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != `select id, name from sys_user where id = $1 AND "deleted" = $2` {
		t.Fatalf("unexpected SQL %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: int64(7)}, {Ordinal: 2, Value: false}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.queryArgs)
	}
	if user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
}

func TestEntitySemanticInterceptor_whenUpdateEntityHasVersion_shouldInjectVersionAndLiveCondition(t *testing.T) {
	fixed := time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC)
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newEntitySemanticRegistry(t, StatementMeta{
		ID:            "UpdateByID",
		Namespace:     "system.semantic.UserMapper",
		FullName:      "system.semantic.UserMapper.UpdateByID",
		Command:       StatementCommandUpdate,
		Source:        StatementSourceAnnotation,
		SQL:           "update sys_user set name = #{Name}, updated_at = #{UpdatedAt} where id = #{ID}",
		ParameterType: "entitySemanticUser",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewEntitySemanticInterceptor(registry, WithEntitySemanticClock(func() time.Time {
		return fixed
	}))))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	user := &entitySemanticUser{ID: 7, Name: "Alice", Version: 3}

	_, err = session.Exec(context.Background(), "system.semantic.UserMapper.UpdateByID", NamedArgs{
		"user":    user,
		"ID":      user.ID,
		"Name":    user.Name,
		"Version": user.Version,
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	expectedSQL := `update sys_user set name = $1, updated_at = $2, "version" = "version" + 1 where id = $3 AND "version" = $4 AND "deleted" = $5`
	if state.exec != expectedSQL {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: fixed},
		{Ordinal: 3, Value: int64(7)},
		{Ordinal: 4, Value: int64(3)},
		{Ordinal: 5, Value: false},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
	if !user.UpdatedAt.Equal(fixed) {
		t.Fatalf("expected updated time to be filled, got %#v", user)
	}
}

func TestSQLSession_Exec_whenMetaObjectHandlerConfigured_shouldFillMapperEntityArgs(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newEntitySemanticRegistry(t, StatementMeta{
		ID:            "InsertAudit",
		Namespace:     "system.semantic.UserMapper",
		FullName:      "system.semantic.UserMapper.InsertAudit",
		Command:       StatementCommandInsert,
		Source:        StatementSourceAnnotation,
		SQL:           "insert into sys_user(name, created_by, updated_by) values(#{Name}, #{createdBy}, #{updatedBy})",
		ParameterType: "entitySemanticUser",
	})
	config := DefaultConfiguration()
	config.MetaObjectHandler = auditMetaObjectHandler{user: "system"}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	user := &entitySemanticUser{Name: "Alice"}

	_, err = session.Exec(context.Background(), "system.semantic.UserMapper.InsertAudit", NamedArgs{
		"user":      user,
		"Name":      user.Name,
		"createdBy": user.CreatedBy,
		"updatedBy": user.UpdatedBy,
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if user.CreatedBy != "system" || user.UpdatedBy != "system" {
		t.Fatalf("expected meta object fields to be filled, got %#v", user)
	}
	if state.exec != "insert into sys_user(name, created_by, updated_by) values($1, $2, $3)" {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: "system"},
		{Ordinal: 3, Value: "system"},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}

func TestEntitySemanticInterceptor_whenDeleteEntityHasSoftDelete_shouldRewriteToUpdate(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newEntitySemanticRegistry(t, StatementMeta{
		ID:            "DeleteByID",
		Namespace:     "system.semantic.UserMapper",
		FullName:      "system.semantic.UserMapper.DeleteByID",
		Command:       StatementCommandDelete,
		Source:        StatementSourceAnnotation,
		SQL:           "delete from sys_user where id = #{ID}",
		ParameterType: "entitySemanticUser",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewEntitySemanticInterceptor(registry)))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.semantic.UserMapper.DeleteByID", NamedArgs{"ID": int64(7)})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	expectedSQL := `UPDATE "sys_user" SET "deleted" = $1 WHERE id = $2 AND "deleted" = $3`
	if state.exec != expectedSQL {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: true},
		{Ordinal: 2, Value: int64(7)},
		{Ordinal: 3, Value: false},
	}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}

func newEntitySemanticRegistry(t *testing.T, statements ...StatementMeta) *Registry {
	t.Helper()
	registry := NewRegistry()
	if err := registry.RegisterEntity(EntityMeta{
		TypeName: "entitySemanticUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", ColumnName: "id", PrimaryKey: true},
			{FieldName: "Name", ColumnName: "name"},
			{FieldName: "Version", ColumnName: "version", Version: true},
			{FieldName: "Deleted", ColumnName: "deleted", SoftDelete: true},
			{FieldName: "CreatedAt", ColumnName: "created_at", CreatedAt: true},
			{FieldName: "UpdatedAt", ColumnName: "updated_at", UpdatedAt: true},
			{FieldName: "CreatedBy", ColumnName: "created_by", Fill: FieldFillInsert},
			{FieldName: "UpdatedBy", ColumnName: "updated_by", Fill: FieldFillInsertUpdate},
		},
	}); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:   "UserMapper",
		Namespace:  "system.semantic.UserMapper",
		Statements: statements,
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	return registry
}
