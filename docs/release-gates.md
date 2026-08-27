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
- Core performance smoke tests with a fixed `-benchtime=100x`.

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

## Real Database Verification

Real database verification is not part of the default gate. Use `scripts/verify-real-db.ps1` to create a temporary driver harness and run the PostgreSQL/MySQL compatibility matrix without importing concrete drivers into core packages.

The standard matrix and environment variables are documented in [database-matrix.md](database-matrix.md).
