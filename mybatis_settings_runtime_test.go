package orm

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSQLSession_whenDefaultResultSetTypeConfigured_shouldApplyStatementOptions(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id from sys_user where id = #{id}",
	})
	executor := &recordingStatementOptionsExecutor{db: state.db}
	config := DefaultConfiguration()
	config.DefaultResultSetType = ResultSetTypeForwardOnly
	session, err := NewSQLSession(registry, executor, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var id int64
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindID", NamedArgs{"id": int64(7)}, &id); err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if len(executor.options) != 1 {
		t.Fatalf("expected one statement options call, got %d", len(executor.options))
	}
	if executor.options[0].ResultSetType != ResultSetTypeForwardOnly {
		t.Fatalf("unexpected statement options %#v", executor.options[0])
	}
}

func TestSQLSession_scanOne_whenAutoMappingUnknownColumnFails_shouldRejectUnmappedColumn(t *testing.T) {
	statement := StatementMeta{
		ID:         "FindOne",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.FindOne",
		Command:    StatementCommandSelect,
		Source:     StatementSourceAnnotation,
		SQL:        "select id, missing_column from sys_user where id = #{id}",
		ResultType: "sqlSessionUser",
	}
	registry := newSQLSessionRegistry(t, statement)
	config := DefaultConfiguration()
	config.AutoMappingUnknownColumnBehavior = AutoMappingUnknownColumnBehaviorFailing
	session, err := NewSQLSession(registry, noopSQLExecutor{}, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	if _, ok := registry.RowScanner("sqlSessionUser"); ok {
		t.Fatalf("test registry should not register generated row scanner")
	}
	bindings := session.columnBindings(statement, reflect.TypeOf(sqlSessionUser{}))
	if !session.shouldFailUnknownAutoMappingColumn(bindings) {
		t.Fatalf("expected auto-mapping bindings to enable unknown column fail-fast: %#v", session.Configuration())
	}

	var user sqlSessionUser
	err = session.scanOne(context.Background(), &memoryRows{
		columns: []string{"id", "missing_column"},
		values:  [][]any{{int64(7), "ignored"}},
	}, statement, &user)
	if err == nil || !strings.Contains(err.Error(), "missing_column") {
		t.Fatalf("expected unknown column mapping error, got %v", err)
	}
}

func TestSQLSession_scanStruct_whenAutoMappingUnknownColumnFails_shouldRejectUnmappedColumn(t *testing.T) {
	session := &SQLSession{configuration: DefaultConfiguration()}
	session.configuration.AutoMappingUnknownColumnBehavior = AutoMappingUnknownColumnBehaviorFailing
	statement := StatementMeta{
		ID:       "FindOne",
		FullName: "system.user.UserMapper.FindOne",
		Command:  StatementCommandSelect,
	}

	var user sqlSessionUser
	err := session.scanStruct(
		context.Background(),
		resultMapValueRow{values: []any{int64(7), "ignored"}},
		[]string{"id", "missing_column"},
		statement,
		reflect.ValueOf(&user).Elem(),
	)
	if err == nil || !strings.Contains(err.Error(), "missing_column") {
		t.Fatalf("expected unknown column mapping error, got %v", err)
	}
}

type noopSQLExecutor struct{}

func (noopSQLExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("noop SQL executor cannot query")
}

func (noopSQLExecutor) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, errors.New("noop SQL executor cannot exec")
}

type memoryRows struct {
	columns []string
	values  [][]any
	index   int
}

func (r *memoryRows) Columns() ([]string, error) {
	return append([]string(nil), r.columns...), nil
}

func (r *memoryRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}

func (r *memoryRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.values) {
		return errors.New("memory rows is not positioned")
	}
	row := r.values[r.index-1]
	for index := range dest {
		if index >= len(row) {
			break
		}
		assignOutDestination(dest[index], row[index])
	}
	return nil
}

func (r *memoryRows) Err() error {
	return nil
}

func (r *memoryRows) Close() error {
	return nil
}
