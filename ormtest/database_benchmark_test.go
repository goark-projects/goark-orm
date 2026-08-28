package ormtest

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	orm "goark.dev/orm"
)

func TestLoadDatabaseBenchmarkConfigFromEnv_whenConfigured_shouldParseOptions(t *testing.T) {
	t.Setenv("GOARK_ORM_BENCH_DRIVER", "fake")
	t.Setenv("GOARK_ORM_BENCH_DSN", "memory")
	t.Setenv("GOARK_ORM_BENCH_DBTYPE", "postgres")
	t.Setenv("GOARK_ORM_BENCH_SETUP_SQL", `["select 1", "select 2"]`)
	t.Setenv("GOARK_ORM_BENCH_TIMEOUT", "3s")
	t.Setenv("GOARK_ORM_BENCH_MAX_IDLE_CONNS", "2")

	config, configured, err := LoadDatabaseBenchmarkConfigFromEnv("")
	if err != nil {
		t.Fatalf("load benchmark config failed: %v", err)
	}
	if !configured {
		t.Fatalf("expected configured benchmark")
	}
	if config.DriverName != "fake" || config.DSN != "memory" || config.DBType != orm.DbTypePostgres {
		t.Fatalf("unexpected config %#v", config)
	}
	if len(config.SetupSQL) != 2 || config.SetupSQL[1] != "select 2" {
		t.Fatalf("unexpected setup SQL %#v", config.SetupSQL)
	}
	if config.Timeout != 3*time.Second || config.MaxIdleConns != 2 {
		t.Fatalf("unexpected timeout or max idle conns: %s %d", config.Timeout, config.MaxIdleConns)
	}
}

func TestNewBenchmarkSuiteConfig_whenPostgres_shouldCreateReusableMatrix(t *testing.T) {
	config, err := NewBenchmarkSuiteConfig(orm.DbTypePostgres, WithBenchmarkTable("goark_orm_bench_unit"))
	if err != nil {
		t.Fatalf("new benchmark suite config failed: %v", err)
	}
	if config.Dialect == nil || config.Registry == nil {
		t.Fatalf("expected dialect and registry")
	}
	if len(config.SetupSQL) == 0 || len(config.CleanupSQL) == 0 || len(config.Cases) != 6 {
		t.Fatalf("unexpected benchmark config: setup=%d cleanup=%d cases=%d", len(config.SetupSQL), len(config.CleanupSQL), len(config.Cases))
	}
	if _, ok := config.Registry.Statement(defaultBenchmarkNamespace + ".SelectGenerated"); !ok {
		t.Fatalf("expected generated scan statement")
	}
	if !strings.Contains(config.SetupSQL[1], "JSONB") {
		t.Fatalf("expected postgres native JSON DDL, got %q", config.SetupSQL[1])
	}
}

func TestRunDatabaseBenchmark_whenConfigured_shouldRunCaseAndCleanup(t *testing.T) {
	driverName, dsn, state := registerSuiteTestDriver(t)
	var runs atomic.Int64

	result := testing.Benchmark(func(b *testing.B) {
		RunDatabaseBenchmark(b, DatabaseBenchmarkConfig{
			DriverName: driverName,
			DSN:        dsn,
			Dialect:    orm.NewQuestionDialect(),
			SetupSQL:   []string{"setup benchmark"},
			CleanupSQL: []string{"cleanup benchmark"},
			Timeout:    time.Second,
			Cases: []DatabaseBenchmarkCase{
				{
					Name: "exec",
					Run: func(scope DatabaseBenchmarkScope) error {
						statement := orm.StatementMeta{
							ID:        "Exec",
							Namespace: "bench.Mapper",
							FullName:  "bench.Mapper.Exec",
							Command:   orm.StatementCommandInsert,
							SQL:       "insert into users(id, name) values (#{id}, #{name})",
						}
						scope.B.ResetTimer()
						for i := 0; i < scope.B.N; i++ {
							_, err := scope.Session.ExecStatement(scope.Context, statement, orm.NamedArgs{"id": int64(i + 1), "name": "Alice"})
							if err != nil {
								return err
							}
							runs.Add(1)
						}
						return nil
					},
				},
			},
		})
	})

	if result.N <= 0 || runs.Load() <= 0 {
		t.Fatalf("expected benchmark to run, result=%#v runs=%d", result, runs.Load())
	}
	execs := state.execStatements()
	joined := strings.Join(execs, "|")
	if !strings.Contains(joined, "setup benchmark") || !strings.Contains(joined, "cleanup benchmark") {
		t.Fatalf("expected setup and cleanup SQL, got %#v", execs)
	}
}

func TestBenchmarkCompatibilityRowScanner_whenColumnsAreDriverSpecific_shouldScanProfile(t *testing.T) {
	scanner := benchmarkCompatibilityRowScanner()
	row := benchmarkScannerRow{values: []any{int64(1), "Alice", int64(31), []byte(`{"role":"admin","level":7}`), "2026-08-26T00:00:00Z"}}
	var record CompatibilityRecord

	err := scanner.ScanRowWithTypeHandlers(t.Context(), []string{"ID", "NAME", "AGE", "PROFILE", "CREATED_AT"}, row, &record, rowScannerHandlers{compatibilityJSONHandler: compatibilityJSONTypeHandler{}})
	if err != nil {
		t.Fatalf("scan row failed: %v", err)
	}
	if record.ID != 1 || record.Age != 31 || record.Profile.Role != "admin" || record.CreatedAt == "" {
		t.Fatalf("unexpected record %#v", record)
	}
}

type benchmarkScannerRow struct {
	values []any
}

func (r benchmarkScannerRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return fmt.Errorf("destination length = %d, want %d", len(dest), len(r.values))
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *int64:
			*target = value.(int64)
		case *int:
			*target = int(value.(int64))
		case *string:
			*target = value.(string)
		case *any:
			*target = value
		default:
			return fmt.Errorf("unsupported target %T", target)
		}
	}
	return nil
}

type rowScannerHandlers map[string]orm.TypeHandler

func (h rowScannerHandlers) TypeHandler(name string) (orm.TypeHandler, bool) {
	handler, ok := h[name]
	return handler, ok
}
