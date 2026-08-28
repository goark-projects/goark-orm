# Provider Example

English is the default language. The Chinese mirror is [README.zh-CN.md](README.zh-CN.md).

This package demonstrates provider-driven SQL construction. Providers are used when SQL depends on runtime context, feature flags, tenant routing, or computed query shapes that are awkward to encode as static mapper SQL.

## Files

| File | Responsibility |
| --- | --- |
| [provider_test.go](provider_test.go) | Executable examples for provider registration, SQL builder output, and validation behavior. |

## What It Covers

- `Registry.RegisterSQLProviderDescriptor`.
- Provider command and statement constraints.
- `SQLSource` with generated SQL, named arguments, and cache keys.
- Builder validation for missing table, missing columns, and unsafe write statements.
- Dialect-specific placeholder compilation through the runtime.

## Test

From the repository root:

```bash
GOWORK=off go test -count=1 ./examples/provider
```

Read the full provider reference in [docs/provider-builder.md](../../docs/provider-builder.md).
