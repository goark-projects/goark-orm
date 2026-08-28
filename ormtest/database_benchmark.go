package ormtest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	orm "goark.dev/orm"
)

const (
	// DefaultBenchmarkEnvPrefix 是真实数据库 benchmark 默认环境变量前缀。
	DefaultBenchmarkEnvPrefix = "GOARK_ORM_BENCH"

	defaultBenchmarkTimeout = 2 * time.Minute
)

// DatabaseBenchmarkConfig 描述一次真实数据库 benchmark 运行。
type DatabaseBenchmarkConfig struct {
	DriverName     string
	DSN            string
	DBType         orm.DbType
	Dialect        orm.Dialect
	Registry       *orm.Registry
	SetupSQL       []string
	CleanupSQL     []string
	Cases          []DatabaseBenchmarkCase
	SessionOptions []orm.SQLSessionOption
	Timeout        time.Duration
	CleanupTimeout time.Duration
	MaxOpenConns   int
	MaxIdleConns   int
}

// DatabaseBenchmarkCase 描述一个真实数据库 benchmark 用例。
type DatabaseBenchmarkCase struct {
	Name string
	Run  func(DatabaseBenchmarkScope) error
}

// DatabaseBenchmarkScope 暴露 benchmark 用例需要的运行期对象。
type DatabaseBenchmarkScope struct {
	Context context.Context
	B       *testing.B
	Session *orm.SQLSession
	DB      *sql.DB
}

// RunDatabaseBenchmarkFromEnv 从环境变量读取配置并运行真实数据库 benchmark。
func RunDatabaseBenchmarkFromEnv(b *testing.B, cases ...DatabaseBenchmarkCase) {
	b.Helper()
	config, configured, err := LoadDatabaseBenchmarkConfigFromEnv("")
	if err != nil {
		b.Fatalf("load database benchmark config failed: %v", err)
	}
	if !configured {
		prefix := envPrefix(DefaultBenchmarkEnvPrefix)
		b.Skipf("set %s_DRIVER, %s_DSN and %s_DBTYPE to run database benchmark", prefix, prefix, prefix)
	}
	config.Cases = append(config.Cases, cases...)
	RunDatabaseBenchmark(b, config)
}

// LoadDatabaseBenchmarkConfigFromEnv 从环境变量加载真实数据库 benchmark 配置。
func LoadDatabaseBenchmarkConfigFromEnv(prefix string) (DatabaseBenchmarkConfig, bool, error) {
	prefix = envPrefix(firstNonEmpty(prefix, DefaultBenchmarkEnvPrefix))
	driverName := strings.TrimSpace(os.Getenv(prefix + "_DRIVER"))
	dsn := strings.TrimSpace(os.Getenv(prefix + "_DSN"))
	if driverName == "" || dsn == "" {
		return DatabaseBenchmarkConfig{}, false, nil
	}
	dbType, err := orm.ParseDbType(os.Getenv(prefix + "_DBTYPE"))
	if err != nil {
		return DatabaseBenchmarkConfig{}, true, err
	}
	separator := os.Getenv(prefix + "_SQL_SEPARATOR")
	setupSQL, err := ParseSQLList(os.Getenv(prefix+"_SETUP_SQL"), separator)
	if err != nil {
		return DatabaseBenchmarkConfig{}, true, fmt.Errorf("parse %s_SETUP_SQL failed: %w", prefix, err)
	}
	cleanupSQL, err := ParseSQLList(os.Getenv(prefix+"_CLEANUP_SQL"), separator)
	if err != nil {
		return DatabaseBenchmarkConfig{}, true, fmt.Errorf("parse %s_CLEANUP_SQL failed: %w", prefix, err)
	}
	timeout, err := parseEnvDuration(os.Getenv(prefix+"_TIMEOUT"), defaultBenchmarkTimeout)
	if err != nil {
		return DatabaseBenchmarkConfig{}, true, fmt.Errorf("parse %s_TIMEOUT failed: %w", prefix, err)
	}
	cleanupTimeout, err := parseEnvDuration(os.Getenv(prefix+"_CLEANUP_TIMEOUT"), timeout)
	if err != nil {
		return DatabaseBenchmarkConfig{}, true, fmt.Errorf("parse %s_CLEANUP_TIMEOUT failed: %w", prefix, err)
	}
	maxOpenConns, err := parseEnvInt(os.Getenv(prefix + "_MAX_OPEN_CONNS"))
	if err != nil {
		return DatabaseBenchmarkConfig{}, true, fmt.Errorf("parse %s_MAX_OPEN_CONNS failed: %w", prefix, err)
	}
	maxIdleConns, err := parseEnvInt(os.Getenv(prefix + "_MAX_IDLE_CONNS"))
	if err != nil {
		return DatabaseBenchmarkConfig{}, true, fmt.Errorf("parse %s_MAX_IDLE_CONNS failed: %w", prefix, err)
	}
	return DatabaseBenchmarkConfig{
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

// RunDatabaseBenchmark 执行真实数据库 benchmark。
func RunDatabaseBenchmark(b *testing.B, config DatabaseBenchmarkConfig) {
	b.Helper()
	normalized, configured, err := normalizeDatabaseBenchmarkConfig(config)
	if err != nil {
		b.Fatalf("normalize database benchmark config failed: %v", err)
	}
	if !configured {
		b.Skip("database benchmark driver name or DSN is not configured")
	}
	db, session := openBenchmarkSession(b, normalized)
	for _, benchmarkCase := range normalized.Cases {
		benchmarkCase := benchmarkCase
		b.Run(benchmarkCase.Name, func(b *testing.B) {
			b.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), normalized.Timeout)
			defer cancel()
			scope := DatabaseBenchmarkScope{Context: ctx, B: b, Session: session, DB: db}
			if err := benchmarkCase.Run(scope); err != nil {
				b.Fatalf("database benchmark %s failed: %v", benchmarkCase.Name, err)
			}
		})
	}
}

