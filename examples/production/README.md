# Production Demo

English is the default language. The Chinese mirror is [README.zh-CN.md](README.zh-CN.md).

This demo is a production-oriented package layout for an account module. It compiles and tests without concrete database drivers or private credentials. A caller-owned application supplies the `*sql.DB`, imports the selected driver, runs migrations, and owns shutdown ordering.

## Layout

| Path | Responsibility |
| --- | --- |
| [goark-orm.json](goark-orm.json) | Generator configuration for the account package. |
| [goark-orm-runtime.json](goark-orm-runtime.json) | Runtime configuration: settings, environment, global DB config, type handlers, mappers, and plugins. |
| [account/model.go](account/model.go) | Entity model with ID generation, tenant field, JSON profile, optimistic lock, logical delete, and fill metadata. |
| [account/mapper.go](account/mapper.go) | Mapper contract with explicit namespace, XML binding, pagination, `affectData`, and provider method. |
| [account/mapper/user_mapper.xml](account/mapper/user_mapper.xml) | XML result map, namespace cache, dynamic SQL, statement options, and PostgreSQL returning statement. |
| [account/provider.go](account/provider.go) | Provider registration and SQL builder with typed argument validation. |
| [account/fill.go](account/fill.go) | `MetaObjectHandler` for insert/update audit fields. |
| [account/zz_goark_orm_account_gen.go](account/zz_goark_orm_account_gen.go) | Generated metadata and mapper implementation. |
| [app/runtime.go](app/runtime.go) | Runtime assembly from explicit metadata, runtime JSON, type handlers, audit middleware, and SQL observation. |
| [app/user_service.go](app/user_service.go) | Application-level validation, timeout control, page-size caps, and limit caps. |
| [app/options.go](app/options.go) | Caller-owned runtime and service options. |
| [app/config.go](app/config.go) | Demo defaults. |

## Generate

From the repository root:

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json --check
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json --diff
```

To rewrite generated metadata:

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json
```

## Test

```bash
GOWORK=off go test -count=1 ./examples/production/...
```

## Runtime Ownership

The demo intentionally does not open a real database connection. Production applications should:

1. Import the concrete `database/sql` driver in the application binary or test harness.
2. Open and tune `*sql.DB`.
3. Run schema migrations outside this repository.
4. Call `app.Assemble` with caller-owned options.
5. Close the returned runtime during shutdown.
6. Close `*sql.DB` from the caller after ORM sessions are closed.

```go
runtime, err := app.Assemble(ctx, app.RuntimeOptions{
	DB:         db,
	ConfigPath: "examples/production/goark-orm-runtime.json",
})
if err != nil {
	return err
}
defer runtime.Close()
```

Full documentation is in [docs/production-demo.md](../../docs/production-demo.md).
