package ormboot_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/ormboot"
)

func TestAssembler_Assemble_whenMetadataRegistrarProvided_shouldCreateRuntimeAndBeans(t *testing.T) {
	db := sql.OpenDB(noopConnector{})
	defer db.Close()
	assembler, err := ormboot.New(ormboot.Config{
		Name: "userORM",
		DB:   db,
		MyBatisConfig: orm.MyBatisConfig{
			Settings:    orm.MyBatisSettings{DefaultExecutorType: orm.ExecutorTypeReuse},
			Environment: orm.MyBatisEnvironment{DbType: orm.DbTypePostgres},
			Mappers:     []orm.MapperRef{{Namespace: "system.user.UserMapper"}},
		},
		MetadataRegistrars: []ormboot.MetadataRegistrar{
			func(registry *orm.Registry) error {
				return registry.RegisterMapper(orm.MapperMeta{
					TypeName:  "UserMapper",
					Namespace: "system.user.UserMapper",
				})
			},
		},
	})
	if err != nil {
		t.Fatalf("new assembler failed: %v", err)
	}
	if assembler.Name() != "userORM" {
		t.Fatalf("unexpected assembler name %q", assembler.Name())
	}

	runtime, err := assembler.Assemble(context.Background())
	if err != nil {
		t.Fatalf("assemble runtime failed: %v", err)
	}
	defer runtime.Close()

	if runtime.SessionFactory() == nil {
		t.Fatalf("expected session factory")
	}
	if runtime.Configuration().Dialect.Name() != "postgres" {
		t.Fatalf("unexpected dialect %s", runtime.Configuration().Dialect.Name())
	}
	if runtime.Configuration().DefaultExecutorType != orm.ExecutorTypeReuse {
		t.Fatalf("expected REUSE executor")
	}
	if _, ok := runtime.Registry().Mapper("system.user.UserMapper"); !ok {
		t.Fatalf("expected registered mapper")
	}
	if !hasBean(runtime.BeanRegistrations(), ormboot.BeanNameSessionFactory) {
		t.Fatalf("expected session factory bean registration")
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("close runtime failed: %v", err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("second close should be idempotent: %v", err)
	}
}

func TestAssembler_Assemble_whenRegistrarFails_shouldReturnContextualError(t *testing.T) {
	db := sql.OpenDB(noopConnector{})
	defer db.Close()
	expected := errors.New("metadata failed")
	assembler, err := ormboot.New(ormboot.Config{
		DB: db,
		MetadataRegistrars: []ormboot.MetadataRegistrar{
			func(*orm.Registry) error {
				return expected
			},
		},
	})
	if err != nil {
		t.Fatalf("new assembler failed: %v", err)
	}

	_, err = assembler.Assemble(context.Background())

	if !errors.Is(err, expected) || !strings.Contains(err.Error(), "register metadata") {
		t.Fatalf("expected wrapped metadata error, got %v", err)
	}
}

func TestNew_whenDatabaseMissing_shouldReject(t *testing.T) {
	_, err := ormboot.New(ormboot.Config{})
	if err == nil || !strings.Contains(err.Error(), "database is nil") {
		t.Fatalf("expected missing database error, got %v", err)
	}
}

func hasBean(registrations []ormboot.BeanRegistration, name string) bool {
	for _, registration := range registrations {
		if registration.Name == name && registration.Instance != nil {
			return true
		}
	}
	return false
}

type noopConnector struct{}

func (noopConnector) Connect(context.Context) (driver.Conn, error) {
	return noopConn{}, nil
}

func (noopConnector) Driver() driver.Driver {
	return noopDriver{}
}

type noopDriver struct{}

func (noopDriver) Open(string) (driver.Conn, error) {
	return noopConn{}, nil
}

type noopConn struct{}

func (noopConn) Prepare(string) (driver.Stmt, error) {
	return noopStmt{}, nil
}

func (noopConn) Close() error {
	return nil
}

func (noopConn) Begin() (driver.Tx, error) {
	return noopTx{}, nil
}

func (noopConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return noopTx{}, nil
}

func (noopConn) Ping(context.Context) error {
	return nil
}

func (noopConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

func (noopConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return noopRows{}, nil
}

type noopStmt struct{}

func (noopStmt) Close() error {
	return nil
}

func (noopStmt) NumInput() int {
	return -1
}

func (noopStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

func (noopStmt) Query([]driver.Value) (driver.Rows, error) {
	return noopRows{}, nil
}

type noopTx struct{}

func (noopTx) Commit() error {
	return nil
}

func (noopTx) Rollback() error {
	return nil
}

type noopRows struct{}

func (noopRows) Columns() []string {
	return []string{"id"}
}

func (noopRows) Close() error {
	return nil
}

func (noopRows) Next([]driver.Value) error {
	return io.EOF
}

var (
	_ driver.Pinger         = noopConn{}
	_ driver.ExecerContext  = noopConn{}
	_ driver.QueryerContext = noopConn{}
	_ driver.ConnBeginTx    = noopConn{}
)
