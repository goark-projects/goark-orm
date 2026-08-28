package ormtest

import (
	"context"
	"fmt"
	"strings"

	orm "goark.dev/orm"
)

const (
	// DefaultBenchmarkTable 是标准真实库 benchmark 使用的默认临时表名。
	DefaultBenchmarkTable = "goark_orm_bench_users"

	defaultBenchmarkNamespace = "goark.ormtest.BenchmarkMapper"
)

type benchmarkSuiteOptions struct {
	table     string
	namespace string
}

// BenchmarkSuiteOption 调整标准真实库 benchmark。
type BenchmarkSuiteOption func(*benchmarkSuiteOptions)

// WithBenchmarkTable 指定标准 benchmark 使用的临时表名。
func WithBenchmarkTable(table string) BenchmarkSuiteOption {
	return func(options *benchmarkSuiteOptions) {
		options.table = strings.TrimSpace(table)
	}
}

// WithBenchmarkNamespace 指定标准 benchmark 使用的 Mapper namespace。
func WithBenchmarkNamespace(namespace string) BenchmarkSuiteOption {
	return func(options *benchmarkSuiteOptions) {
		options.namespace = strings.TrimSpace(namespace)
	}
}

// SupportedBenchmarkDBTypes 返回标准真实库 benchmark 当前支持的数据库类型。
func SupportedBenchmarkDBTypes() []orm.DbType {
	return SupportedCompatibilityDBTypes()
}

// IsBenchmarkDBTypeSupported 判断标准真实库 benchmark 是否支持指定数据库类型。
func IsBenchmarkDBTypeSupported(dbType orm.DbType) bool {
	return IsCompatibilityDBTypeSupported(dbType)
}

// NewBenchmarkSuiteConfig 创建覆盖真实查询、生成式扫描、JSON、批写和 UPSERT 的标准 benchmark。
func NewBenchmarkSuiteConfig(dbType orm.DbType, options ...BenchmarkSuiteOption) (DatabaseBenchmarkConfig, error) {
	opts := normalizeBenchmarkSuiteOptions(options...)
	if !IsBenchmarkDBTypeSupported(dbType) {
		return DatabaseBenchmarkConfig{}, unsupportedBenchmarkDBTypeError(dbType)
	}
	dialect, err := orm.NewDialect(dbType)
	if err != nil {
		return DatabaseBenchmarkConfig{}, err
	}
	quotedTable, err := quoteCompatibilityTable(dialect, opts.table)
	if err != nil {
		return DatabaseBenchmarkConfig{}, err
	}
	setupSQL, cleanupSQL, err := benchmarkDDL(dbType, quotedTable)
	if err != nil {
		return DatabaseBenchmarkConfig{}, err
	}
	registry, err := newBenchmarkRegistry(opts.namespace, quotedTable, dbType)
	if err != nil {
		return DatabaseBenchmarkConfig{}, err
	}
	configuration := orm.DefaultConfiguration()
	configuration.Dialect = dialect
	configuration.DefaultExecutorType = orm.ExecutorTypeReuse
	return DatabaseBenchmarkConfig{
		DBType:         dbType,
		Dialect:        dialect,
		Registry:       registry,
		SetupSQL:       setupSQL,
		CleanupSQL:     cleanupSQL,
		Cases:          benchmarkCases(opts.namespace, opts.table, quotedTable, dbType),
		SessionOptions: []orm.SQLSessionOption{orm.WithConfiguration(configuration), orm.WithLocalCache(false)},
		Timeout:        defaultBenchmarkTimeout,
		CleanupTimeout: defaultSuiteTimeout,
		MaxOpenConns:   16,
		MaxIdleConns:   16,
	}, nil
}

func normalizeBenchmarkSuiteOptions(options ...BenchmarkSuiteOption) benchmarkSuiteOptions {
	opts := benchmarkSuiteOptions{
		table:     DefaultBenchmarkTable,
		namespace: defaultBenchmarkNamespace,
	}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if opts.table == "" {
		opts.table = DefaultBenchmarkTable
	}
	if opts.namespace == "" {
		opts.namespace = defaultBenchmarkNamespace
	}
	return opts
}

