package runtime

import (
	"context"
	"database/sql/driver"
	"reflect"
	"testing"
	"time"

	"goark.dev/orm/internal/jsoncodec"
)

func TestRegistry_whenCreated_shouldIncludeBuiltinTypeHandlers(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"json", "time", "decimal", "string", "bool", "bytes"} {
		if _, ok := registry.TypeHandler(name); !ok {
			t.Fatalf("expected builtin type-handler %q", name)
		}
	}
}

func TestTypeHandlerAdapter_whenFunctionsProvided_shouldDelegate(t *testing.T) {
	handler := NewTypeHandler(
		func(ctx context.Context, value any) (any, error) {
			_ = ctx
			return "db:" + value.(string), nil
		},
		func(ctx context.Context, value any, target any) error {
			_ = ctx
			*(target.(*string)) = "go:" + value.(string)
			return nil
		},
	)

	databaseValue, err := handler.ToDB(context.Background(), "value")
	if err != nil {
		t.Fatalf("adapter ToDB failed: %v", err)
	}
	if databaseValue != "db:value" {
		t.Fatalf("unexpected database value %#v", databaseValue)
	}
	var out string
	if err := handler.FromDB(context.Background(), "value", &out); err != nil {
		t.Fatalf("adapter FromDB failed: %v", err)
	}
	if out != "go:value" {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestStringTypeHandler_whenBytesProvided_shouldAssignString(t *testing.T) {
	handler := NewStringTypeHandler()
	var out string

	if err := handler.FromDB(context.Background(), []byte("Alice"), &out); err != nil {
		t.Fatalf("string FromDB failed: %v", err)
	}
	if out != "Alice" {
		t.Fatalf("unexpected string %q", out)
	}
}

func TestBoolTypeHandler_whenDatabaseValuesProvided_shouldParse(t *testing.T) {
	handler := NewBoolTypeHandler()
	cases := []struct {
		value any
		want  bool
	}{
		{"Y", true},
		{[]byte("0"), false},
		{int64(2), true},
		{0, false},
	}
	for _, item := range cases {
		var out bool
		if err := handler.FromDB(context.Background(), item.value, &out); err != nil {
			t.Fatalf("bool FromDB for %#v failed: %v", item.value, err)
		}
		if out != item.want {
			t.Fatalf("bool FromDB for %#v = %v", item.value, out)
		}
	}
}

func TestBytesTypeHandler_whenStringProvided_shouldCopyBytes(t *testing.T) {
	handler := NewBytesTypeHandler()
	databaseValue, err := handler.ToDB(context.Background(), "abc")
	if err != nil {
		t.Fatalf("bytes ToDB failed: %v", err)
	}
	bytes, ok := databaseValue.([]byte)
	if !ok || string(bytes) != "abc" {
		t.Fatalf("unexpected bytes database value %#v", databaseValue)
	}
	bytes[0] = 'z'
	var out []byte
	if err := handler.FromDB(context.Background(), []byte("abc"), &out); err != nil {
		t.Fatalf("bytes FromDB failed: %v", err)
	}
	if string(out) != "abc" {
		t.Fatalf("unexpected bytes output %q", out)
	}
}

func TestJSONTypeHandler_whenStructValue_shouldMarshalAndUnmarshal(t *testing.T) {
	handler := NewJSONTypeHandler()
	value := map[string]string{"role": "admin"}

	databaseValue, err := handler.ToDB(context.Background(), value)
	if err != nil {
		t.Fatalf("json ToDB failed: %v", err)
	}
	data, ok := databaseValue.([]byte)
	if !ok || !jsoncodec.Valid(data) {
		t.Fatalf("expected valid json bytes, got %#v", databaseValue)
	}

	var out map[string]string
	if err := handler.FromDB(context.Background(), data, &out); err != nil {
		t.Fatalf("json FromDB failed: %v", err)
	}
	if !reflect.DeepEqual(out, value) {
		t.Fatalf("unexpected json value %#v", out)
	}
}

func TestTimeTypeHandler_whenStringValue_shouldParseTime(t *testing.T) {
	handler := NewTimeTypeHandler()
	var out time.Time

	if err := handler.FromDB(context.Background(), "2026-08-21T10:11:12Z", &out); err != nil {
		t.Fatalf("time FromDB failed: %v", err)
	}
	if out.UTC().Format(time.RFC3339) != "2026-08-21T10:11:12Z" {
		t.Fatalf("unexpected time %s", out.UTC().Format(time.RFC3339))
	}
}

func TestDecimalTypeHandler_whenStringValue_shouldAssignNumericTarget(t *testing.T) {
	handler := NewDecimalTypeHandler()
	var out float64

	if err := handler.FromDB(context.Background(), "12.5", &out); err != nil {
		t.Fatalf("decimal FromDB failed: %v", err)
	}
	if out != 12.5 {
		t.Fatalf("unexpected decimal %v", out)
	}
}

func TestSQLSession_whenRegistryTypeHandlerRegistered_shouldBindWithoutSessionOption(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "InsertProfile",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.InsertProfile",
		Command:       StatementCommandInsert,
		Source:        StatementSourceAnnotation,
		SQL:           "insert into sys_user(profile) values(#{Profile})",
		ParameterType: "sqlSessionUser",
	})
	if err := registry.RegisterTypeHandler("profile", constantProfileTypeHandler{value: "registry"}); err != nil {
		t.Fatalf("register type-handler failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.InsertProfile", NamedArgs{
		"Profile": sqlSessionProfile{Text: "admin"},
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: "registry"}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestSQLSession_whenSessionTypeHandlerRegistered_shouldOverrideRegistryHandler(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "InsertProfile",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.InsertProfile",
		Command:       StatementCommandInsert,
		Source:        StatementSourceAnnotation,
		SQL:           "insert into sys_user(profile) values(#{Profile})",
		ParameterType: "sqlSessionUser",
	})
	if err := registry.RegisterTypeHandler("profile", constantProfileTypeHandler{value: "registry"}); err != nil {
		t.Fatalf("register type-handler failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil, WithTypeHandler("profile", constantProfileTypeHandler{value: "session"}))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.InsertProfile", NamedArgs{
		"Profile": sqlSessionProfile{Text: "admin"},
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: "session"}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestRegistry_RegisterTypeHandlers_shouldRegisterAtomically(t *testing.T) {
	registry := NewRegistry()
	err := registry.RegisterTypeHandlers(map[string]TypeHandler{
		"profile": constantProfileTypeHandler{value: "profile"},
		"broken":  nil,
	})
	if err == nil {
		t.Fatalf("expected nil handler error")
	}
	if _, ok := registry.TypeHandler("profile"); ok {
		t.Fatalf("batch registration should not partially mutate registry")
	}

	if err := registry.RegisterTypeHandlers(map[string]TypeHandler{" profile ": constantProfileTypeHandler{value: "profile"}}); err != nil {
		t.Fatalf("register type handlers failed: %v", err)
	}
	if _, ok := registry.TypeHandler("profile"); !ok {
		t.Fatalf("expected normalized type-handler")
	}
}

func TestSQLSession_WithTypeHandlers_shouldOverrideRegistryHandlers(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "InsertProfile",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.InsertProfile",
		Command:       StatementCommandInsert,
		Source:        StatementSourceAnnotation,
		SQL:           "insert into sys_user(profile) values(#{Profile})",
		ParameterType: "sqlSessionUser",
	})
	if err := registry.RegisterTypeHandler("profile", constantProfileTypeHandler{value: "registry"}); err != nil {
		t.Fatalf("register type-handler failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil, WithTypeHandlers(map[string]TypeHandler{
		"profile": constantProfileTypeHandler{value: "session"},
	}))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.InsertProfile", NamedArgs{
		"Profile": sqlSessionProfile{Text: "admin"},
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: "session"}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

type constantProfileTypeHandler struct {
	value string
}

func (h constantProfileTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	_ = value
	return h.value, nil
}

func (h constantProfileTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	_ = value
	_ = target
	return nil
}