func normalizeDatabaseBenchmarkConfig(config DatabaseBenchmarkConfig) (DatabaseBenchmarkConfig, bool, error) {
	config.DriverName = strings.TrimSpace(config.DriverName)
	config.DSN = strings.TrimSpace(config.DSN)
	if config.DriverName == "" || config.DSN == "" {
		return DatabaseBenchmarkConfig{}, false, nil
	}
	if config.Dialect == nil {
		dialect, err := orm.NewDialect(config.DBType)
		if err != nil {
			return DatabaseBenchmarkConfig{}, true, err
		}
		config.Dialect = dialect
	}
	if config.Registry == nil {
		config.Registry = orm.NewRegistry()
	}
	config.SetupSQL = compactSQLList(config.SetupSQL)
	config.CleanupSQL = compactSQLList(config.CleanupSQL)
	if config.Timeout <= 0 {
		config.Timeout = defaultBenchmarkTimeout
	}
	if config.CleanupTimeout <= 0 {
		config.CleanupTimeout = config.Timeout
	}
	if len(config.Cases) == 0 {
		config.Cases = []DatabaseBenchmarkCase{PingBenchmarkCase()}
	}
	for index := range config.Cases {
		config.Cases[index].Name = strings.TrimSpace(config.Cases[index].Name)
		if config.Cases[index].Name == "" {
			config.Cases[index].Name = fmt.Sprintf("benchmark-%d", index+1)
		}
		if config.Cases[index].Run == nil {
			return DatabaseBenchmarkConfig{}, true, fmt.Errorf("database benchmark case %s run function is nil", config.Cases[index].Name)
		}
	}
	return config, true, nil
}

func openBenchmarkSession(b *testing.B, config DatabaseBenchmarkConfig) (*sql.DB, *orm.SQLSession) {
	b.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	db, err := sql.Open(config.DriverName, config.DSN)
	if err != nil {
		b.Fatalf("open database failed: %v", err)
	}
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	b.Cleanup(func() {
		_ = db.Close()
	})
	if err := db.PingContext(ctx); err != nil {
		b.Fatalf("ping database failed: %v", err)
	}
	if err := execRawSQLList(ctx, db, config.SetupSQL); err != nil {
		b.Fatalf("setup benchmark database failed: %v", err)
	}
	b.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), config.CleanupTimeout)
		defer cleanupCancel()
		if err := execRawSQLList(cleanupCtx, db, config.CleanupSQL); err != nil {
			b.Errorf("cleanup benchmark database failed: %v", err)
		}
	})
	session, err := orm.NewSQLSession(config.Registry, db, config.Dialect, config.SessionOptions...)
	if err != nil {
		b.Fatalf("create benchmark SQL session failed: %v", err)
	}
	return db, session
}

// PingBenchmarkCase 创建最小 ping benchmark。
func PingBenchmarkCase() DatabaseBenchmarkCase {
	return DatabaseBenchmarkCase{
		Name: "ping",
		Run: func(scope DatabaseBenchmarkScope) error {
			scope.B.ReportAllocs()
			scope.B.ResetTimer()
			for i := 0; i < scope.B.N; i++ {
				if err := scope.DB.PingContext(scope.Context); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
