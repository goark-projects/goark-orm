package orm

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

func TestStructuredErrors_whenStatementMissing_shouldExposeStatementContext(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user",
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.Missing", nil, &users)
	if !errors.Is(err, ErrStatementNotFound) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected statement-not-found classification, got %v", err)
	}
	var typed *StatementNotFoundError
	if !errors.As(err, &typed) || typed.Statement != "system.user.UserMapper.Missing" {
		t.Fatalf("expected statement context, got %#v", typed)
	}
}

func TestStructuredErrors_whenSQLParameterMissing_shouldExposeBindingContext(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", nil, &user)
	if !errors.Is(err, ErrBinding) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected binding classification, got %v", err)
	}
	var typed *BindingError
	if !errors.As(err, &typed) {
		t.Fatalf("expected BindingError, got %T", err)
	}
	if typed.Statement != "system.user.UserMapper.FindByID" || typed.Parameter != "id" || typed.Operation != "compile" {
		t.Fatalf("unexpected binding context %#v", typed)
	}
}

func TestStructuredErrors_whenQueryOneReturnsMultipleRows_shouldExposeTooManyResults(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values: [][]driver.Value{
			{int64(7), "Alice"},
			{int64(8), "Bob"},
		},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindAny",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindAny",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user",
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindAny", nil, &user)
	if !errors.Is(err, ErrTooManyResults) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected too-many-results classification, got %v", err)
	}
	var typed *TooManyResultsError
	if !errors.As(err, &typed) || typed.Statement != "system.user.UserMapper.FindAny" {
		t.Fatalf("expected too-many-results context, got %#v", typed)
	}
}

func TestStructuredErrors_whenResultMapTypeHandlerMissing_shouldExposeMappingContext(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"profile"},
		values:  [][]driver.Value{{[]byte(`{"text":"Alice"}`)}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindProfile",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindProfile",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select profile from sys_user",
		ResultMap: "UserResult",
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindProfile", nil, &user)
	if !errors.Is(err, ErrMapping) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected mapping classification, got %v", err)
	}
	var typed *MappingError
	if !errors.As(err, &typed) {
		t.Fatalf("expected MappingError, got %T", err)
	}
	if typed.Statement != "system.user.UserMapper.FindProfile" || typed.Column != "profile" || typed.Field != "Profile" {
		t.Fatalf("unexpected mapping context %#v", typed)
	}
}

func TestStructuredErrors_whenExecFails_shouldExposeExecutorContext(t *testing.T) {
	state := openTestSQLState(t)
	driverErr := errors.New("driver exec failed")
	state.execErrors = []error{driverErr}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "UpdateName",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.UpdateName",
		Command:   StatementCommandUpdate,
		Source:    StatementSourceAnnotation,
		SQL:       "update sys_user set name = #{name} where id = #{id}",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"})
	if !errors.Is(err, ErrExecutor) || !errors.Is(err, driverErr) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected executor classification, got %v", err)
	}
	var typed *ExecutorError
	if !errors.As(err, &typed) {
		t.Fatalf("expected ExecutorError, got %T", err)
	}
	if typed.Statement != "system.user.UserMapper.UpdateName" || typed.Operation != "exec" || typed.SQL != "update sys_user set name = $1 where id = $2" {
		t.Fatalf("unexpected executor context %#v", typed)
	}
}

func TestStructuredErrors_whenConfigurationInvalid_shouldExposeConfigurationContext(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user",
	})
	config := DefaultConfiguration()
	config.DefaultExecutorType = ExecutorType("invalid")

	_, err := NewSQLSession(registry, state.db, nil, WithConfiguration(config))
	if !errors.Is(err, ErrConfiguration) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected configuration classification, got %v", err)
	}
	var typed *ConfigurationError
	if !errors.As(err, &typed) {
		t.Fatalf("expected ConfigurationError, got %T", err)
	}
}

func TestStructuredErrors_whenRegistryInvalid_shouldExposeRegistryContext(t *testing.T) {
	var registry *Registry
	err := registry.RegisterEntity(EntityMeta{})
	if !errors.Is(err, ErrRegistry) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected registry classification, got %v", err)
	}
	var typed *RegistryError
	if !errors.As(err, &typed) || typed.Resource != "registry" {
		t.Fatalf("expected RegistryError, got %#v", typed)
	}
}
