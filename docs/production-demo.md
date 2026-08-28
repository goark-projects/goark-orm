# Production Demo

The production demo lives under [examples/production](../examples/production). It is a real Go package layout that compiles without concrete database drivers or private credentials.

## Layout

| Path | Responsibility |
| --- | --- |
| `examples/production/goark-orm.json` | Generator configuration. |
| `examples/production/goark-orm-runtime.json` | Runtime configuration with strict JSON fields and no DSN. |
| `examples/production/account` | Entity, Mapper contract, XML mapper, SQL provider, generated metadata, and fill handler. |
| `examples/production/app` | Application assembly, runtime ownership, service-level validation, timeouts, and tests. |

## What It Demonstrates

- Explicit entity and mapper metadata.
- XML result maps, namespace cache, dynamic SQL, statement options, and `affectData` returning statements.
- Provider-based SQL builder usage with typed argument validation and cache keys.
- JSON TypeHandler registration through runtime configuration.
- Runtime assembly through `RuntimeConfig` and `RuntimeAssembly`.
- Optional audit middleware and SQL observation hooks.
- Service-layer resource protection: tenant validation, positive ID checks, page-size caps, email-limit caps, and context timeout handling.
- Tests that verify generated metadata, provider SQL compilation, runtime config assembly, and application service behavior.

## Generate

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json --check
```

## Test

```bash
GOWORK=off go test -count=1 ./examples/production/...
GOWORK=off go test -count=1 ./...
```

## Connect A Real Database

The demo does not store DSNs. A caller-owned binary or test harness should:

1. Import the concrete `database/sql` driver.
2. Open and tune `*sql.DB`.
3. Run migrations or setup SQL outside this repository.
4. Call `app.Assemble(ctx, app.RuntimeOptions{DB: db, ConfigPath: "examples/production/goark-orm-runtime.json"})`.
5. Close the returned runtime on shutdown.

```go
runtime, err := app.Assemble(ctx, app.RuntimeOptions{
	DB:         db,
	ConfigPath: "examples/production/goark-orm-runtime.json",
})
if err != nil {
	return err
}
defer runtime.Close()

users, err := runtime.Users.ListUsers(ctx, "tenant-a", account.UserStatusActive, orm.NewPageRequest(1, 20))
if err != nil {
	return err
}
_ = users
```

For database matrix verification, use [database-matrix.md](database-matrix.md) and the repository scripts:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db-bench.ps1 -BenchTime 1s
```
