// Package ormtest 提供真实数据库兼容性测试辅助工具。
package ormtest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	orm "goark.dev/orm"
)

const (
	// DefaultEnvPrefix 是真实数据库测试默认使用的环境变量前缀。
	DefaultEnvPrefix = "GOARK_ORM_INTEGRATION"
	// DefaultSQLSeparator 是多条环境变量 SQL 的默认分隔符。
	DefaultSQLSeparator = "\n-- goark-orm statement --\n"
)

const defaultSuiteTimeout = 30 * time.Second

// DatabaseSuiteConfig 描述一次真实数据库兼容性套件运行。
type DatabaseSuiteConfig struct {
	DriverName     string
	DSN            string
	DBType         orm.DbType
	Dialect        orm.Dialect
	Registry       *orm.Registry
	SetupSQL       []string
	CleanupSQL     []string
	Cases          []DatabaseCase
	SessionOptions []orm.SQLSessionOption
	Timeout        time.Duration
	CleanupTimeout time.Duration
	MaxOpenConns   int
	MaxIdleConns   int
}

// DatabaseCase 描述一个可复用的数据库兼容性用例。
type DatabaseCase struct {
	Name string
	Run  func(context.Context, *orm.SQLSession, *sql.DB) error
}

// EnvSuiteOption 调整从环境变量加载套件的行为。
type EnvSuiteOption func(*envSuiteOptions)

type envSuiteOptions struct {
	prefix         string
	registry       *orm.Registry
	cases          []DatabaseCase
	sessionOptions []orm.SQLSessionOption
}

// WithEnvPrefix 指定环境变量前缀。
func WithEnvPrefix(prefix string) EnvSuiteOption {
	return func(options *envSuiteOptions) {
		options.prefix = strings.TrimSpace(prefix)
	}
}

// WithEnvRegistry 指定真实库套件使用的元数据注册表。
func WithEnvRegistry(registry *orm.Registry) EnvSuiteOption {
	return func(options *envSuiteOptions) {
		options.registry = registry
	}
}

// WithEnvCases 追加环境变量套件要执行的用例。
func WithEnvCases(cases ...DatabaseCase) EnvSuiteOption {
	return func(options *envSuiteOptions) {
		options.cases = append(options.cases, cases...)
	}
}

// WithEnvSessionOptions 追加 SQLSession 配置。
func WithEnvSessionOptions(sessionOptions ...orm.SQLSessionOption) EnvSuiteOption {
	return func(options *envSuiteOptions) {
		options.sessionOptions = append(options.sessionOptions, sessionOptions...)
	}
}

// RunDatabaseSuiteFromEnv 从环境变量读取配置并运行真实数据库套件。
func RunDatabaseSuiteFromEnv(t *testing.T, options ...EnvSuiteOption) {
	t.Helper()
	opts := envSuiteOptions{prefix: DefaultEnvPrefix}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	config, configured, err := LoadDatabaseSuiteConfigFromEnv(opts.prefix)
	if err != nil {
		t.Fatalf("load database suite config failed: %v", err)
	}
	if !configured {
		t.Skipf("set %s_DRIVER and %s_DSN to run database integration suite", envPrefix(opts.prefix), envPrefix(opts.prefix))
	}
	config.Registry = opts.registry
	config.Cases = append(config.Cases, opts.cases...)
	config.SessionOptions = append(config.SessionOptions, opts.sessionOptions...)
	RunDatabaseSuite(t, config)
}

