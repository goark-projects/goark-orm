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
	if !strings.Contains(config.SetupSQL[4], "profile JSONB NOT NULL") {
		t.Fatalf("postgres DDL should declare native JSONB: %s", config.SetupSQL[4])
	}
	if len(config.Cases) != 12 {
		t.Fatalf("unexpected case count %d", len(config.Cases))
	}
	if !containsDatabaseCase(config.Cases, compatibilityJSONNativeCaseName) {
		t.Fatalf("expected native json compatibility case")
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
	if !strings.Contains(config.SetupSQL[4], "profile JSON NOT NULL") {
		t.Fatalf("mysql DDL should declare native JSON: %s", config.SetupSQL[4])
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
	if len(config.Cases) != 12 || !containsDatabaseCase(config.Cases, "compatibility-callable") {
		t.Fatalf("expected callable cases for mariadb, got %#v", config.Cases)
	}
	if !containsDatabaseCase(config.Cases, compatibilityJSONNativeCaseName) {
		t.Fatalf("expected native json compatibility case")
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
	if !strings.Contains(config.SetupSQL[3], "json_valid(profile)") {
		t.Fatalf("sqlite DDL should validate JSON text: %s", config.SetupSQL[3])
	}
	if strings.Contains(strings.Join(config.SetupSQL, " "), "PROCEDURE") || strings.Contains(strings.Join(config.SetupSQL, " "), "FUNCTION") {
		t.Fatalf("sqlite setup SQL must not create callable routine: %#v", config.SetupSQL)
	}
	if len(config.Cases) != 11 || containsDatabaseCase(config.Cases, "compatibility-callable") {
		t.Fatalf("sqlite should skip callable case, got %#v", config.Cases)
	}
	if !containsDatabaseCase(config.Cases, compatibilityJSONNativeCaseName) {
		t.Fatalf("expected native json compatibility case")
	}
}

func TestNewCompatibilitySuiteConfig_whenSQLServer_shouldBuildDDLAndCases(t *testing.T) {
	config, err := NewCompatibilitySuiteConfig(orm.DbTypeSQLServer, WithCompatibilityTable("dbo.goark_orm_compat_users"))
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	if config.Dialect.Name() != "sqlserver" {
		t.Fatalf("unexpected dialect %s", config.Dialect.Name())
	}
	if len(config.SetupSQL) != 6 {
		t.Fatalf("unexpected setup SQL count %d", len(config.SetupSQL))
	}
	if !strings.Contains(config.SetupSQL[3], "IDENTITY(1,1)") {
		t.Fatalf("sqlserver key DDL should declare identity: %s", config.SetupSQL[3])
	}
	if !strings.Contains(config.SetupSQL[4], "[dbo].[goark_orm_compat_users]") {
		t.Fatalf("sqlserver DDL should quote table: %s", config.SetupSQL[4])
	}
	if !strings.Contains(config.SetupSQL[4], "ISJSON(profile) = 1") {
		t.Fatalf("sqlserver DDL should validate JSON text: %s", config.SetupSQL[4])
	}
	if !containsDatabaseCase(config.Cases, compatibilityJSONNativeCaseName) {
		t.Fatalf("expected native json compatibility case")
	}
	if !containsDatabaseCase(config.Cases, "compatibility-callable") {
		t.Fatalf("expected callable compatibility case")
	}
}

func TestNewCompatibilitySuiteConfig_whenOracle_shouldBuildDDLAndCases(t *testing.T) {
	config, err := NewCompatibilitySuiteConfig(orm.DbTypeOracle)
	if err != nil {
		t.Fatalf("create compatibility suite failed: %v", err)
	}
	if config.Dialect.Name() != "oracle" {
		t.Fatalf("unexpected dialect %s", config.Dialect.Name())
	}
	if len(config.SetupSQL) != 4 {
		t.Fatalf("unexpected setup SQL count %d", len(config.SetupSQL))
	}
	if !strings.Contains(config.SetupSQL[2], "GENERATED BY DEFAULT ON NULL AS IDENTITY") {
		t.Fatalf("oracle key DDL should declare identity: %s", config.SetupSQL[2])
	}
	if !strings.Contains(config.SetupSQL[3], `"goark_orm_compat_users"`) {
		t.Fatalf("oracle DDL should quote table: %s", config.SetupSQL[3])
	}
	if !strings.Contains(config.SetupSQL[3], "profile IS JSON") {
		t.Fatalf("oracle DDL should validate JSON CLOB: %s", config.SetupSQL[3])
	}
	if !containsDatabaseCase(config.Cases, compatibilityJSONNativeCaseName) {
		t.Fatalf("expected native json compatibility case")
	}
	if !containsDatabaseCase(config.Cases, "compatibility-callable") {
		t.Fatalf("expected callable compatibility case")
	}
}

func TestCompatibilityJSONProbeSQL_whenDialectProvided_shouldUseNativeExpression(t *testing.T) {
	tests := []struct {
		name     string
		dialect  orm.Dialect
		expected string
	}{
		{name: "postgres", dialect: orm.NewPostgresDialect(), expected: "profile ->> 'role'"},
		{name: "mysql", dialect: orm.NewMySQLDialect(), expected: "JSON_UNQUOTE(JSON_EXTRACT(profile, '$.role'))"},
		{name: "sqlserver", dialect: orm.NewSQLServerDialect(), expected: "JSON_VALUE(profile, '$.role')"},
		{name: "oracle", dialect: orm.NewOracleDialect(), expected: "JSON_VALUE(profile, '$.role')"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlText, err := compatibilityJSONProbeSQL(tt.dialect, "goark_orm_compat_users")
			if err != nil {
				t.Fatalf("build JSON probe SQL failed: %v", err)
			}
			if !strings.Contains(sqlText, tt.expected) {
				t.Fatalf("unexpected JSON probe SQL %q", sqlText)
			}
		})
	}
}

func TestNewCompatibilitySuiteConfig_whenUnsupportedDBType_shouldReject(t *testing.T) {
	for _, dbType := range []orm.DbType{orm.DbTypeQuestion} {
		_, err := NewCompatibilitySuiteConfig(dbType)
		if err == nil || !strings.Contains(err.Error(), "supports postgres, mysql, mariadb, sqlite, sqlserver and oracle") {
			t.Fatalf("expected %s to be rejected, got %v", dbType, err)
		}
	}
}

func TestSupportedCompatibilityDBTypes_shouldExposeStandardRealDatabaseTargets(t *testing.T) {
	supported := SupportedCompatibilityDBTypes()
	expected := []orm.DbType{orm.DbTypePostgres, orm.DbTypeMySQL, orm.DbTypeMariaDB, orm.DbTypeSQLite, orm.DbTypeSQLServer, orm.DbTypeOracle}
	if len(supported) != len(expected) {
		t.Fatalf("unexpected supported database types %#v", supported)
	}
	for index, dbType := range expected {
		if supported[index] != dbType || !IsCompatibilityDBTypeSupported(dbType) {
			t.Fatalf("expected %s at index %d, got %#v", dbType, index, supported)
		}
	}
	supported[0] = orm.DbTypeOracle
	if !IsCompatibilityDBTypeSupported(orm.DbTypePostgres) {
		t.Fatalf("supported result mutation must not affect support checks")
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
