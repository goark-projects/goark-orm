param(
    [string]$PostgresDriver = $env:GOARK_ORM_POSTGRES_DRIVER,
    [string]$PostgresDSN = $env:GOARK_ORM_POSTGRES_DSN,
    [string]$MySQLDriver = $env:GOARK_ORM_MYSQL_DRIVER,
    [string]$MySQLDSN = $env:GOARK_ORM_MYSQL_DSN,
    [string]$MariaDBDriver = $env:GOARK_ORM_MARIADB_DRIVER,
    [string]$MariaDBDSN = $env:GOARK_ORM_MARIADB_DSN,
    [string]$SQLiteDriver = $env:GOARK_ORM_SQLITE_DRIVER,
    [string]$SQLiteDSN = $env:GOARK_ORM_SQLITE_DSN,
    [string]$SQLiteImport = $env:GOARK_ORM_SQLITE_IMPORT
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($PostgresDriver)) {
    $PostgresDriver = "pgx"
}
if ([string]::IsNullOrWhiteSpace($MySQLDriver)) {
    $MySQLDriver = "mysql"
}
if ([string]::IsNullOrWhiteSpace($MariaDBDriver)) {
    $MariaDBDriver = "mysql"
}
if ([string]::IsNullOrWhiteSpace($SQLiteDriver)) {
    $SQLiteDriver = "sqlite"
}
if (-not [string]::IsNullOrWhiteSpace($SQLiteDSN) -and [string]::IsNullOrWhiteSpace($SQLiteImport)) {
    throw "GOARK_ORM_SQLITE_IMPORT is required when GOARK_ORM_SQLITE_DSN is set"
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("goark-orm-real-db-" + [System.Guid]::NewGuid().ToString("N"))
$sqliteImportBlock = ""
if (-not [string]::IsNullOrWhiteSpace($SQLiteDSN)) {
    $sqliteImportBlock = "`n`t_ `"$SQLiteImport`""
}

function Write-Utf8File {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Content
    )
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

function Assert-GoImportPath {
    param(
        [Parameter(Mandatory = $true)][string]$ImportPath
    )
    if ($ImportPath -notmatch '^[A-Za-z0-9._~/+-]+$') {
        throw "invalid Go import path: $ImportPath"
    }
}

try {
    New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
    if (-not [string]::IsNullOrWhiteSpace($SQLiteDSN)) {
        Assert-GoImportPath -ImportPath $SQLiteImport
    }
    Write-Utf8File -Path (Join-Path $tempRoot "go.mod") -Content @"
module goark.dev/orm-real-db-smoke

go 1.25

require (
	github.com/go-sql-driver/mysql v1.9.3
	github.com/jackc/pgx/v5 v5.7.6
	goark.dev/orm v0.0.0
)

replace goark.dev/orm => $($repoRoot.Replace("\", "/"))
"@
    Write-Utf8File -Path (Join-Path $tempRoot "smoke_test.go") -Content @"
package ormrealdbsmoke_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
$sqliteImportBlock
	orm "goark.dev/orm"
	"goark.dev/orm/ormgen"
	"goark.dev/orm/ormtest"
)

const ormReplacePath = "$($repoRoot.Replace("\", "/"))"

func TestPostgresCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_POSTGRES_DSN", "GOARK_ORM_POSTGRES_DRIVER", "pgx", "postgres")
}

func TestPostgresReverseGeneratedSourceCompiles(t *testing.T) {
	runReverseGeneratedSource(t, "GOARK_ORM_POSTGRES_DSN", "GOARK_ORM_POSTGRES_DRIVER", "pgx", orm.DbTypePostgres)
}

func TestMySQLCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_MYSQL_DSN", "GOARK_ORM_MYSQL_DRIVER", "mysql", "mysql")
}

func TestMySQLReverseGeneratedSourceCompiles(t *testing.T) {
	runReverseGeneratedSource(t, "GOARK_ORM_MYSQL_DSN", "GOARK_ORM_MYSQL_DRIVER", "mysql", orm.DbTypeMySQL)
}

func TestMariaDBCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_MARIADB_DSN", "GOARK_ORM_MARIADB_DRIVER", "mysql", "mariadb")
}

func TestMariaDBReverseGeneratedSourceCompiles(t *testing.T) {
	runReverseGeneratedSource(t, "GOARK_ORM_MARIADB_DSN", "GOARK_ORM_MARIADB_DRIVER", "mysql", orm.DbTypeMariaDB)
}

func TestSQLiteCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_SQLITE_DSN", "GOARK_ORM_SQLITE_DRIVER", "sqlite", "sqlite")
}

func TestSQLiteReverseGeneratedSourceCompiles(t *testing.T) {
	runReverseGeneratedSource(t, "GOARK_ORM_SQLITE_DSN", "GOARK_ORM_SQLITE_DRIVER", "sqlite", orm.DbTypeSQLite)
}

func runCompatibility(t *testing.T, dsnEnv string, driverEnv string, defaultDriver string, dbType string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(dsnEnv))
	if dsn == "" {
		t.Skipf("set %s to run %s compatibility suite", dsnEnv, dbType)
	}
	driver := strings.TrimSpace(os.Getenv(driverEnv))
	if driver == "" {
		driver = defaultDriver
	}
	t.Setenv("GOARK_ORM_INTEGRATION_DRIVER", driver)
	t.Setenv("GOARK_ORM_INTEGRATION_DSN", dsn)
	t.Setenv("GOARK_ORM_INTEGRATION_DBTYPE", dbType)
	ormtest.RunCompatibilitySuiteFromEnv(t)
}

func runReverseGeneratedSource(t *testing.T, dsnEnv string, driverEnv string, defaultDriver string, dbType orm.DbType) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(dsnEnv))
	if dsn == "" {
		t.Skipf("set %s to run %s reverse generated source suite", dsnEnv, dbType)
	}
	driver := strings.TrimSpace(os.Getenv(driverEnv))
	if driver == "" {
		driver = defaultDriver
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("open %s database failed: %v", dbType, err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping %s database failed: %v", dbType, err)
	}
	table := "goark_orm_reverse_users"
	setupSQL, cleanupSQL, schema, err := reverseGeneratedSourceSQL(dbType, table)
	if err != nil {
		t.Fatalf("build reverse generated source SQL failed: %v", err)
	}
	execSQLList(t, ctx, db, cleanupSQL)
	defer cleanupSQLList(t, db, cleanupSQL)
	execSQLList(t, ctx, db, setupSQL)
	schemaDialect, err := ormgen.NewSQLSchemaDialect(dbType)
	if err != nil {
		t.Fatalf("new schema dialect failed: %v", err)
	}
	introspector, err := ormgen.NewSQLSchemaIntrospector(db, schemaDialect)
	if err != nil {
		t.Fatalf("new schema introspector failed: %v", err)
	}
	source, err := ormgen.ReverseEngineerWithRenderer(ctx, introspector, ormgen.ReverseEngineerSpec{
		PackageName: "reversegen",
		DatabaseID:  string(dbType),
		Schema:      schema,
		Tables:      []string{table},
		TablePrefix: "goark_orm_",
		ColumnOverrides: map[string]ormgen.SchemaColumnOverride{
			"profile": {GoType: "sonic.NoCopyRawMessage", TypeHandler: "json"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("reverse engineer generated source failed: %v", err)
	}
	generated := string(source)
	if !strings.Contains(generated, "\"github.com/bytedance/sonic\"") {
		t.Fatalf("generated source should import sonic:\n%s", generated)
	}
	if strings.Contains(generated, "\"encoding/json\"") {
		t.Fatalf("generated source must not import encoding/json:\n%s", generated)
	}
	compileGeneratedSource(t, source)
}

func reverseGeneratedSourceSQL(dbType orm.DbType, table string) ([]string, []string, string, error) {
	cleanup := []string{"DROP TABLE IF EXISTS " + table}
	switch dbType {
	case orm.DbTypePostgres:
		return []string{
			"CREATE TABLE " + table + " (id BIGSERIAL PRIMARY KEY, name VARCHAR(64) NOT NULL, profile JSONB NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)",
		}, cleanup, "public", nil
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		return []string{
			"CREATE TABLE " + table + " (id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(64) NOT NULL, profile JSON NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		}, cleanup, "", nil
	case orm.DbTypeSQLite:
		return []string{
			"CREATE TABLE " + table + " (id INTEGER PRIMARY KEY AUTOINCREMENT, name VARCHAR(64) NOT NULL, profile JSON NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)",
		}, cleanup, "", nil
	default:
		return nil, nil, "", fmt.Errorf("unsupported reverse generated source db type %q", dbType)
	}
}

func execSQLList(t *testing.T, ctx context.Context, db *sql.DB, statements []string) {
	t.Helper()
	for _, statement := range statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("exec SQL %q failed: %v", statement, err)
		}
	}
}

func cleanupSQLList(t *testing.T, db *sql.DB, statements []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, statement := range statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Errorf("cleanup SQL %q failed: %v", statement, err)
		}
	}
}

func compileGeneratedSource(t *testing.T, source []byte) {
	t.Helper()
	dir := t.TempDir()
	writeTextFile(t, filepath.Join(dir, "go.mod"), "module goark.dev/orm-reverse-generated-smoke\n\ngo 1.25\n\nrequire (\n\tgithub.com/bytedance/sonic v1.15.2\n\tgoark.dev/orm v0.0.0\n)\n\nreplace goark.dev/orm => "+ormReplacePath+"\n")
	writeBytesFile(t, filepath.Join(dir, "zz_goark_orm_reverse_gen.go"), source)
	writeTextFile(t, filepath.Join(dir, "generated_test.go"), "package reversegen\n\nimport (\n\t\"testing\"\n\n\torm \"goark.dev/orm\"\n)\n\nfunc TestGeneratedReverseMetadataCompiles(t *testing.T) {\n\tregistry := orm.NewRegistry()\n\tif err := registry.RegisterTypeHandler(\"json\", orm.NewJSONTypeHandler()); err != nil {\n\t\tt.Fatalf(\"register json type-handler failed: %v\", err)\n\t}\n\tif err := RegisterGoarkORMMetadata(registry); err != nil {\n\t\tt.Fatalf(\"register metadata failed: %v\", err)\n\t}\n\tif err := registry.Validate(); err != nil {\n\t\tt.Fatalf(\"validate metadata failed: %v\", err)\n\t}\n}\n")
	cmd := exec.Command("go", "test", "-mod=mod", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated source did not compile: %v\n%s", err, output)
	}
}

func writeTextFile(t *testing.T, path string, content string) {
	t.Helper()
	writeBytesFile(t, path, []byte(content))
}

func writeBytesFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s failed: %v", path, err)
	}
}
"@

    Push-Location $tempRoot
    try {
        $env:GOWORK = "off"
        $env:GOARK_ORM_POSTGRES_DRIVER = $PostgresDriver
        $env:GOARK_ORM_POSTGRES_DSN = $PostgresDSN
        $env:GOARK_ORM_MYSQL_DRIVER = $MySQLDriver
        $env:GOARK_ORM_MYSQL_DSN = $MySQLDSN
        $env:GOARK_ORM_MARIADB_DRIVER = $MariaDBDriver
        $env:GOARK_ORM_MARIADB_DSN = $MariaDBDSN
        $env:GOARK_ORM_SQLITE_DRIVER = $SQLiteDriver
        $env:GOARK_ORM_SQLITE_DSN = $SQLiteDSN
        go mod tidy
        go test -count=1 -v
    } finally {
        Pop-Location
    }
} finally {
    if (Test-Path -LiteralPath $tempRoot) {
        $resolvedTemp = (Resolve-Path -LiteralPath $tempRoot).Path
        if ($resolvedTemp.StartsWith([System.IO.Path]::GetTempPath(), [System.StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
        }
    }
}
