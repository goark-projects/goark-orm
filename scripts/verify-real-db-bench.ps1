param(
    [string]$PostgresDriver = $env:GOARK_ORM_POSTGRES_DRIVER,
    [string]$PostgresDSN = $env:GOARK_ORM_POSTGRES_DSN,
    [string]$MySQLDriver = $env:GOARK_ORM_MYSQL_DRIVER,
    [string]$MySQLDSN = $env:GOARK_ORM_MYSQL_DSN,
    [string]$MariaDBDriver = $env:GOARK_ORM_MARIADB_DRIVER,
    [string]$MariaDBDSN = $env:GOARK_ORM_MARIADB_DSN,
    [string]$SQLiteDriver = $env:GOARK_ORM_SQLITE_DRIVER,
    [string]$SQLiteDSN = $env:GOARK_ORM_SQLITE_DSN,
    [string]$SQLiteImport = $env:GOARK_ORM_SQLITE_IMPORT,
    [string]$SQLServerDriver = $env:GOARK_ORM_SQLSERVER_DRIVER,
    [string]$SQLServerDSN = $env:GOARK_ORM_SQLSERVER_DSN,
    [string]$SQLServerAdminDSN = $env:GOARK_ORM_SQLSERVER_ADMIN_DSN,
    [string]$SQLServerDatabase = $env:GOARK_ORM_SQLSERVER_DATABASE,
    [string]$OracleDriver = $env:GOARK_ORM_ORACLE_DRIVER,
    [string]$OracleDSN = $env:GOARK_ORM_ORACLE_DSN,
    [string]$BenchTime = $env:GOARK_ORM_REAL_DB_BENCHTIME
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
if ([string]::IsNullOrWhiteSpace($SQLServerDriver)) {
    $SQLServerDriver = "sqlserver"
}
if ([string]::IsNullOrWhiteSpace($SQLServerDatabase)) {
    $SQLServerDatabase = "goark_orm_test"
}
if ([string]::IsNullOrWhiteSpace($OracleDriver)) {
    $OracleDriver = "oracle"
}
if ([string]::IsNullOrWhiteSpace($BenchTime)) {
    $BenchTime = "1s"
}
if (-not [string]::IsNullOrWhiteSpace($SQLiteDSN) -and [string]::IsNullOrWhiteSpace($SQLiteImport)) {
    throw "GOARK_ORM_SQLITE_IMPORT is required when GOARK_ORM_SQLITE_DSN is set"
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("goark-orm-real-db-bench-" + [System.Guid]::NewGuid().ToString("N"))
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
module goark.dev/orm-real-db-bench

go 1.25

require (
	github.com/go-sql-driver/mysql v1.9.3
	github.com/jackc/pgx/v5 v5.7.6
	github.com/microsoft/go-mssqldb v1.11.0
	github.com/sijms/go-ora/v2 v2.9.0
	goark.dev/orm v0.0.0
)

replace goark.dev/orm => $($repoRoot.Replace("\", "/"))
"@
    Write-Utf8File -Path (Join-Path $tempRoot "real_db_benchmark_test.go") -Content @"
package ormrealdbbench_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"
$sqliteImportBlock
	orm "goark.dev/orm"
	"goark.dev/orm/ormtest"
)

func TestMain(m *testing.M) {
	if err := ensureSQLServerDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "prepare sqlserver database failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func BenchmarkPostgres(b *testing.B) {
	runBenchmark(b, "GOARK_ORM_POSTGRES_DSN", "GOARK_ORM_POSTGRES_DRIVER", "pgx", orm.DbTypePostgres)
}

func BenchmarkMySQL(b *testing.B) {
	runBenchmark(b, "GOARK_ORM_MYSQL_DSN", "GOARK_ORM_MYSQL_DRIVER", "mysql", orm.DbTypeMySQL)
}

func BenchmarkMariaDB(b *testing.B) {
	runBenchmark(b, "GOARK_ORM_MARIADB_DSN", "GOARK_ORM_MARIADB_DRIVER", "mysql", orm.DbTypeMariaDB)
}

func BenchmarkSQLite(b *testing.B) {
	runBenchmark(b, "GOARK_ORM_SQLITE_DSN", "GOARK_ORM_SQLITE_DRIVER", "sqlite", orm.DbTypeSQLite)
}

func BenchmarkSQLServer(b *testing.B) {
	runBenchmark(b, "GOARK_ORM_SQLSERVER_DSN", "GOARK_ORM_SQLSERVER_DRIVER", "sqlserver", orm.DbTypeSQLServer)
}

func BenchmarkOracle(b *testing.B) {
	runBenchmark(b, "GOARK_ORM_ORACLE_DSN", "GOARK_ORM_ORACLE_DRIVER", "oracle", orm.DbTypeOracle)
}

func runBenchmark(b *testing.B, dsnEnv string, driverEnv string, defaultDriver string, dbType orm.DbType) {
	b.Helper()
	dsn := strings.TrimSpace(os.Getenv(dsnEnv))
	if dsn == "" {
		b.Skipf("set %s to run %s benchmark", dsnEnv, dbType)
	}
	driver := strings.TrimSpace(os.Getenv(driverEnv))
	if driver == "" {
		driver = defaultDriver
	}
	config, err := ormtest.NewBenchmarkSuiteConfig(dbType)
	if err != nil {
		b.Fatalf("create benchmark suite failed: %v", err)
	}
	config.DriverName = driver
	config.DSN = dsn
	ormtest.RunDatabaseBenchmark(b, config)
}

func ensureSQLServerDatabase() error {
	if strings.TrimSpace(os.Getenv("GOARK_ORM_SQLSERVER_DSN")) != "" {
		return nil
	}
	adminDSN := strings.TrimSpace(os.Getenv("GOARK_ORM_SQLSERVER_ADMIN_DSN"))
	if adminDSN == "" {
		return nil
	}
	database := strings.TrimSpace(os.Getenv("GOARK_ORM_SQLSERVER_DATABASE"))
	if database == "" {
		database = "goark_orm_test"
	}
	if !safeSQLServerIdentifier(database) {
		return fmt.Errorf("invalid sqlserver database name %q", database)
	}
	driver := strings.TrimSpace(os.Getenv("GOARK_ORM_SQLSERVER_DRIVER"))
	if driver == "" {
		driver = "sqlserver"
	}
	db, err := sql.Open(driver, adminDSN)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "IF DB_ID(@p1) IS NULL BEGIN DECLARE @sql nvarchar(max) = N'CREATE DATABASE ' + QUOTENAME(@p1); EXEC (@sql); END", database)
	if err != nil {
		return err
	}
	dsn, err := sqlServerDSNForDatabase(adminDSN, database)
	if err != nil {
		return err
	}
	return os.Setenv("GOARK_ORM_SQLSERVER_DSN", dsn)
}

func sqlServerDSNForDatabase(adminDSN string, database string) (string, error) {
	parsed, err := url.Parse(adminDSN)
	if err != nil || parsed.Scheme == "" {
		return "", fmt.Errorf("sqlserver admin DSN must use URL form: %w", err)
	}
	query := parsed.Query()
	query.Set("database", database)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func safeSQLServerIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			continue
		}
		if index > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
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
        $env:GOARK_ORM_SQLSERVER_DRIVER = $SQLServerDriver
        $env:GOARK_ORM_SQLSERVER_DSN = $SQLServerDSN
        $env:GOARK_ORM_SQLSERVER_ADMIN_DSN = $SQLServerAdminDSN
        $env:GOARK_ORM_SQLSERVER_DATABASE = $SQLServerDatabase
        $env:GOARK_ORM_ORACLE_DRIVER = $OracleDriver
        $env:GOARK_ORM_ORACLE_DSN = $OracleDSN
        go mod tidy
        go test -run '^$' -bench . "-benchtime=$BenchTime" -benchmem -timeout 30m -v
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
