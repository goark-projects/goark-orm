package orm

import (
	"context"
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
