package orm

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestRegistry_whenCreated_shouldIncludeBuiltinTypeHandlers(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"json", "time", "decimal"} {
		if _, ok := registry.TypeHandler(name); !ok {
			t.Fatalf("expected builtin type-handler %q", name)
		}
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
	if !ok || !json.Valid(data) {
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
