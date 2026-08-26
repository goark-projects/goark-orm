package ormtest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	orm "goark.dev/orm"
)

var suiteTestStates sync.Map

func TestParseSQLList_whenJSONProvided_shouldParseAndTrim(t *testing.T) {
	items, err := ParseSQLList(`[" create table t(id int) ", "", "drop table t"]`, "")
	if err != nil {
		t.Fatalf("parse SQL list failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected SQL count %d", len(items))
	}
	if items[0] != "create table t(id int)" || items[1] != "drop table t" {
		t.Fatalf("unexpected SQL list %#v", items)
	}
}

func TestLoadDatabaseSuiteConfigFromEnv_whenConfigured_shouldParseOptions(t *testing.T) {
	t.Setenv("GOARK_ORM_INTEGRATION_DRIVER", "fake")
	t.Setenv("GOARK_ORM_INTEGRATION_DSN", "memory")
	t.Setenv("GOARK_ORM_INTEGRATION_DBTYPE", "postgres")
	t.Setenv("GOARK_ORM_INTEGRATION_SETUP_SQL", "select 1||select 2")
	t.Setenv("GOARK_ORM_INTEGRATION_SQL_SEPARATOR", "||")
	t.Setenv("GOARK_ORM_INTEGRATION_TIMEOUT", "2s")
	t.Setenv("GOARK_ORM_INTEGRATION_MAX_OPEN_CONNS", "3")

	config, configured, err := LoadDatabaseSuiteConfigFromEnv("")
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if !configured {
		t.Fatalf("expected configured suite")
	}
	if config.DriverName != "fake" || config.DSN != "memory" || config.DBType != orm.DbTypePostgres {
		t.Fatalf("unexpected config %#v", config)
	}
	if len(config.SetupSQL) != 2 || config.SetupSQL[1] != "select 2" {
		t.Fatalf("unexpected setup SQL %#v", config.SetupSQL)
	}
	if config.Timeout != 2*time.Second || config.MaxOpenConns != 3 {
		t.Fatalf("unexpected timeout or max open conns: %s %d", config.Timeout, config.MaxOpenConns)
	}
}

func TestRunDatabaseSuite_whenConfigured_shouldRunSetupCaseAndCleanup(t *testing.T) {
	driverName, dsn, state := registerSuiteTestDriver(t)
	statement := orm.StatementMeta{
		ID:        "UpdateName",
		Namespace: "suite.UserMapper",
		FullName:  "suite.UserMapper.UpdateName",
		Command:   orm.StatementCommandUpdate,
		SQL:       "update users set name = #{name} where id = #{id}",
	}

	t.Run("suite", func(t *testing.T) {
		RunDatabaseSuite(t, DatabaseSuiteConfig{
			DriverName: driverName,
			DSN:        dsn,
			Dialect:    orm.NewQuestionDialect(),
			SetupSQL:   []string{"setup users"},
			CleanupSQL: []string{"cleanup users"},
			Timeout:    time.Second,
			Cases: []DatabaseCase{
				ExecStatementCase("exec", statement, orm.NamedArgs{"id": int64(7), "name": "Alice"}, func(result orm.Result) error {
					if result.RowsAffected != 1 {
						return fmt.Errorf("unexpected rows affected %d", result.RowsAffected)
					}
					return nil
				}),
			},
		})
	})

	if atomic.LoadInt64(&state.pings) == 0 {
		t.Fatalf("expected database ping")
	}
	execs := state.execStatements()
	if strings.Join(execs, "|") != "setup users|update users set name = ? where id = ?|cleanup users" {
		t.Fatalf("unexpected executed SQL %#v", execs)
	}
}

func registerSuiteTestDriver(t *testing.T) (string, string, *suiteTestState) {
	t.Helper()
	driverName := fmt.Sprintf("goark_orm_ormtest_%d", time.Now().UnixNano())
	dsn := driverName + "_dsn"
	state := &suiteTestState{}
	suiteTestStates.Store(dsn, state)
	sql.Register(driverName, suiteTestDriver{})
	t.Cleanup(func() {
		suiteTestStates.Delete(dsn)
	})
	return driverName, dsn, state
}

type suiteTestState struct {
	pings int64
	mu    sync.Mutex
	execs []string
}

func (s *suiteTestState) recordExec(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execs = append(s.execs, query)
}

func (s *suiteTestState) execStatements() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.execs...)
}

type suiteTestDriver struct{}

func (suiteTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := suiteTestStates.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown suite test dsn %s", name)
	}
	return &suiteTestConn{state: value.(*suiteTestState)}, nil
}

type suiteTestConn struct {
	state *suiteTestState
}

func (c *suiteTestConn) Prepare(query string) (driver.Stmt, error) {
	return &suiteTestStmt{conn: c, query: query}, nil
}

func (c *suiteTestConn) Close() error {
	return nil
}

func (c *suiteTestConn) Begin() (driver.Tx, error) {
	return suiteTestTx{}, nil
}

func (c *suiteTestConn) Ping(context.Context) error {
	atomic.AddInt64(&c.state.pings, 1)
	return nil
}

func (c *suiteTestConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.state.recordExec(query)
	return driver.RowsAffected(1), nil
}

func (c *suiteTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &suiteTestRows{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}, nil
}

type suiteTestStmt struct {
	conn  *suiteTestConn
	query string
}

func (s *suiteTestStmt) Close() error {
	return nil
}

func (s *suiteTestStmt) NumInput() int {
	return -1
}

func (s *suiteTestStmt) Exec([]driver.Value) (driver.Result, error) {
	s.conn.state.recordExec(s.query)
	return driver.RowsAffected(1), nil
}

func (s *suiteTestStmt) Query([]driver.Value) (driver.Rows, error) {
	return &suiteTestRows{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}, nil
}

type suiteTestTx struct{}

func (suiteTestTx) Commit() error {
	return nil
}

func (suiteTestTx) Rollback() error {
	return nil
}

type suiteTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *suiteTestRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (r *suiteTestRows) Close() error {
	return nil
}

func (r *suiteTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

var (
	_ driver.Pinger         = (*suiteTestConn)(nil)
	_ driver.ExecerContext  = (*suiteTestConn)(nil)
	_ driver.QueryerContext = (*suiteTestConn)(nil)
)
