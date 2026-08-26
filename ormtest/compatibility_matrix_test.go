package ormtest

import (
	"context"
	"strings"
	"testing"

	orm "goark.dev/orm"
)

func TestNewCompatibilitySuiteConfig_whenPostgres_shouldBuildQuotedDDLAndCases(t *testing.T) {
	config, err := NewCompatibilitySuiteConfig(orm.DbTypePostgres, WithCompatibilityTable("public.goark_orm_compat_users"))
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	if config.Dialect.Name() != "postgres" {
		t.Fatalf("unexpected dialect %s", config.Dialect.Name())
	}
	if len(config.SetupSQL) != 2 || !strings.Contains(config.SetupSQL[1], `"public"."goark_orm_compat_users"`) {
		t.Fatalf("unexpected setup SQL %#v", config.SetupSQL)
	}
	if len(config.Cases) != 7 {
		t.Fatalf("unexpected case count %d", len(config.Cases))
	}
	statement, ok := config.Registry.Statement(defaultCompatibilityNamespace + ".SelectOne")
	if !ok {
		t.Fatalf("expected select statement")
	}
	if !strings.Contains(statement.SQL, `"public"."goark_orm_compat_users"`) {
		t.Fatalf("statement did not use quoted table: %s", statement.SQL)
	}
}

func TestNewCompatibilitySuiteConfig_whenMySQL_shouldUseUTF8MB4DDL(t *testing.T) {
	config, err := NewCompatibilitySuiteConfig(orm.DbTypeMySQL)
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	if len(config.SetupSQL) != 2 {
		t.Fatalf("unexpected setup SQL count %d", len(config.SetupSQL))
	}
	if !strings.Contains(config.SetupSQL[1], "DEFAULT CHARSET=utf8mb4") {
		t.Fatalf("mysql DDL should declare utf8mb4: %s", config.SetupSQL[1])
	}
	if !strings.Contains(config.SetupSQL[1], "`goark_orm_compat_users`") {
		t.Fatalf("mysql DDL should quote table: %s", config.SetupSQL[1])
	}
}

func TestNewCompatibilitySuiteConfig_whenTableNameInvalid_shouldReject(t *testing.T) {
	_, err := NewCompatibilitySuiteConfig(orm.DbTypePostgres, WithCompatibilityTable("compat;drop table users"))
	if err == nil {
		t.Fatalf("expected invalid table error")
	}
}

func TestCompatibilityJSONTypeHandler_shouldRoundTripProfile(t *testing.T) {
	handler := compatibilityJSONTypeHandler{}
	value := CompatibilityProfile{Role: "admin", Level: 9}

	databaseValue, err := handler.ToDB(context.Background(), value)
	if err != nil {
		t.Fatalf("json ToDB failed: %v", err)
	}
	text, ok := databaseValue.(string)
	if !ok || !strings.Contains(text, `"role":"admin"`) {
		t.Fatalf("unexpected database value %#v", databaseValue)
	}
	var out CompatibilityProfile
	if err := handler.FromDB(context.Background(), databaseValue, &out); err != nil {
		t.Fatalf("json FromDB failed: %v", err)
	}
	if out != value {
		t.Fatalf("unexpected profile %#v", out)
	}
}
