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
	if len(config.SetupSQL) != 6 || !strings.Contains(config.SetupSQL[4], `"public"."goark_orm_compat_users"`) {
		t.Fatalf("unexpected setup SQL %#v", config.SetupSQL)
	}
	if !strings.Contains(config.SetupSQL[3], `"public"."goark_orm_compat_users_keys"`) {
		t.Fatalf("setup SQL should create generated key table: %#v", config.SetupSQL)
	}
	if !strings.Contains(config.SetupSQL[5], `CREATE FUNCTION "public"."goark_orm_compat_users_report"`) {
		t.Fatalf("setup SQL should create callable function: %#v", config.SetupSQL)
	}
	if len(config.Cases) != 11 {
		t.Fatalf("unexpected case count %d", len(config.Cases))
	}
	if !containsDatabaseCase(config.Cases, "compatibility-callable") {
		t.Fatalf("expected callable compatibility case")
	}
	statement, ok := config.Registry.Statement(defaultCompatibilityNamespace + ".SelectOne")
	if !ok {
		t.Fatalf("expected select statement")
	}
	if !strings.Contains(statement.SQL, `"public"."goark_orm_compat_users"`) {
		t.Fatalf("statement did not use quoted table: %s", statement.SQL)
	}
	call, ok := config.Registry.Statement(defaultCompatibilityNamespace + ".CallReport")
	if !ok {
		t.Fatalf("expected callable statement")
	}
	if call.Command != orm.StatementCommandCall || len(call.ResultSets) != 1 {
		t.Fatalf("unexpected callable statement %#v", call)
	}
}

func TestNewCompatibilitySuiteConfig_whenMySQL_shouldUseUTF8MB4DDL(t *testing.T) {
	config, err := NewCompatibilitySuiteConfig(orm.DbTypeMySQL)
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	if len(config.SetupSQL) != 6 {
		t.Fatalf("unexpected setup SQL count %d", len(config.SetupSQL))
	}
	if !strings.Contains(config.SetupSQL[3], "AUTO_INCREMENT") {
		t.Fatalf("mysql key DDL should declare AUTO_INCREMENT: %s", config.SetupSQL[3])
	}
	if !strings.Contains(config.SetupSQL[4], "DEFAULT CHARSET=utf8mb4") {
		t.Fatalf("mysql DDL should declare utf8mb4: %s", config.SetupSQL[4])
	}
	if !strings.Contains(config.SetupSQL[4], "`goark_orm_compat_users`") {
		t.Fatalf("mysql DDL should quote table: %s", config.SetupSQL[4])
	}
	if !strings.Contains(config.SetupSQL[5], "CREATE PROCEDURE `goark_orm_compat_users_report`") {
		t.Fatalf("mysql setup SQL should create callable procedure: %s", config.SetupSQL[5])
	}
}

func containsDatabaseCase(cases []DatabaseCase, name string) bool {
	for _, testCase := range cases {
		if testCase.Name == name {
			return true
		}
	}
	return false
}

func TestNewCompatibilitySuiteConfig_whenTableNameInvalid_shouldReject(t *testing.T) {
	_, err := NewCompatibilitySuiteConfig(orm.DbTypePostgres, WithCompatibilityTable("compat;drop table users"))
	if err == nil {
		t.Fatalf("expected invalid table error")
	}
}

func TestCompatibilityRelatedTable_whenSchemaProvided_shouldSuffixTableOnly(t *testing.T) {
	table, err := compatibilityRelatedTable("public.goark_orm_compat_users", "_keys")
	if err != nil {
		t.Fatalf("build related table failed: %v", err)
	}
	if table != "public.goark_orm_compat_users_keys" {
		t.Fatalf("unexpected related table %q", table)
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
