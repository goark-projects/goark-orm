package ormtest

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/internal/jsoncodec"
)

const (
	// DefaultCompatibilityTable 是标准兼容套件使用的默认临时表名。
	DefaultCompatibilityTable = "goark_orm_compat_users"

	defaultCompatibilityNamespace   = "goark.ormtest.CompatibilityMapper"
	defaultCompatibilityResultMapID = "CompatibilityRecordMap"
	compatibilityJSONHandler        = "ormtest_json"
)

// CompatibilityRecord 是标准真实库兼容套件使用的最小实体。
type CompatibilityRecord struct {
	ID        int64
	Name      string
	Age       int
	Profile   CompatibilityProfile
	CreatedAt string
}

// CompatibilityProfile 验证 TypeHandler 在入库和出库方向都参与转换。
type CompatibilityProfile struct {
	Role  string `json:"role"`
	Level int    `json:"level"`
}

type compatibilitySuiteOptions struct {
	table     string
	namespace string
	prefix    string
}

// CompatibilitySuiteOption 调整标准真实库兼容套件。
type CompatibilitySuiteOption func(*compatibilitySuiteOptions)

// WithCompatibilityTable 指定标准兼容套件使用的临时表名。
func WithCompatibilityTable(table string) CompatibilitySuiteOption {
	return func(options *compatibilitySuiteOptions) {
		options.table = strings.TrimSpace(table)
	}
}

// WithCompatibilityNamespace 指定标准兼容套件使用的 Mapper namespace。
func WithCompatibilityNamespace(namespace string) CompatibilitySuiteOption {
	return func(options *compatibilitySuiteOptions) {
		options.namespace = strings.TrimSpace(namespace)
	}
}

// WithCompatibilityEnvPrefix 指定从环境变量运行标准兼容套件时使用的前缀。
func WithCompatibilityEnvPrefix(prefix string) CompatibilitySuiteOption {
	return func(options *compatibilitySuiteOptions) {
		options.prefix = strings.TrimSpace(prefix)
	}
}

// NewCompatibilitySuiteConfig 创建覆盖 CRUD、分页、批处理和 TypeHandler 的标准真实库套件。
func NewCompatibilitySuiteConfig(dbType orm.DbType, options ...CompatibilitySuiteOption) (DatabaseSuiteConfig, error) {
	opts := normalizeCompatibilitySuiteOptions(options...)
	dialect, err := orm.NewDialect(dbType)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	quotedTable, err := quoteCompatibilityTable(dialect, opts.table)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	setupSQL, cleanupSQL, err := compatibilityDDL(dbType, quotedTable)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	registry, err := newCompatibilityRegistry(opts.namespace, quotedTable)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	return DatabaseSuiteConfig{
		DBType:     dbType,
		Dialect:    dialect,
		Registry:   registry,
		SetupSQL:   setupSQL,
		CleanupSQL: cleanupSQL,
		Cases:      compatibilityCases(opts.namespace, quotedTable),
	}, nil
}

// RunCompatibilitySuiteFromEnv 从环境变量读取连接信息并运行标准真实库兼容套件。
func RunCompatibilitySuiteFromEnv(t *testing.T, options ...CompatibilitySuiteOption) {
	t.Helper()
	opts := normalizeCompatibilitySuiteOptions(options...)
	base, configured, err := LoadDatabaseSuiteConfigFromEnv(opts.prefix)
	if err != nil {
		t.Fatalf("load database suite config failed: %v", err)
	}
	if !configured {
		prefix := envPrefix(opts.prefix)
		t.Skipf("set %s_DRIVER, %s_DSN and %s_DBTYPE to run compatibility suite", prefix, prefix, prefix)
	}
	compatibility, err := NewCompatibilitySuiteConfig(base.DBType, options...)
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	base.Dialect = compatibility.Dialect
	base.Registry = compatibility.Registry
	base.SetupSQL = append(append([]string(nil), compatibility.SetupSQL...), base.SetupSQL...)
	base.CleanupSQL = append(append([]string(nil), base.CleanupSQL...), compatibility.CleanupSQL...)
	base.Cases = append(append([]DatabaseCase(nil), compatibility.Cases...), base.Cases...)
	RunDatabaseSuite(t, base)
}

