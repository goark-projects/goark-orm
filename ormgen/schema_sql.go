package ormgen

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	orm "goark.dev/orm"
)

// SQLQueryer 是 *sql.DB 和 *sql.Tx 的最小查询接口。
type SQLQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// SchemaRows 描述 schema 查询返回的最小行集接口。
type SchemaRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// SchemaQueryer 适配真实数据库或测试替身的 schema 查询能力。
type SchemaQueryer interface {
	QuerySchema(ctx context.Context, query string, args ...any) (SchemaRows, error)
}

// SchemaQueryerFunc 将函数适配为 SchemaQueryer。
type SchemaQueryerFunc func(ctx context.Context, query string, args ...any) (SchemaRows, error)

// QuerySchema 执行函数式 schema 查询器。
func (f SchemaQueryerFunc) QuerySchema(ctx context.Context, query string, args ...any) (SchemaRows, error) {
	if f == nil {
		return nil, fmt.Errorf("goark-orm: schema queryer is nil")
	}
	return f(ctx, query, args...)
}

// SQLSchemaQuery 描述一次 schema SQL 查询。
type SQLSchemaQuery struct {
	SQL  string
	Args []any
}

// SQLSchemaDialect 生成不同数据库的 schema 查询 SQL。
type SQLSchemaDialect interface {
	TablesQuery(request SchemaIntrospectionRequest) (SQLSchemaQuery, error)
	ColumnsQuery(request SchemaIntrospectionRequest, table SchemaTable) (SQLSchemaQuery, error)
}

// SQLSchemaIntrospector 使用 database/sql 查询真实数据库结构。
type SQLSchemaIntrospector struct {
	queryer SchemaQueryer
	dialect SQLSchemaDialect
}

// NewSQLSchemaIntrospector 创建基于 database/sql 的 schema 读取器。
func NewSQLSchemaIntrospector(queryer SQLQueryer, dialect SQLSchemaDialect) (*SQLSchemaIntrospector, error) {
	if queryer == nil {
		return nil, fmt.Errorf("goark-orm: SQL queryer is nil")
	}
	return NewSQLSchemaIntrospectorWithQueryer(sqlSchemaQueryer{queryer: queryer}, dialect)
}

// NewSQLSchemaIntrospectorWithQueryer 使用自定义查询器创建 schema 读取器。
func NewSQLSchemaIntrospectorWithQueryer(queryer SchemaQueryer, dialect SQLSchemaDialect) (*SQLSchemaIntrospector, error) {
	if queryer == nil {
		return nil, fmt.Errorf("goark-orm: schema queryer is nil")
	}
	if dialect == nil {
		return nil, fmt.Errorf("goark-orm: SQL schema dialect is nil")
	}
	return &SQLSchemaIntrospector{queryer: queryer, dialect: dialect}, nil
}

// NewSQLSchemaDialect 根据数据库类型创建 schema SQL 方言。
func NewSQLSchemaDialect(dbType orm.DbType) (SQLSchemaDialect, error) {
	switch dbType {
	case "", orm.DbTypeQuestion, orm.DbTypePostgres:
		return PostgresSQLSchemaDialect{}, nil
	case orm.DbTypeMySQL:
		return MySQLSQLSchemaDialect{}, nil
	case orm.DbTypeMariaDB:
		return MariaDBSQLSchemaDialect{}, nil
	case orm.DbTypeSQLite:
		return SQLiteSQLSchemaDialect{}, nil
	case orm.DbTypeSQLServer:
		return SQLServerSQLSchemaDialect{}, nil
	case orm.DbTypeOracle:
		return OracleSQLSchemaDialect{}, nil
	default:
		return nil, fmt.Errorf("goark-orm: unsupported SQL schema db type %q", dbType)
	}
}

