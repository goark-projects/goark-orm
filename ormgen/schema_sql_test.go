package ormgen

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"

	orm "goark.dev/orm"
)

func TestSQLSchemaIntrospector_whenTablesRequested_shouldBuildSchemaModel(t *testing.T) {
	queryer := &recordingSchemaQueryer{
		results: []*fakeSchemaRows{
			newFakeSchemaRows([][]any{{"sys_role"}, {"sys_user"}}),
			newFakeSchemaRows([][]any{
				{"id", "int8", "NO", nil, nil, "nextval('sys_user_id_seq'::regclass)", "true", "true"},
				{"user_name", "varchar", "YES", "64", nil, nil, "false", "false"},
				{"created_at", "timestamp", "NO", nil, nil, nil, "false", "false"},
			}),
		},
	}
	introspector, err := NewSQLSchemaIntrospectorWithQueryer(queryer, PostgresSQLSchemaDialect{})
	if err != nil {
		t.Fatalf("new SQL schema introspector failed: %v", err)
	}

	schema, err := introspector.IntrospectSchema(context.Background(), SchemaIntrospectionRequest{
		Schema: "public",
		Tables: []string{"sys_user"},
	})
	if err != nil {
		t.Fatalf("introspect schema failed: %v", err)
	}

	if len(queryer.queries) != 2 {
		t.Fatalf("expected table and column queries, got %#v", queryer.queries)
	}
	if !strings.Contains(queryer.queries[0], "information_schema.tables") {
		t.Fatalf("unexpected table query %q", queryer.queries[0])
	}
	if !reflect.DeepEqual(queryer.args[1], []any{"public", "sys_user"}) {
		t.Fatalf("unexpected column query args %#v", queryer.args[1])
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "sys_user" {
		t.Fatalf("unexpected schema tables %#v", schema.Tables)
	}
	columns := schema.Tables[0].Columns
	if len(columns) != 3 {
		t.Fatalf("unexpected columns %#v", columns)
	}
	if !columns[0].PrimaryKey || !columns[0].AutoIncrement || columns[0].DefaultValue == "" {
		t.Fatalf("unexpected id column %#v", columns[0])
	}
	if columns[1].Nullable == nil || !*columns[1].Nullable || columns[1].Size == nil || *columns[1].Size != 64 {
		t.Fatalf("unexpected name column %#v", columns[1])
	}
}

func TestSQLSchemaIntrospector_whenRequestedTableMissing_shouldReturnError(t *testing.T) {
	queryer := &recordingSchemaQueryer{
		results: []*fakeSchemaRows{newFakeSchemaRows([][]any{{"sys_user"}})},
	}
	introspector, err := NewSQLSchemaIntrospectorWithQueryer(queryer, PostgresSQLSchemaDialect{})
	if err != nil {
		t.Fatalf("new SQL schema introspector failed: %v", err)
	}

	_, err = introspector.IntrospectSchema(context.Background(), SchemaIntrospectionRequest{Tables: []string{"missing_user"}})
	if err == nil || !strings.Contains(err.Error(), `schema table "missing_user" not found`) {
		t.Fatalf("expected missing table error, got %v", err)
	}
}

func TestNewSQLSchemaDialect_whenDbTypeProvided_shouldReturnDialect(t *testing.T) {
	cases := []struct {
		dbType orm.DbType
		want   any
	}{
		{orm.DbTypePostgres, PostgresSQLSchemaDialect{}},
		{orm.DbTypeMySQL, MySQLSQLSchemaDialect{}},
		{orm.DbTypeMariaDB, MariaDBSQLSchemaDialect{}},
		{orm.DbTypeSQLite, SQLiteSQLSchemaDialect{}},
		{orm.DbTypeSQLServer, SQLServerSQLSchemaDialect{}},
		{orm.DbTypeOracle, OracleSQLSchemaDialect{}},
	}
	for _, item := range cases {
		dialect, err := NewSQLSchemaDialect(item.dbType)
		if err != nil {
			t.Fatalf("new dialect for %s failed: %v", item.dbType, err)
		}
		if reflect.TypeOf(dialect) != reflect.TypeOf(item.want) {
			t.Fatalf("unexpected dialect for %s: %T", item.dbType, dialect)
		}
	}
}

type recordingSchemaQueryer struct {
	queries []string
	args    [][]any
	results []*fakeSchemaRows
}

func (q *recordingSchemaQueryer) QuerySchema(_ context.Context, query string, args ...any) (SchemaRows, error) {
	q.queries = append(q.queries, query)
	q.args = append(q.args, append([]any(nil), args...))
	if len(q.results) == 0 {
		return nil, fmt.Errorf("unexpected query %q", query)
	}
	rows := q.results[0]
	q.results = q.results[1:]
	return rows, nil
}

type fakeSchemaRows struct {
	values [][]any
	index  int
	closed bool
}

func newFakeSchemaRows(values [][]any) *fakeSchemaRows {
	return &fakeSchemaRows{values: values}
}

func (r *fakeSchemaRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}

func (r *fakeSchemaRows) Scan(dest ...any) error {
	row := r.values[r.index-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan destination count %d does not match row count %d", len(dest), len(row))
	}
	for index, value := range row {
		switch target := dest[index].(type) {
		case *string:
			text, _ := value.(string)
			*target = text
		case *sql.NullString:
			if value == nil {
				*target = sql.NullString{}
				continue
			}
			*target = sql.NullString{String: fmt.Sprint(value), Valid: true}
		default:
			return fmt.Errorf("unsupported scan target %T", dest[index])
		}
	}
	return nil
}

func (r *fakeSchemaRows) Err() error {
	return nil
}

func (r *fakeSchemaRows) Close() error {
	r.closed = true
	return nil
}
