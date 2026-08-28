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
	compatibilityKeyTableSuffix     = "_keys"
	compatibilityCallRoutineSuffix  = "_report"
)

// SupportedCompatibilityDBTypes 返回标准真实库兼容套件当前承诺支持的数据库类型。
func SupportedCompatibilityDBTypes() []orm.DbType {
	return []orm.DbType{orm.DbTypePostgres, orm.DbTypeMySQL, orm.DbTypeMariaDB, orm.DbTypeSQLite, orm.DbTypeSQLServer, orm.DbTypeOracle}
}

// IsCompatibilityDBTypeSupported 判断标准真实库兼容套件是否支持指定数据库类型。
func IsCompatibilityDBTypeSupported(dbType orm.DbType) bool {
	switch dbType {
	case orm.DbTypePostgres, orm.DbTypeMySQL, orm.DbTypeMariaDB, orm.DbTypeSQLite, orm.DbTypeSQLServer, orm.DbTypeOracle:
		return true
	default:
		return false
	}
}

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
	if !IsCompatibilityDBTypeSupported(dbType) {
		return DatabaseSuiteConfig{}, unsupportedCompatibilityDBTypeError(dbType)
	}
	dialect, err := orm.NewDialect(dbType)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	keyTable, err := compatibilityRelatedTable(opts.table, compatibilityKeyTableSuffix)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	callRoutine, err := compatibilityRelatedTable(opts.table, compatibilityCallRoutineSuffix)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	quotedTable, err := quoteCompatibilityTable(dialect, opts.table)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	quotedKeyTable, err := quoteCompatibilityTable(dialect, keyTable)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	quotedCallRoutine, err := quoteCompatibilityTable(dialect, callRoutine)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	setupSQL, cleanupSQL, err := compatibilityDDL(dbType, quotedTable, quotedKeyTable, quotedCallRoutine)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	callSQL := compatibilityCallSQL(dbType, quotedTable, quotedCallRoutine)
	registry, err := newCompatibilityRegistry(opts.namespace, quotedTable, callSQL)
	if err != nil {
		return DatabaseSuiteConfig{}, err
	}
	return DatabaseSuiteConfig{
		DBType:     dbType,
		Dialect:    dialect,
		Registry:   registry,
		SetupSQL:   setupSQL,
		CleanupSQL: cleanupSQL,
		Cases:      compatibilityCases(opts.namespace, opts.table, quotedTable, keyTable, callSQL),
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

func compatibilityDDL(dbType orm.DbType, quotedTable string, quotedKeyTable string, quotedCallRoutine string) ([]string, []string, error) {
	switch dbType {
	case orm.DbTypePostgres:
		return []string{
			"DROP FUNCTION IF EXISTS " + quotedCallRoutine + "(INTEGER)",
			"DROP TABLE IF EXISTS " + quotedKeyTable,
			"DROP TABLE IF EXISTS " + quotedTable,
			"CREATE TABLE " + quotedKeyTable + " (id BIGSERIAL PRIMARY KEY, name VARCHAR(64) NOT NULL)",
			"CREATE TABLE " + quotedTable + " (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile TEXT NOT NULL, created_at VARCHAR(40) NOT NULL)",
			compatibilityPostgresCallRoutineDDL(quotedTable, quotedCallRoutine),
		}, []string{"DROP FUNCTION IF EXISTS " + quotedCallRoutine + "(INTEGER)", "DROP TABLE IF EXISTS " + quotedKeyTable, "DROP TABLE IF EXISTS " + quotedTable}, nil
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		return []string{
			"DROP PROCEDURE IF EXISTS " + quotedCallRoutine,
			"DROP TABLE IF EXISTS " + quotedKeyTable,
			"DROP TABLE IF EXISTS " + quotedTable,
			"CREATE TABLE " + quotedKeyTable + " (id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(64) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			"CREATE TABLE " + quotedTable + " (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile TEXT NOT NULL, created_at VARCHAR(40) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			compatibilityMySQLCallRoutineDDL(quotedTable, quotedCallRoutine),
		}, []string{"DROP PROCEDURE IF EXISTS " + quotedCallRoutine, "DROP TABLE IF EXISTS " + quotedKeyTable, "DROP TABLE IF EXISTS " + quotedTable}, nil
	case orm.DbTypeSQLite:
		return []string{
			"DROP TABLE IF EXISTS " + quotedKeyTable,
			"DROP TABLE IF EXISTS " + quotedTable,
			"CREATE TABLE " + quotedKeyTable + " (id INTEGER PRIMARY KEY AUTOINCREMENT, name VARCHAR(64) NOT NULL)",
			"CREATE TABLE " + quotedTable + " (id INTEGER PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile TEXT NOT NULL, created_at VARCHAR(40) NOT NULL)",
		}, []string{"DROP TABLE IF EXISTS " + quotedKeyTable, "DROP TABLE IF EXISTS " + quotedTable}, nil
	case orm.DbTypeSQLServer:
		return []string{
			"DROP PROCEDURE IF EXISTS " + quotedCallRoutine,
			"DROP TABLE IF EXISTS " + quotedKeyTable,
			"DROP TABLE IF EXISTS " + quotedTable,
			"CREATE TABLE " + quotedKeyTable + " (id BIGINT IDENTITY(1,1) PRIMARY KEY, name NVARCHAR(64) NOT NULL)",
			"CREATE TABLE " + quotedTable + " (id BIGINT PRIMARY KEY, name NVARCHAR(64) NOT NULL, age INT NOT NULL, profile NVARCHAR(MAX) NOT NULL, created_at NVARCHAR(40) NOT NULL)",
			compatibilitySQLServerCallRoutineDDL(quotedTable, quotedCallRoutine),
		}, []string{"DROP PROCEDURE IF EXISTS " + quotedCallRoutine, "DROP TABLE IF EXISTS " + quotedKeyTable, "DROP TABLE IF EXISTS " + quotedTable}, nil
	case orm.DbTypeOracle:
		return []string{
			compatibilityOracleDropTableSQL(quotedKeyTable),
			compatibilityOracleDropTableSQL(quotedTable),
			"CREATE TABLE " + quotedKeyTable + " (id NUMBER(19) GENERATED BY DEFAULT ON NULL AS IDENTITY PRIMARY KEY, name VARCHAR2(64 CHAR) NOT NULL)",
			"CREATE TABLE " + quotedTable + " (id NUMBER(19) PRIMARY KEY, name VARCHAR2(64 CHAR) NOT NULL, age NUMBER(10) NOT NULL, profile CLOB NOT NULL, created_at VARCHAR2(40 CHAR) NOT NULL)",
		}, []string{compatibilityOracleDropTableSQL(quotedKeyTable), compatibilityOracleDropTableSQL(quotedTable)}, nil
	default:
		return nil, nil, unsupportedCompatibilityDBTypeError(dbType)
	}
}

func unsupportedCompatibilityDBTypeError(dbType orm.DbType) error {
	return fmt.Errorf("goark-orm: standard compatibility suite supports postgres, mysql, mariadb, sqlite, sqlserver and oracle, got %q", dbType)
}

func newCompatibilityRegistry(namespace string, table string, callSQL string) (*orm.Registry, error) {
	registry := orm.NewRegistry()
	if err := registry.RegisterTypeHandler(compatibilityJSONHandler, compatibilityJSONTypeHandler{}); err != nil {
		return nil, err
	}
	if err := registry.RegisterEntity(compatibilityEntityMeta(table)); err != nil {
		return nil, err
	}
	if err := registry.RegisterMapper(compatibilityMapperMeta(namespace, table, callSQL)); err != nil {
		return nil, err
	}
	if err := registry.Validate(); err != nil {
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

func compatibilityMapperMeta(namespace string, table string, callSQL string) orm.MapperMeta {
	statements := []orm.StatementMeta{
		compatibilityStatement(namespace, "Insert", orm.StatementCommandInsert, "insert into "+table+" (id, name, age, profile, created_at) values (#{record.ID}, #{record.Name}, #{record.Age}, #{record.Profile}, #{record.CreatedAt})", ""),
		compatibilityStatement(namespace, "Update", orm.StatementCommandUpdate, "update "+table+" set name = #{name}, age = #{age} where id = #{id}", ""),
		compatibilityStatement(namespace, "Delete", orm.StatementCommandDelete, "delete from "+table+" where id = #{id}", ""),
		compatibilityStatement(namespace, "SelectOne", orm.StatementCommandSelect, "select id, name, age, profile, created_at from "+table+" where id = #{id}", defaultCompatibilityResultMapID),
		compatibilityStatement(namespace, "SelectList", orm.StatementCommandSelect, "select id, name, age, profile, created_at from "+table+" where age >= #{minAge} order by id", defaultCompatibilityResultMapID),
		compatibilityStatement(namespace, "Count", orm.StatementCommandSelect, "select count(*) from "+table+" where age >= #{minAge}", ""),
	}
	if strings.TrimSpace(callSQL) != "" {
		statements = append(statements, compatibilityCallStatement(namespace, callSQL))
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

func compatibilityCases(namespace string, table string, quotedTable string, keyTable string, callSQL string) []DatabaseCase {
	hasCallable := strings.TrimSpace(callSQL) != ""
	capacity := 10
	if hasCallable {
		capacity++
	}
	cases := make([]DatabaseCase, 0, capacity)
	cases = append(cases,
		compatibilityInsertCase(namespace, quotedTable),
		compatibilityQueryOneCase(namespace, quotedTable),
		compatibilityUpdateCase(namespace, quotedTable),
		compatibilityUpsertCase(namespace, table, quotedTable),
		compatibilityGeneratedKeyCase(keyTable),
		compatibilityRowLockCase(table),
	)
	if hasCallable {
		cases = append(cases, compatibilityCallableCase(namespace, callSQL))
	}
	cases = append(cases,
		compatibilityQueryListCase(namespace, quotedTable),
		compatibilityPageCase(namespace, quotedTable),
		compatibilityBatchCase(namespace, quotedTable),
		compatibilityDeleteCase(namespace, quotedTable),
	)
	return cases
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

func compatibilityUpsertCase(namespace string, table string, quotedTable string) DatabaseCase {
	return DatabaseCase{
		Name: "compatibility-upsert",
		Run: func(ctx context.Context, session *orm.SQLSession, db *sql.DB) error {
			if !orm.DialectCapabilitiesOf(session.Dialect()).SupportsUpsert() {
				return nil
			}
			if err := execCompatibilityUpsert(ctx, db, session.Dialect(), table, "Dora"); err != nil {
				return err
			}
			if err := execCompatibilityUpsert(ctx, db, session.Dialect(), table, "Dora Updated"); err != nil {
				return err
			}
			var record CompatibilityRecord
			err := session.QueryOneStatement(
				ctx,
				compatibilityCaseStatement(namespace, quotedTable, "SelectOne"),
				orm.NamedArgs{"id": int64(4)},
				&record,
			)
			if err != nil {
				return err
			}
			if record.ID != 4 || record.Name != "Dora Updated" || record.Age != 18 {
				return fmt.Errorf("unexpected upsert record %#v", record)
			}
			return nil
		},
	}
}

func compatibilityGeneratedKeyCase(table string) DatabaseCase {
	return DatabaseCase{
		Name: "compatibility-generated-key",
		Run: func(ctx context.Context, session *orm.SQLSession, db *sql.DB) error {
			plan, err := orm.NewGeneratedKeyPlan(session.Dialect(), "id")
			if err != nil {
				return err
			}
			switch plan.Style {
			case orm.DialectGeneratedKeyNone:
				return nil
			case orm.DialectGeneratedKeyReturning:
				source, err := orm.NewInsertSQLBuilder().Into(table).Value("name", "Generated").Build()
				if err != nil {
					return err
				}
				compiled, err := orm.CompileSQLContext(ctx, source.SQL+" "+plan.SQLClause, source.Args, session.Dialect())
				if err != nil {
					return err
				}
				var id int64
				if err := db.QueryRowContext(ctx, compiled.SQL, compiled.Args...).Scan(&id); err != nil {
					return err
				}
				if id <= 0 {
					return fmt.Errorf("generated key id = %d", id)
				}
				return nil
			case orm.DialectGeneratedKeyLastInsertID:
				source, err := orm.NewInsertSQLBuilder().Into(table).Value("name", "Generated").Build()
				if err != nil {
					return err
				}
				compiled, err := orm.CompileSQLContext(ctx, source.SQL, source.Args, session.Dialect())
				if err != nil {
					return err
				}
				result, err := db.ExecContext(ctx, compiled.SQL, compiled.Args...)
				if err != nil {
					return err
				}
				id, err := result.LastInsertId()
				if err != nil {
					return err
				}
				if id <= 0 {
					return fmt.Errorf("generated key id = %d", id)
				}
				return nil
			case orm.DialectGeneratedKeyOutput:
				quotedTable, err := quoteCompatibilityTable(session.Dialect(), table)
				if err != nil {
					return err
				}
				sqlText := "INSERT INTO " + quotedTable + " (" + session.Dialect().QuoteIdent("name") + ") OUTPUT inserted." + session.Dialect().QuoteIdent("id") + " VALUES (#{name})"
				compiled, err := orm.CompileSQLContext(ctx, sqlText, orm.NamedArgs{"name": "Generated"}, session.Dialect())
				if err != nil {
					return err
				}
				var id int64
				if err := db.QueryRowContext(ctx, compiled.SQL, compiled.Args...).Scan(&id); err != nil {
					return err
				}
				if id <= 0 {
					return fmt.Errorf("generated key id = %d", id)
				}
				return nil
			case orm.DialectGeneratedKeyReturningInto:
				quotedTable, err := quoteCompatibilityTable(session.Dialect(), table)
				if err != nil {
					return err
				}
				var id int64
				sqlText := "INSERT INTO " + quotedTable + " (name) VALUES (#{name}) RETURNING " + session.Dialect().QuoteIdent("ID") + " INTO #{id}"
				compiled, err := orm.CompileSQLContext(ctx, sqlText, orm.NamedArgs{"name": "Generated", "id": sql.Out{Dest: &id}}, session.Dialect())
				if err != nil {
					return err
				}
				if _, err := db.ExecContext(ctx, compiled.SQL, compiled.Args...); err != nil {
					return err
				}
				if id <= 0 {
					return fmt.Errorf("generated key id = %d", id)
				}
				return nil
			default:
				return nil
			}
		},
	}
}

func compatibilityRowLockCase(table string) DatabaseCase {
	return DatabaseCase{
		Name: "compatibility-row-lock",
		Run: func(ctx context.Context, session *orm.SQLSession, db *sql.DB) error {
			capabilities := orm.DialectCapabilitiesOf(session.Dialect())
			if !capabilities.SupportsRowLock() {
				return nil
			}
			source, err := orm.NewSelectSQLBuilder().
				Select(compatibilityIDColumn(session.Dialect())).
				From(table).
				WhereEq(compatibilityIDColumn(session.Dialect()), int64(1)).
				ForUpdate(session.Dialect(), orm.RowLockOptions{SkipLocked: capabilities.SkipLocked}).
				Build()
			if err != nil {
				return err
			}
			compiled, err := orm.CompileSQLContext(ctx, source.SQL, source.Args, session.Dialect())
			if err != nil {
				return err
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()
			var id int64
			if err := tx.QueryRowContext(ctx, compiled.SQL, compiled.Args...).Scan(&id); err != nil {
				return err
			}
			if id != 1 {
				return fmt.Errorf("row lock id = %d", id)
			}
			return tx.Commit()
		},
	}
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
	mapper := compatibilityMapperMeta(namespace, table, "")
	for _, statement := range mapper.Statements {
		if statement.ID == id {
			return statement
		}
	}
	return orm.StatementMeta{}
}

func execCompatibilityUpsert(ctx context.Context, db *sql.DB, dialect orm.Dialect, table string, name string) error {
	columns := compatibilityUpsertColumns(dialect)
	source, err := orm.BuildUpsertSQL(dialect, orm.UpsertSpec{
		Table:           table,
		InsertColumns:   columns.insert,
		ConflictColumns: columns.conflict,
		UpdateColumns:   columns.update,
		Values:          columns.values(name),
	})
	if err != nil {
		return err
	}
	compiled, err := orm.CompileSQLContext(ctx, source.SQL, source.Args, dialect)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected <= 0 {
		return fmt.Errorf("upsert rows affected = %d", rowsAffected)
	}
	return nil
}

type compatibilityUpsertColumnSet struct {
	insert   []string
	conflict []string
	update   []string
}

func compatibilityUpsertColumns(dialect orm.Dialect) compatibilityUpsertColumnSet {
	if orm.DialectCapabilitiesOf(dialect).DBType == orm.DbTypeOracle {
		return compatibilityUpsertColumnSet{
			insert:   []string{"ID", "NAME", "AGE", "PROFILE", "CREATED_AT"},
			conflict: []string{"ID"},
			update:   []string{"NAME", "AGE", "PROFILE", "CREATED_AT"},
		}
	}
	return compatibilityUpsertColumnSet{
		insert:   []string{"id", "name", "age", "profile", "created_at"},
		conflict: []string{"id"},
		update:   []string{"name", "age", "profile", "created_at"},
	}
}

func compatibilityIDColumn(dialect orm.Dialect) string {
	if orm.DialectCapabilitiesOf(dialect).DBType == orm.DbTypeOracle {
		return "ID"
	}
	return "id"
}

func (c compatibilityUpsertColumnSet) values(name string) orm.NamedArgs {
	return orm.NamedArgs{
		c.insert[0]: int64(4),
		c.insert[1]: name,
		c.insert[2]: 18,
		c.insert[3]: `{"role":"guest","level":1}`,
		c.insert[4]: "2026-08-26T00:03:00Z",
	}
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

func compatibilityRelatedTable(table string, suffix string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", fmt.Errorf("goark-orm: compatibility table is required")
	}
	parts := strings.Split(table, ".")
	last := strings.TrimSpace(parts[len(parts)-1])
	if !validCompatibilityIdentifier(last) {
		return "", fmt.Errorf("goark-orm: invalid compatibility table identifier %q", table)
	}
	related := last + strings.TrimSpace(suffix)
	if !validCompatibilityIdentifier(related) {
		return "", fmt.Errorf("goark-orm: invalid compatibility related table identifier %q", related)
	}
	parts[len(parts)-1] = related
	return strings.Join(parts, "."), nil
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