func unsupportedBenchmarkDBTypeError(dbType orm.DbType) error {
	return fmt.Errorf("goark-orm: standard benchmark suite supports postgres, mysql, mariadb, sqlite, sqlserver and oracle, got %q", dbType)
}

func newBenchmarkRegistry(namespace string, table string, dbType orm.DbType) (*orm.Registry, error) {
	registry := orm.NewRegistry()
	if err := registry.RegisterTypeHandler(compatibilityJSONHandler, compatibilityJSONTypeHandler{}); err != nil {
		return nil, err
	}
	if err := registry.RegisterEntity(compatibilityEntityMeta(table)); err != nil {
		return nil, err
	}
	if err := registry.RegisterMapper(benchmarkMapperMeta(namespace, table, dbType)); err != nil {
		return nil, err
	}
	if err := registry.RegisterRowScanner("CompatibilityRecord", benchmarkCompatibilityRowScanner()); err != nil {
		return nil, err
	}
	if err := registry.Validate(); err != nil {
		return nil, err
	}
	return registry, nil
}

func benchmarkMapperMeta(namespace string, table string, dbType orm.DbType) orm.MapperMeta {
	return orm.MapperMeta{
		TypeName:   "BenchmarkMapper",
		Namespace:  namespace,
		ResultMaps: []orm.ResultMapMeta{compatibilityResultMap()},
		Statements: []orm.StatementMeta{
			benchmarkStatement(namespace, "Insert", orm.StatementCommandInsert, "insert into "+table+" (id, name, age, profile, created_at) values (#{record.ID}, #{record.Name}, #{record.Age}, #{record.Profile}, #{record.CreatedAt})", "", ""),
			benchmarkStatement(namespace, "SelectGenerated", orm.StatementCommandSelect, "select id, name, age, profile, created_at from "+table+" where id = #{id}", "", "CompatibilityRecord"),
			benchmarkStatement(namespace, "SelectResultMap", orm.StatementCommandSelect, "select id, name, age, profile, created_at from "+table+" where id = #{id}", defaultCompatibilityResultMapID, ""),
			benchmarkStatement(namespace, "Upsert", orm.StatementCommandInsert, benchmarkUpsertSQL(table, dbType), "", ""),
		},
	}
}

func benchmarkStatement(namespace string, id string, command orm.StatementCommand, sqlText string, resultMap string, resultType string) orm.StatementMeta {
	return orm.StatementMeta{
		ID:            id,
		Namespace:     namespace,
		FullName:      namespace + "." + id,
		Command:       command,
		Source:        orm.StatementSourceAnnotation,
		SQL:           sqlText,
		ResultMap:     resultMap,
		ResultType:    resultType,
		ParameterType: "CompatibilityRecord",
	}
}

func benchmarkCompatibilityRowScanner() orm.TypeHandlerRowScanner {
	return orm.TypeHandlerRowScannerFunc(func(ctx context.Context, columns []string, row orm.RowScannerRow, dest any, handlers orm.RowScannerTypeHandlers) error {
		record, ok := dest.(*CompatibilityRecord)
		if !ok {
			return fmt.Errorf("goark-orm: benchmark row scanner destination must be *CompatibilityRecord")
		}
		var profile any
		var discard any
		var targetStack [8]any
		targets := targetStack[:]
		if len(columns) > len(targets) {
			targets = make([]any, len(columns))
		} else {
			targets = targets[:len(columns)]
		}
		for index, column := range columns {
			switch benchmarkColumnKey(column) {
			case "id":
				targets[index] = &record.ID
			case "name":
				targets[index] = &record.Name
			case "age":
				targets[index] = &record.Age
			case "profile":
				targets[index] = &profile
			case "createdat":
				targets[index] = &record.CreatedAt
			default:
				targets[index] = &discard
			}
		}
		if err := row.Scan(targets...); err != nil {
			return err
		}
		if profile == nil {
			return nil
		}
		handler, ok := handlers.TypeHandler(compatibilityJSONHandler)
		if !ok {
			return fmt.Errorf("goark-orm: benchmark json type-handler is missing")
		}
		return handler.FromDB(ctx, profile, &record.Profile)
	})
}

func benchmarkColumnKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch r {
		case '_', '-', '.', ' ':
			continue
		default:
			builder.WriteRune(r)
		}
	}
	return strings.ToLower(builder.String())
}
