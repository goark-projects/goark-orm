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

## Real Database Verification

Real database verification is not part of the default gate. To verify PostgreSQL or MySQL, create a caller-owned temporary test harness, blank-import the selected driver, and run `ormtest.RunCompatibilitySuiteFromEnv` with explicit environment variables.

The standard matrix and environment variables are documented in [database-matrix.md](database-matrix.md).