// IntrospectSchema 读取表和列元数据，并转换为统一 schema 中间模型。
func (i *SQLSchemaIntrospector) IntrospectSchema(ctx context.Context, request SchemaIntrospectionRequest) (SchemaModel, error) {
	if ctx == nil {
		return SchemaModel{}, fmt.Errorf("goark-orm: context is nil")
	}
	if i == nil || i.queryer == nil || i.dialect == nil {
		return SchemaModel{}, fmt.Errorf("goark-orm: SQL schema introspector is nil")
	}
	tableQuery, err := i.dialect.TablesQuery(request)
	if err != nil {
		return SchemaModel{}, err
	}
	tableRows, err := i.queryer.QuerySchema(ctx, tableQuery.SQL, tableQuery.Args...)
	if err != nil {
		return SchemaModel{}, err
	}
	tables, err := scanSQLSchemaTables(tableRows)
	if err != nil {
		return SchemaModel{}, err
	}
	tables, err = selectRequestedSchemaTables(tables, request.Tables)
	if err != nil {
		return SchemaModel{}, err
	}
	for index := range tables {
		columnQuery, err := i.dialect.ColumnsQuery(request, tables[index])
		if err != nil {
			return SchemaModel{}, err
		}
		columnRows, err := i.queryer.QuerySchema(ctx, columnQuery.SQL, columnQuery.Args...)
		if err != nil {
			return SchemaModel{}, err
		}
		columns, err := scanSQLSchemaColumns(columnRows)
		if err != nil {
			return SchemaModel{}, err
		}
		tables[index].Columns = columns
	}
	return SchemaModel{Tables: tables}, nil
}

type sqlSchemaQueryer struct {
	queryer SQLQueryer
}

func (q sqlSchemaQueryer) QuerySchema(ctx context.Context, query string, args ...any) (SchemaRows, error) {
	return q.queryer.QueryContext(ctx, query, args...)
}

func scanSQLSchemaTables(rows SchemaRows) ([]SchemaTable, error) {
	defer rows.Close()
	tables := make([]SchemaTable, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name != "" {
			tables = append(tables, SchemaTable{Name: name})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}

func scanSQLSchemaColumns(rows SchemaRows) ([]SchemaColumn, error) {
	defer rows.Close()
	columns := make([]SchemaColumn, 0)
	for rows.Next() {
		var name, dbType, nullable, size, scale, defaultValue, primary, auto sql.NullString
		if err := rows.Scan(&name, &dbType, &nullable, &size, &scale, &defaultValue, &primary, &auto); err != nil {
			return nil, err
		}
		columnName := strings.TrimSpace(name.String)
		if columnName == "" {
			continue
		}
		columns = append(columns, SchemaColumn{
			Name:          columnName,
			DBType:        strings.TrimSpace(dbType.String),
			Nullable:      parseSQLSchemaNullable(nullable),
			Size:          parseSQLSchemaInt(size),
			NumericScale:  parseSQLSchemaInt(scale),
			DefaultValue:  strings.TrimSpace(defaultValue.String),
			PrimaryKey:    parseSQLSchemaBool(primary),
			AutoIncrement: parseSQLSchemaBool(auto),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

func selectRequestedSchemaTables(discovered []SchemaTable, requested []string) ([]SchemaTable, error) {
	requested = compactSchemaNames(requested)
	if len(requested) == 0 {
		return discovered, nil
	}
	out := make([]SchemaTable, 0, len(requested))
	for _, name := range requested {
		table, ok := findSchemaTable(discovered, name)
		if !ok {
			return nil, fmt.Errorf("goark-orm: schema table %q not found", name)
		}
		out = append(out, table)
	}
	return out, nil
}

func findSchemaTable(tables []SchemaTable, name string) (SchemaTable, bool) {
	for _, table := range tables {
		if strings.EqualFold(strings.TrimSpace(table.Name), name) {
			return table, true
		}
	}
	return SchemaTable{}, false
}

func compactSchemaNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func parseSQLSchemaNullable(value sql.NullString) *bool {
	if !value.Valid {
		return nil
	}
	nullable := parseSQLSchemaBool(sql.NullString{String: value.String, Valid: true})
	return &nullable
}

func parseSQLSchemaBool(value sql.NullString) bool {
	if !value.Valid {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value.String)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func parseSQLSchemaInt(value sql.NullString) *int {
	if !value.Valid {
		return nil
	}
	text := strings.TrimSpace(value.String)
	if text == "" {
		return nil
	}
	parsed, err := strconv.Atoi(text)
	if err != nil || parsed < 0 {
		return nil
	}
	return &parsed
}

func schemaNameOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

// PostgresSQLSchemaDialect 生成 PostgreSQL information_schema 查询。
type PostgresSQLSchemaDialect struct{}

// TablesQuery 返回 PostgreSQL 表查询。
func (PostgresSQLSchemaDialect) TablesQuery(request SchemaIntrospectionRequest) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name",
		Args: []any{schemaNameOrDefault(request.Schema, "public")},
	}, nil
}

// ColumnsQuery 返回 PostgreSQL 列查询。
func (PostgresSQLSchemaDialect) ColumnsQuery(request SchemaIntrospectionRequest, table SchemaTable) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT c.column_name, c.udt_name, c.is_nullable, c.character_maximum_length::text, c.numeric_scale::text, c.column_default, CASE WHEN tc.constraint_type = 'PRIMARY KEY' THEN 'true' ELSE 'false' END, CASE WHEN c.is_identity = 'YES' OR c.column_default LIKE 'nextval(%' THEN 'true' ELSE 'false' END FROM information_schema.columns c LEFT JOIN information_schema.key_column_usage kcu ON kcu.table_schema = c.table_schema AND kcu.table_name = c.table_name AND kcu.column_name = c.column_name LEFT JOIN information_schema.table_constraints tc ON tc.constraint_schema = kcu.constraint_schema AND tc.constraint_name = kcu.constraint_name AND tc.constraint_type = 'PRIMARY KEY' WHERE c.table_schema = $1 AND c.table_name = $2 ORDER BY c.ordinal_position",
		Args: []any{schemaNameOrDefault(request.Schema, "public"), strings.TrimSpace(table.Name)},
	}, nil
}

// MySQLSQLSchemaDialect 生成 MySQL information_schema 查询。
type MySQLSQLSchemaDialect struct{}

// TablesQuery 返回 MySQL 表查询。
func (MySQLSQLSchemaDialect) TablesQuery(request SchemaIntrospectionRequest) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT table_name FROM information_schema.tables WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE()) AND table_type = 'BASE TABLE' ORDER BY table_name",
		Args: []any{strings.TrimSpace(request.Schema)},
	}, nil
}

