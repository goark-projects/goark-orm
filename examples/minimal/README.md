# Minimal Example

English is the default language. The Chinese mirror is [README.zh-CN.md](README.zh-CN.md).

This package shows the smallest useful Goark ORM workflow:

- Declare an entity with `//goark-orm:entity`.
- Declare a mapper interface with an explicit namespace.
- Use annotation SQL for mapper methods.
- Commit generated metadata in `zz_goark_orm_minimal_gen.go`.
- Verify generated output with `--check` and package tests.

## Files

| File | Responsibility |
| --- | --- |
| [mapper.go](mapper.go) | Entity, Mapper interface, annotation SQL, and field tags. |
| [zz_goark_orm_minimal_gen.go](zz_goark_orm_minimal_gen.go) | Generated metadata, mapper implementation, field helpers, and row scanners. |
| [minimal_test.go](minimal_test.go) | Smoke tests for generated metadata and mapper behavior. |

## Generate

From the repository root:

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --check
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --diff
```

To rewrite the generated file:

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --output examples/minimal/zz_goark_orm_minimal_gen.go
```

## Test

```bash
GOWORK=off go test -count=1 ./examples/minimal
```
