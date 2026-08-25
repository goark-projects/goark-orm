package orm

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
)

type testUserStatus string

func (s testUserStatus) EnumValue() any {
	return string(s)
}

func TestCompileSQL_whenEnumValuerProvided_shouldBindEnumDatabaseValue(t *testing.T) {
	t.Parallel()

	compiled, err := CompileSQL(
		"select id from sys_user where status = #{status}",
		NamedArgs{"status": testUserStatus("ACTIVE")},
		NewQuestionDialect(),
	)
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	if compiled.Args[0] != "ACTIVE" {
		t.Fatalf("unexpected enum database value %#v", compiled.Args[0])
	}
}

type contextEnumValue struct{}

func (contextEnumValue) EnumValueContext(ctx context.Context) (any, error) {
	return ctx.Value(enumContextKey{}), nil
}

type enumContextKey struct{}

func TestDatabaseEnumValue_whenContextEnumProvided_shouldUseContextValue(t *testing.T) {
	t.Parallel()

	value, err := databaseEnumValue(context.WithValue(context.Background(), enumContextKey{}, "CTX"), contextEnumValue{})
	if err != nil {
		t.Fatalf("enum value failed: %v", err)
	}
	if value != "CTX" {
		t.Fatalf("unexpected enum value %#v", value)
	}
}

func TestSQLSession_Exec_whenContextEnumProvided_shouldUseCallerContext(t *testing.T) {
	t.Parallel()

	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UpdateStatus",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UpdateStatus",
		Command:   StatementCommandUpdate,
		SQL:       "update sys_user set status = #{status} where id = #{id}",
	})
	state := openTestSQLState(t)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	ctx := context.WithValue(context.Background(), enumContextKey{}, "CTX")
	_, err = session.Exec(ctx, "system.user.UserMapper.UpdateStatus", NamedArgs{
		"id":     int64(7),
		"status": contextEnumValue{},
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "CTX"}, {Ordinal: 2, Value: int64(7)}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
}