// ColumnsQuery 返回 MySQL 列查询。
func (MySQLSQLSchemaDialect) ColumnsQuery(request SchemaIntrospectionRequest, table SchemaTable) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT c.column_name, c.column_type, c.is_nullable, CAST(c.character_maximum_length AS CHAR), CAST(c.numeric_scale AS CHAR), c.column_default, IF(c.column_key = 'PRI', 'true', 'false'), IF(c.extra LIKE '%auto_increment%', 'true', 'false') FROM information_schema.columns c WHERE c.table_schema = COALESCE(NULLIF(?, ''), DATABASE()) AND c.table_name = ? ORDER BY c.ordinal_position",
		Args: []any{strings.TrimSpace(request.Schema), strings.TrimSpace(table.Name)},
	}, nil
}

// MariaDBSQLSchemaDialect 生成 MariaDB information_schema 查询。
type MariaDBSQLSchemaDialect struct {
	MySQLSQLSchemaDialect
}

// SQLiteSQLSchemaDialect 生成 SQLite schema 查询。
type SQLiteSQLSchemaDialect struct{}

// TablesQuery 返回 SQLite 表查询。
func (SQLiteSQLSchemaDialect) TablesQuery(SchemaIntrospectionRequest) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL: "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
	}, nil
}

// ColumnsQuery 返回 SQLite 列查询。
func (SQLiteSQLSchemaDialect) ColumnsQuery(_ SchemaIntrospectionRequest, table SchemaTable) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT name, type, CASE WHEN \"notnull\" = 0 THEN 'true' ELSE 'false' END, NULL, NULL, CAST(dflt_value AS TEXT), CASE WHEN pk > 0 THEN 'true' ELSE 'false' END, CASE WHEN pk = 1 AND lower(type) = 'integer' THEN 'true' ELSE 'false' END FROM pragma_table_info(?) ORDER BY cid",
		Args: []any{strings.TrimSpace(table.Name)},
	}, nil
}

