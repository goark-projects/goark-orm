# Goark ORM Local Release Gates

## Scope

Release gates are executed by maintainers on a local workstation or release host. The repository does not define remote CI for this module. Gates validate the core runtime, generator, examples, API contracts, and performance smoke tests.

The gates do not run migrations, store private DSNs, or import concrete database drivers into core packages.

## Standard Gate

```bash
GOWORK=off ./scripts/verify-release.sh
```

The script runs:

- `gofmt -l` over tracked and non-ignored Go files.
- `goark-orm generate orm --dir examples/minimal --check` to verify generated example files are current.
- `go test -count=1 ./...`.
- `go vet ./...`.
- `git diff --check`.
- Core performance smoke tests in `./internal/runtime` with a fixed `-benchtime=100x`.

Use a longer performance run when needed:

```bash
GOARK_ORM_BENCHTIME=1s GOWORK=off ./scripts/verify-release.sh
```

## Performance Threshold Gate

Use the PowerShell threshold gate when a change touches SQL compilation, dynamic SQL, wrappers, scanning, TypeHandler, cache key generation, or generated Mapper code:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-bench.ps1
```

The threshold file is [../scripts/benchmark-thresholds.json](../scripts/benchmark-thresholds.json). The gate always enforces `B/op` and `allocs/op`. Add `-EnforceTime` on a stable local or release host to also enforce `ns/op`:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-bench.ps1 -EnforceTime
```

The current threshold set runs against `./internal/runtime` and includes explicit allocation budgets for dynamic SQL rendering and SQLSession scan paths, including generated row scanners and TypeHandler-backed result mapping.

## Real Database Verification

Real database verification is not part of the default gate. Use `scripts/verify-real-db.ps1` to create a temporary driver harness and run the PostgreSQL, MySQL, MariaDB, SQLite, SQL Server, and Oracle compatibility matrix without importing concrete drivers into core packages.

Use `scripts/verify-real-db-bench.ps1` for the matching real-database benchmark matrix. The benchmark suite measures prepared query reuse, generated row scanners, ResultMap JSON mapping, single-row insert, multi-row insert, batch insert, and native upsert. Treat `ns/op` as environment-specific because network, storage, database configuration, and driver behavior dominate absolute latency; compare repeated runs on the same release host.

PostgreSQL and MySQL are part of the standard local matrix when their DSNs are configured. MariaDB uses the MySQL-compatible driver path by default. SQL Server can create the target database from `GOARK_ORM_SQLSERVER_ADMIN_DSN` when `GOARK_ORM_SQLSERVER_DSN` is empty. SQLite is optional and requires both `GOARK_ORM_SQLITE_DSN` and `GOARK_ORM_SQLITE_IMPORT`; its standard suite skips callable statements because Go `database/sql` SQLite drivers do not expose a portable stored-procedure model.

The standard matrix and environment variables are documented in [database-matrix.md](database-matrix.md).
