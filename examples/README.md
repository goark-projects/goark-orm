# Goark ORM Examples

English is the default language. The Chinese mirror is [README.zh-CN.md](README.zh-CN.md).

The examples are real Go packages used by tests and generator checks. They do not store DSNs, private SQL, credentials, or database driver imports in the core module.

## Example Map

| Path | Purpose | Verification |
| --- | --- | --- |
| [minimal](minimal) | Smallest generated mapper and entity contract. | `GOWORK=off go run ../cmd/goark-orm generate orm --dir minimal --check` from this directory, or the root release gate. |
| [provider](provider) | Provider and SQL builder usage through package tests. | `GOWORK=off go test -count=1 ./examples/provider` from the repository root. |
| [production](production) | Production-oriented account module and application assembly demo. | `GOWORK=off go test -count=1 ./examples/production/...` from the repository root. |

## Boundaries

- Examples import `goark.dev/orm` packages only.
- Concrete database drivers belong in caller-owned binaries or test harnesses.
- Schema migrations and DDL lifecycle stay outside the examples.
- Generated files are committed so `--check` can detect stale metadata.

## Read Next

- [docs/examples.md](../docs/examples.md)
- [docs/production-demo.md](../docs/production-demo.md)
- [docs/annotations.md](../docs/annotations.md)