// SQLServerSQLSchemaDialect 生成 SQL Server sys schema 查询。
type SQLServerSQLSchemaDialect struct{}

// TablesQuery 返回 SQL Server 表查询。
func (SQLServerSQLSchemaDialect) TablesQuery(request SchemaIntrospectionRequest) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT t.name FROM sys.tables t INNER JOIN sys.schemas s ON s.schema_id = t.schema_id WHERE s.name = COALESCE(NULLIF(@p1, ''), SCHEMA_NAME()) ORDER BY t.name",
		Args: []any{strings.TrimSpace(request.Schema)},
	}, nil
}

// ColumnsQuery 返回 SQL Server 列查询。
func (SQLServerSQLSchemaDialect) ColumnsQuery(request SchemaIntrospectionRequest, table SchemaTable) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT c.name, ty.name, CASE WHEN c.is_nullable = 1 THEN 'true' ELSE 'false' END, NULLIF(CAST(c.max_length AS varchar(32)), '-1'), CAST(c.scale AS varchar(32)), OBJECT_DEFINITION(c.default_object_id), CASE WHEN pk.column_id IS NULL THEN 'false' ELSE 'true' END, CASE WHEN c.is_identity = 1 THEN 'true' ELSE 'false' END FROM sys.columns c INNER JOIN sys.tables t ON t.object_id = c.object_id INNER JOIN sys.schemas s ON s.schema_id = t.schema_id INNER JOIN sys.types ty ON ty.user_type_id = c.user_type_id OUTER APPLY (SELECT ic.column_id FROM sys.indexes i INNER JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id WHERE i.object_id = t.object_id AND i.is_primary_key = 1 AND ic.column_id = c.column_id) pk WHERE s.name = COALESCE(NULLIF(@p1, ''), SCHEMA_NAME()) AND t.name = @p2 ORDER BY c.column_id",
		Args: []any{strings.TrimSpace(request.Schema), strings.TrimSpace(table.Name)},
	}, nil
}

// OracleSQLSchemaDialect 生成 Oracle all_* 元数据查询。
type OracleSQLSchemaDialect struct{}

// TablesQuery 返回 Oracle 表查询。
func (OracleSQLSchemaDialect) TablesQuery(request SchemaIntrospectionRequest) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT table_name FROM all_tables WHERE owner = COALESCE(NULLIF(:1, ''), SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) ORDER BY table_name",
		Args: []any{strings.ToUpper(strings.TrimSpace(request.Schema))},
	}, nil
}

// ColumnsQuery 返回 Oracle 列查询。
func (OracleSQLSchemaDialect) ColumnsQuery(request SchemaIntrospectionRequest, table SchemaTable) (SQLSchemaQuery, error) {
	return SQLSchemaQuery{
		SQL:  "SELECT c.column_name, c.data_type, CASE WHEN c.nullable = 'Y' THEN 'true' ELSE 'false' END, TO_CHAR(c.char_length), TO_CHAR(c.data_scale), c.data_default, CASE WHEN pk.column_name IS NULL THEN 'false' ELSE 'true' END, CASE WHEN c.identity_column = 'YES' THEN 'true' ELSE 'false' END FROM all_tab_columns c LEFT JOIN (SELECT acc.owner, acc.table_name, acc.column_name FROM all_constraints ac INNER JOIN all_cons_columns acc ON acc.owner = ac.owner AND acc.constraint_name = ac.constraint_name WHERE ac.constraint_type = 'P') pk ON pk.owner = c.owner AND pk.table_name = c.table_name AND pk.column_name = c.column_name WHERE c.owner = COALESCE(NULLIF(:1, ''), SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) AND c.table_name = :2 ORDER BY c.column_id",
		Args: []any{strings.ToUpper(strings.TrimSpace(request.Schema)), strings.ToUpper(strings.TrimSpace(table.Name))},
	}, nil
}