func normalizeCompatibilitySuiteOptions(options ...CompatibilitySuiteOption) compatibilitySuiteOptions {
	opts := compatibilitySuiteOptions{
		table:     DefaultCompatibilityTable,
		namespace: defaultCompatibilityNamespace,
		prefix:    DefaultEnvPrefix,
	}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if opts.table == "" {
		opts.table = DefaultCompatibilityTable
	}
	if opts.namespace == "" {
		opts.namespace = defaultCompatibilityNamespace
	}
	if opts.prefix == "" {
		opts.prefix = DefaultEnvPrefix
	}
	return opts
}

func compatibilityDDL(dbType orm.DbType, quotedTable string) ([]string, []string, error) {
	switch dbType {
	case orm.DbTypePostgres:
		return []string{
			"DROP TABLE IF EXISTS " + quotedTable,
			"CREATE TABLE " + quotedTable + " (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile TEXT NOT NULL, created_at VARCHAR(40) NOT NULL)",
		}, []string{"DROP TABLE IF EXISTS " + quotedTable}, nil
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		return []string{
			"DROP TABLE IF EXISTS " + quotedTable,
			"CREATE TABLE " + quotedTable + " (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile TEXT NOT NULL, created_at VARCHAR(40) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		}, []string{"DROP TABLE IF EXISTS " + quotedTable}, nil
	case orm.DbTypeQuestion, orm.DbTypeSQLite:
		return []string{
			"DROP TABLE IF EXISTS " + quotedTable,
			"CREATE TABLE " + quotedTable + " (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile TEXT NOT NULL, created_at VARCHAR(40) NOT NULL)",
		}, []string{"DROP TABLE IF EXISTS " + quotedTable}, nil
	default:
		return nil, nil, fmt.Errorf("goark-orm: compatibility suite does not support db type %q", dbType)
	}
}

func newCompatibilityRegistry(namespace string, table string) (*orm.Registry, error) {
	registry := orm.NewRegistry()
	if err := registry.RegisterTypeHandler(compatibilityJSONHandler, compatibilityJSONTypeHandler{}); err != nil {
		return nil, err
	}
	if err := registry.RegisterEntity(compatibilityEntityMeta(table)); err != nil {
		return nil, err
	}
	if err := registry.RegisterMapper(compatibilityMapperMeta(namespace, table)); err != nil {
		return nil, err
	}
	return registry, nil
}

func compatibilityEntityMeta(table string) orm.EntityMeta {
	return orm.EntityMeta{
		TypeName: "CompatibilityRecord",
		Table:    table,
		Columns: []orm.ColumnMeta{
			{FieldName: "ID", ColumnName: "id", PrimaryKey: true},
			{FieldName: "Name", ColumnName: "name"},
			{FieldName: "Age", ColumnName: "age"},
			{FieldName: "Profile", ColumnName: "profile", TypeHandler: compatibilityJSONHandler},
			{FieldName: "CreatedAt", ColumnName: "created_at"},
		},
	}
}

func compatibilityMapperMeta(namespace string, table string) orm.MapperMeta {
	statements := []orm.StatementMeta{
		compatibilityStatement(namespace, "Insert", orm.StatementCommandInsert, "insert into "+table+" (id, name, age, profile, created_at) values (#{record.ID}, #{record.Name}, #{record.Age}, #{record.Profile}, #{record.CreatedAt})", ""),
		compatibilityStatement(namespace, "Update", orm.StatementCommandUpdate, "update "+table+" set name = #{name}, age = #{age} where id = #{id}", ""),
		compatibilityStatement(namespace, "Delete", orm.StatementCommandDelete, "delete from "+table+" where id = #{id}", ""),
		compatibilityStatement(namespace, "SelectOne", orm.StatementCommandSelect, "select id, name, age, profile, created_at from "+table+" where id = #{id}", defaultCompatibilityResultMapID),
		compatibilityStatement(namespace, "SelectList", orm.StatementCommandSelect, "select id, name, age, profile, created_at from "+table+" where age >= #{minAge} order by id", defaultCompatibilityResultMapID),
		compatibilityStatement(namespace, "Count", orm.StatementCommandSelect, "select count(*) from "+table+" where age >= #{minAge}", ""),
	}
	return orm.MapperMeta{
		TypeName:   "CompatibilityMapper",
		Namespace:  namespace,
		ResultMaps: []orm.ResultMapMeta{compatibilityResultMap()},
		Statements: statements,
	}
}

func compatibilityStatement(namespace string, id string, command orm.StatementCommand, sqlText string, resultMap string) orm.StatementMeta {
	return orm.StatementMeta{
		ID:            id,
		Namespace:     namespace,
		FullName:      namespace + "." + id,
		Command:       command,
		Source:        orm.StatementSourceAnnotation,
		SQL:           sqlText,
		ResultMap:     resultMap,
		ParameterType: "CompatibilityRecord",
	}
}

func compatibilityResultMap() orm.ResultMapMeta {
	return orm.ResultMapMeta{
		ID:       defaultCompatibilityResultMapID,
		TypeName: "CompatibilityRecord",
		Fields: []orm.ResultFieldMeta{
			{Property: "ID", Column: "id", ID: true},
			{Property: "Name", Column: "name"},
			{Property: "Age", Column: "age"},
			{Property: "Profile", Column: "profile", TypeHandler: compatibilityJSONHandler},
			{Property: "CreatedAt", Column: "created_at"},
		},
	}
}

func compatibilityCases(namespace string, table string) []DatabaseCase {
	return []DatabaseCase{
		compatibilityInsertCase(namespace, table),
		compatibilityQueryOneCase(namespace, table),
		compatibilityUpdateCase(namespace, table),
		compatibilityQueryListCase(namespace, table),
		compatibilityPageCase(namespace, table),
		compatibilityBatchCase(namespace, table),
		compatibilityDeleteCase(namespace, table),
	}
}

func compatibilityInsertCase(namespace string, table string) DatabaseCase {
	record := CompatibilityRecord{ID: 1, Name: "Alice", Age: 31, Profile: CompatibilityProfile{Role: "admin", Level: 7}, CreatedAt: "2026-08-26T00:00:00Z"}
	return ExecStatementCase("compatibility-insert", compatibilityCaseStatement(namespace, table, "Insert"), orm.NamedArgs{"record": record}, func(result orm.Result) error {
		if result.RowsAffected != 1 {
			return fmt.Errorf("insert rows affected = %d", result.RowsAffected)
		}
		return nil
	})
}

func compatibilityQueryOneCase(namespace string, table string) DatabaseCase {
	return QueryOneStatementCase("compatibility-query-one", compatibilityCaseStatement(namespace, table, "SelectOne"), orm.NamedArgs{"id": int64(1)}, func(record CompatibilityRecord) error {
		if record.ID != 1 || record.Name != "Alice" || record.Age != 31 {
			return fmt.Errorf("unexpected record %#v", record)
		}
		if record.Profile.Role != "admin" || record.Profile.Level != 7 {
			return fmt.Errorf("unexpected profile %#v", record.Profile)
		}
		return nil
	})
}

func compatibilityUpdateCase(namespace string, table string) DatabaseCase {
	return ExecStatementCase("compatibility-update", compatibilityCaseStatement(namespace, table, "Update"), orm.NamedArgs{"id": int64(1), "name": "Alice Updated", "age": 32}, func(result orm.Result) error {
		if result.RowsAffected != 1 {
			return fmt.Errorf("update rows affected = %d", result.RowsAffected)
		}
		return nil
	})
}

func compatibilityQueryListCase(namespace string, table string) DatabaseCase {
	return QueryStatementCase("compatibility-query-list", compatibilityCaseStatement(namespace, table, "SelectList"), orm.NamedArgs{"minAge": 30}, func(records []CompatibilityRecord) error {
		if len(records) != 1 {
			return fmt.Errorf("query list length = %d", len(records))
		}
		if records[0].Name != "Alice Updated" || records[0].Age != 32 {
			return fmt.Errorf("unexpected updated record %#v", records[0])
		}
		return nil
	})
}

func compatibilityPageCase(namespace string, table string) DatabaseCase {
	return PageStatementCase("compatibility-page", compatibilityCaseStatement(namespace, table, "SelectList"), orm.NamedArgs{"minAge": 30}, orm.PageRequest{Current: 1, Size: 1, SearchCount: true}, func(page orm.Page[CompatibilityRecord]) error {
		if page.Total != 1 || page.Pages != 1 || len(page.Records) != 1 {
			return fmt.Errorf("unexpected page %#v", page)
		}
		return nil
	})
}

func compatibilityBatchCase(namespace string, table string) DatabaseCase {
	return DatabaseCase{
		Name: "compatibility-batch",
		Run: func(ctx context.Context, session *orm.SQLSession, _ *sql.DB) error {
			batch, err := orm.NewBatchSession(session)
			if err != nil {
				return err
			}
			records := []CompatibilityRecord{
				{ID: 2, Name: "Bob", Age: 22, Profile: CompatibilityProfile{Role: "user", Level: 2}, CreatedAt: "2026-08-26T00:01:00Z"},
				{ID: 3, Name: "Cindy", Age: 27, Profile: CompatibilityProfile{Role: "user", Level: 3}, CreatedAt: "2026-08-26T00:02:00Z"},
			}
			for _, record := range records {
				if _, err := batch.ExecStatement(ctx, compatibilityCaseStatement(namespace, table, "Insert"), orm.NamedArgs{"record": record}); err != nil {
					return err
				}
			}
			results, err := batch.Flush(ctx)
			if err != nil {
				return err
			}
			if len(results) != len(records) {
				return fmt.Errorf("batch results length = %d", len(results))
			}
			var count int64
			if err := session.QueryOneStatement(ctx, compatibilityCaseStatement(namespace, table, "Count"), orm.NamedArgs{"minAge": 20}, &count); err != nil {
				return err
			}
			if count != 3 {
				return fmt.Errorf("batch count = %d", count)
			}
			return nil
		},
	}
}

func compatibilityDeleteCase(namespace string, table string) DatabaseCase {
	return ExecStatementCase("compatibility-delete", compatibilityCaseStatement(namespace, table, "Delete"), orm.NamedArgs{"id": int64(1)}, func(result orm.Result) error {
		if result.RowsAffected != 1 {
			return fmt.Errorf("delete rows affected = %d", result.RowsAffected)
		}
		return nil
	})
}

func compatibilityCaseStatement(namespace string, table string, id string) orm.StatementMeta {
	mapper := compatibilityMapperMeta(namespace, table)
	for _, statement := range mapper.Statements {
		if statement.ID == id {
			return statement
		}
	}
	return orm.StatementMeta{}
}

type compatibilityJSONTypeHandler struct{}

func (compatibilityJSONTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		data, err := jsoncodec.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal compatibility json failed: %w", err)
		}
		return string(data), nil
	}
}

func (compatibilityJSONTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	if target == nil || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return jsoncodec.UnmarshalString(typed, target)
	case []byte:
		return jsoncodec.Unmarshal(typed, target)
	default:
		data, err := jsoncodec.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal database json value failed: %w", err)
		}
		return jsoncodec.Unmarshal(data, target)
	}
}

func quoteCompatibilityTable(dialect orm.Dialect, table string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", fmt.Errorf("goark-orm: compatibility table is required")
	}
	parts := strings.Split(table, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !validCompatibilityIdentifier(part) {
			return "", fmt.Errorf("goark-orm: invalid compatibility table identifier %q", table)
		}
		quoted = append(quoted, dialect.QuoteIdent(part))
	}
	return strings.Join(quoted, "."), nil
}

func validCompatibilityIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			continue
		}
		if index > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}
