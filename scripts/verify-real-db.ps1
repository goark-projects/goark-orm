param(
    [string]$PostgresDriver = $env:GOARK_ORM_POSTGRES_DRIVER,
    [string]$PostgresDSN = $env:GOARK_ORM_POSTGRES_DSN,
    [string]$MySQLDriver = $env:GOARK_ORM_MYSQL_DRIVER,
    [string]$MySQLDSN = $env:GOARK_ORM_MYSQL_DSN
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($PostgresDriver)) {
    $PostgresDriver = "pgx"
}
if ([string]::IsNullOrWhiteSpace($MySQLDriver)) {
    $MySQLDriver = "mysql"
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("goark-orm-real-db-" + [System.Guid]::NewGuid().ToString("N"))

function Write-Utf8File {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Content
    )
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

try {
    New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
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
	"goark.dev/orm/ormtest"
)

func TestPostgresCompatibility(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("GOARK_ORM_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set GOARK_ORM_POSTGRES_DSN to run PostgreSQL compatibility suite")
	}
	driver := strings.TrimSpace(os.Getenv("GOARK_ORM_POSTGRES_DRIVER"))
	if driver == "" {
		driver = "pgx"
	}
	t.Setenv("GOARK_ORM_INTEGRATION_DRIVER", driver)
	t.Setenv("GOARK_ORM_INTEGRATION_DSN", dsn)
	t.Setenv("GOARK_ORM_INTEGRATION_DBTYPE", "postgres")
	ormtest.RunCompatibilitySuiteFromEnv(t)
}

func TestMySQLCompatibility(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("GOARK_ORM_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set GOARK_ORM_MYSQL_DSN to run MySQL compatibility suite")
	}
	driver := strings.TrimSpace(os.Getenv("GOARK_ORM_MYSQL_DRIVER"))
	if driver == "" {
		driver = "mysql"
	}
	t.Setenv("GOARK_ORM_INTEGRATION_DRIVER", driver)
	t.Setenv("GOARK_ORM_INTEGRATION_DSN", dsn)
	t.Setenv("GOARK_ORM_INTEGRATION_DBTYPE", "mysql")
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
