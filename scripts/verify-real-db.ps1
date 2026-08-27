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
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
$sqliteImportBlock
	"goark.dev/orm/ormtest"
)

func TestPostgresCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_POSTGRES_DSN", "GOARK_ORM_POSTGRES_DRIVER", "pgx", "postgres")
}

func TestMySQLCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_MYSQL_DSN", "GOARK_ORM_MYSQL_DRIVER", "mysql", "mysql")
}

func TestMariaDBCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_MARIADB_DSN", "GOARK_ORM_MARIADB_DRIVER", "mysql", "mariadb")
}

func TestSQLiteCompatibility(t *testing.T) {
	runCompatibility(t, "GOARK_ORM_SQLITE_DSN", "GOARK_ORM_SQLITE_DRIVER", "sqlite", "sqlite")
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