// LoadDatabaseSuiteConfigFromEnv 从环境变量加载真实数据库套件配置。
func LoadDatabaseSuiteConfigFromEnv(prefix string) (DatabaseSuiteConfig, bool, error) {
	prefix = envPrefix(prefix)
	driverName := strings.TrimSpace(os.Getenv(prefix + "_DRIVER"))
	dsn := strings.TrimSpace(os.Getenv(prefix + "_DSN"))
	if driverName == "" || dsn == "" {
		return DatabaseSuiteConfig{}, false, nil
	}
	dbType, err := orm.ParseDbType(os.Getenv(prefix + "_DBTYPE"))
	if err != nil {
		return DatabaseSuiteConfig{}, true, err
	}
	separator := os.Getenv(prefix + "_SQL_SEPARATOR")
	setupSQL, err := ParseSQLList(os.Getenv(prefix+"_SETUP_SQL"), separator)
	if err != nil {
		return DatabaseSuiteConfig{}, true, fmt.Errorf("parse %s_SETUP_SQL failed: %w", prefix, err)
	}
	cleanupSQL, err := ParseSQLList(os.Getenv(prefix+"_CLEANUP_SQL"), separator)
	if err != nil {
		return DatabaseSuiteConfig{}, true, fmt.Errorf("parse %s_CLEANUP_SQL failed: %w", prefix, err)
	}
	timeout, err := parseEnvDuration(os.Getenv(prefix+"_TIMEOUT"), defaultSuiteTimeout)
	if err != nil {
		return DatabaseSuiteConfig{}, true, fmt.Errorf("parse %s_TIMEOUT failed: %w", prefix, err)
	}
	cleanupTimeout, err := parseEnvDuration(os.Getenv(prefix+"_CLEANUP_TIMEOUT"), timeout)
	if err != nil {
		return DatabaseSuiteConfig{}, true, fmt.Errorf("parse %s_CLEANUP_TIMEOUT failed: %w", prefix, err)
	}
	maxOpenConns, err := parseEnvInt(os.Getenv(prefix + "_MAX_OPEN_CONNS"))
	if err != nil {
		return DatabaseSuiteConfig{}, true, fmt.Errorf("parse %s_MAX_OPEN_CONNS failed: %w", prefix, err)
	}
	maxIdleConns, err := parseEnvInt(os.Getenv(prefix + "_MAX_IDLE_CONNS"))
	if err != nil {
		return DatabaseSuiteConfig{}, true, fmt.Errorf("parse %s_MAX_IDLE_CONNS failed: %w", prefix, err)
	}
	return DatabaseSuiteConfig{
		DriverName:     driverName,
		DSN:            dsn,
		DBType:         dbType,
		SetupSQL:       setupSQL,
		CleanupSQL:     cleanupSQL,
		Timeout:        timeout,
		CleanupTimeout: cleanupTimeout,
		MaxOpenConns:   maxOpenConns,
		MaxIdleConns:   maxIdleConns,
	}, true, nil
}

// ParseSQLList 解析环境变量中的 SQL 列表，支持 JSON 字符串数组或分隔符文本。
func ParseSQLList(value string, separator string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if strings.HasPrefix(value, "[") {
		var items []string
		if err := json.Unmarshal([]byte(value), &items); err != nil {
			return nil, err
		}
		return compactSQLList(items), nil
	}
	if separator == "" {
		separator = DefaultSQLSeparator
	}
	return compactSQLList(strings.Split(value, separator)), nil
}

// RunDatabaseSuite 执行真实数据库兼容性套件。
func RunDatabaseSuite(t *testing.T, config DatabaseSuiteConfig) {
	t.Helper()
	normalized, configured, err := normalizeDatabaseSuiteConfig(config)
	if err != nil {
		t.Fatalf("normalize database suite config failed: %v", err)
	}
	if !configured {
		t.Skip("database suite driver name or DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), normalized.Timeout)
	defer cancel()
	db, err := sql.Open(normalized.DriverName, normalized.DSN)
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	if normalized.MaxOpenConns > 0 {
		db.SetMaxOpenConns(normalized.MaxOpenConns)
	}
	if normalized.MaxIdleConns > 0 {
		db.SetMaxIdleConns(normalized.MaxIdleConns)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database failed: %v", err)
	}
	if err := execRawSQLList(ctx, db, normalized.SetupSQL); err != nil {
		t.Fatalf("setup database failed: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), normalized.CleanupTimeout)
		defer cleanupCancel()
		if err := execRawSQLList(cleanupCtx, db, normalized.CleanupSQL); err != nil {
			t.Errorf("cleanup database failed: %v", err)
		}
	})
	session, err := orm.NewSQLSession(normalized.Registry, db, normalized.Dialect, normalized.SessionOptions...)
	if err != nil {
		t.Fatalf("create SQL session failed: %v", err)
	}
	for _, testCase := range normalized.Cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			if testCase.Run == nil {
				t.Fatalf("database case %s run function is nil", testCase.Name)
			}
			caseCtx, caseCancel := context.WithTimeout(context.Background(), normalized.Timeout)
			defer caseCancel()
			if err := testCase.Run(caseCtx, session, db); err != nil {
				t.Fatalf("database case %s failed: %v", testCase.Name, err)
			}
		})
	}
}

// QueryStatementCase 创建多行查询用例。
func QueryStatementCase[T any](name string, statement orm.StatementMeta, args orm.NamedArgs, check func([]T) error) DatabaseCase {
	return DatabaseCase{
		Name: name,
		Run: func(ctx context.Context, session *orm.SQLSession, _ *sql.DB) error {
			var out []T
			if err := session.QueryStatement(ctx, statement, cloneNamedArgs(args), &out); err != nil {
				return err
			}
			if check != nil {
				return check(out)
			}
			return nil
		},
	}
}

