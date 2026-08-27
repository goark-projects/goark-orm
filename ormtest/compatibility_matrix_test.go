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

func TestNewCompatibilitySuiteConfig_whenMariaDB_shouldUseMySQLCompatibleDDL(t *testing.T) {
	config, err := NewCompatibilitySuiteConfig(orm.DbTypeMariaDB)
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	if config.Dialect.Name() != "mariadb" {
		t.Fatalf("unexpected dialect %s", config.Dialect.Name())
	}
	if len(config.SetupSQL) != 6 {
		t.Fatalf("unexpected setup SQL count %d", len(config.SetupSQL))
	}
	if !strings.Contains(config.SetupSQL[3], "AUTO_INCREMENT") {
		t.Fatalf("mariadb key DDL should declare AUTO_INCREMENT: %s", config.SetupSQL[3])
	}
	if !strings.Contains(config.SetupSQL[5], "CREATE PROCEDURE `goark_orm_compat_users_report`") {
		t.Fatalf("mariadb setup SQL should create callable procedure: %s", config.SetupSQL[5])
	}
	if len(config.Cases) != 11 || !containsDatabaseCase(config.Cases, "compatibility-callable") {
		t.Fatalf("expected callable cases for mariadb, got %#v", config.Cases)
	}
}

func TestNewCompatibilitySuiteConfig_whenSQLite_shouldBuildDDLWithoutCallable(t *testing.T) {
	config, err := NewCompatibilitySuiteConfig(orm.DbTypeSQLite)
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	if config.Dialect.Name() != "sqlite" {
		t.Fatalf("unexpected dialect %s", config.Dialect.Name())
	}
	if len(config.SetupSQL) != 4 {
		t.Fatalf("unexpected setup SQL count %d", len(config.SetupSQL))
	}
	if !strings.Contains(config.SetupSQL[2], "AUTOINCREMENT") {
		t.Fatalf("sqlite key DDL should declare AUTOINCREMENT: %s", config.SetupSQL[2])
	}
	if strings.Contains(strings.Join(config.SetupSQL, " "), "PROCEDURE") || strings.Contains(strings.Join(config.SetupSQL, " "), "FUNCTION") {
		t.Fatalf("sqlite setup SQL must not create callable routine: %#v", config.SetupSQL)
	}
	if len(config.Cases) != 10 || containsDatabaseCase(config.Cases, "compatibility-callable") {
		t.Fatalf("sqlite should skip callable case, got %#v", config.Cases)
	}
}

func TestNewCompatibilitySuiteConfig_whenUnsupportedDBType_shouldReject(t *testing.T) {
	for _, dbType := range []orm.DbType{orm.DbTypeQuestion, orm.DbTypeSQLServer, orm.DbTypeOracle} {
		_, err := NewCompatibilitySuiteConfig(dbType)
		if err == nil || !strings.Contains(err.Error(), "supports postgres, mysql, mariadb and sqlite") {
			t.Fatalf("expected %s to be rejected, got %v", dbType, err)
		}
	}
}

func TestSupportedCompatibilityDBTypes_shouldExposeStandardRealDatabaseTargets(t *testing.T) {
	supported := SupportedCompatibilityDBTypes()
	expected := []orm.DbType{orm.DbTypePostgres, orm.DbTypeMySQL, orm.DbTypeMariaDB, orm.DbTypeSQLite}
	if len(supported) != len(expected) {
		t.Fatalf("unexpected supported database types %#v", supported)
	}
	for index, dbType := range expected {
		if supported[index] != dbType || !IsCompatibilityDBTypeSupported(dbType) {
			t.Fatalf("expected %s at index %d, got %#v", dbType, index, supported)
		}
	}
	supported[0] = orm.DbTypeOracle
	if IsCompatibilityDBTypeSupported(orm.DbTypeOracle) {
		t.Fatalf("oracle must stay outside current standard real database boundary")
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