// QueryOneStatementCase 创建单行查询用例。
func QueryOneStatementCase[T any](name string, statement orm.StatementMeta, args orm.NamedArgs, check func(T) error) DatabaseCase {
	return DatabaseCase{
		Name: name,
		Run: func(ctx context.Context, session *orm.SQLSession, _ *sql.DB) error {
			var out T
			if err := session.QueryOneStatement(ctx, statement, cloneNamedArgs(args), &out); err != nil {
				return err
			}
			if check != nil {
				return check(out)
			}
			return nil
		},
	}
}

// ExecStatementCase 创建写语句用例。
func ExecStatementCase(name string, statement orm.StatementMeta, args orm.NamedArgs, check func(orm.Result) error) DatabaseCase {
	return DatabaseCase{
		Name: name,
		Run: func(ctx context.Context, session *orm.SQLSession, _ *sql.DB) error {
			result, err := session.ExecStatement(ctx, statement, cloneNamedArgs(args))
			if err != nil {
				return err
			}
			if check != nil {
				return check(result)
			}
			return nil
		},
	}
}

// PageStatementCase 创建分页查询用例。
func PageStatementCase[T any](name string, statement orm.StatementMeta, args orm.NamedArgs, page orm.PageRequest, check func(orm.Page[T]) error) DatabaseCase {
	return DatabaseCase{
		Name: name,
		Run: func(ctx context.Context, session *orm.SQLSession, _ *sql.DB) error {
			var records []T
			result, err := session.QueryPageStatement(ctx, statement, cloneNamedArgs(args), page, &records)
			if err != nil {
				return err
			}
			out := orm.Page[T]{
				Records: records,
				Total:   result.Total,
				Size:    result.Size,
				Current: result.Current,
				Pages:   result.Pages,
			}
			if check != nil {
				return check(out)
			}
			return nil
		},
	}
}

// CallStatementCase 创建存储过程或 callable statement 用例。
func CallStatementCase(name string, statement orm.StatementMeta, args orm.NamedArgs, resultSets []any, check func(orm.CallResult) error) DatabaseCase {
	return DatabaseCase{
		Name: name,
		Run: func(ctx context.Context, session *orm.SQLSession, _ *sql.DB) error {
			result, err := session.CallStatement(ctx, statement, cloneNamedArgs(args), resultSets...)
			if err != nil {
				return err
			}
			if check != nil {
				return check(result)
			}
			return nil
		},
	}
}

func normalizeDatabaseSuiteConfig(config DatabaseSuiteConfig) (DatabaseSuiteConfig, bool, error) {
	config.DriverName = strings.TrimSpace(config.DriverName)
	config.DSN = strings.TrimSpace(config.DSN)
	if config.DriverName == "" || config.DSN == "" {
		return DatabaseSuiteConfig{}, false, nil
	}
	if config.Dialect == nil {
		dialect, err := orm.NewDialect(config.DBType)
		if err != nil {
			return DatabaseSuiteConfig{}, true, err
		}
		config.Dialect = dialect
	}
	if config.Registry == nil {
		config.Registry = orm.NewRegistry()
	}
	config.SetupSQL = compactSQLList(config.SetupSQL)
	config.CleanupSQL = compactSQLList(config.CleanupSQL)
	if config.Timeout <= 0 {
		config.Timeout = defaultSuiteTimeout
	}
	if config.CleanupTimeout <= 0 {
		config.CleanupTimeout = config.Timeout
	}
	if len(config.Cases) == 0 {
		config.Cases = []DatabaseCase{PingCase()}
	}
	for index := range config.Cases {
		config.Cases[index].Name = strings.TrimSpace(config.Cases[index].Name)
		if config.Cases[index].Name == "" {
			config.Cases[index].Name = fmt.Sprintf("case-%d", index+1)
		}
	}
	return config, true, nil
}

// PingCase 创建显式 ping 用例，便于在子测试输出中记录基础连通性。
func PingCase() DatabaseCase {
	return DatabaseCase{
		Name: "ping",
		Run: func(ctx context.Context, _ *orm.SQLSession, db *sql.DB) error {
			return db.PingContext(ctx)
		},
	}
}

func execRawSQLList(ctx context.Context, db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func compactSQLList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func cloneNamedArgs(args orm.NamedArgs) orm.NamedArgs {
	if len(args) == 0 {
		return nil
	}
	out := make(orm.NamedArgs, len(args))
	for key, value := range args {
		out[key] = value
	}
	return out
}

func envPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return DefaultEnvPrefix
	}
	return prefix
}

func parseEnvDuration(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration, nil
	}
	seconds, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		return 0, err
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseEnvInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, fmt.Errorf("value must be non-negative")
	}
	return parsed, nil
}
