# Goark ORM Changelog

English is the default changelog language. The Chinese mirror is maintained in [CHANGELOG.zh-CN.md](CHANGELOG.zh-CN.md).

## [Unreleased]

### Documentation

- Reworked the documentation system around English-first bilingual guides.
- Added dedicated annotation, struct-tag, XML mapper, and dynamic SQL references.
- Added bilingual README files for the example workspace, minimal example, provider example, and production demo.
- Expanded production-demo documentation to show configuration, runtime assembly, service boundaries, validation, and verification commands.
- Removed inline credential-shaped DSN examples from public docs; real database runs now document environment-variable injection without storing sample secrets.

## [v0.0.1] - 2026-08-28

### Release Positioning

`v0.0.1` is the first public Go module release of Goark ORM. The module path is `goark.dev/orm`.

This release targets production Go service data-access layers. It uses explicit metadata, deterministic code generation, `database/sql`, small runtime contracts, and low-reflection execution paths. The runtime does not depend on Goark core, boot, or CLI packages. Applications keep ownership of database drivers, connection pools, transaction boundaries, schema migrations, and deployment configuration.

### Added

- Entity and mapper declarations through `//goark-orm:entity`, `//goark-orm:mapper`, and strict `goark-orm` struct tags.
- Annotation mappers, XML mappers, provider SQL, and the unified `StatementMeta` execution model.
- Dynamic SQL nodes: `sql/include`, `bind`, `if`, `where`, `set`, `trim`, `foreach`, and `choose/when/otherwise`.
- `BaseMapper`, `Service`, chain query/update APIs, typed wrappers, pagination, field-value queries, and ID-list helpers through `dbkit`.
- Batch sessions, transaction sessions, routing sessions, cursor streaming, and explicit lazy loading.
- ResultMap constructor arguments, associations, collections, discriminators, nested selects, and multi-result-set mapping.
- Session local cache, mapper-namespace second-level cache, LRU, TTL, blocking miss coalescing, and transaction-aware cache publication.
- Type handlers, SQL providers, statement interceptors, handler middleware, audit middleware, and SQL observation hooks.
- Dialects for PostgreSQL, MySQL, MariaDB, SQLite, SQL Server, Oracle, and the question-placeholder SQL generation dialect.
- `ormgen` generation, schema reverse engineering, schema drift detection, reusable real-database compatibility suites, and benchmark harnesses.

### Architecture

- The root package `goark.dev/orm` is the stable public facade.
- Runtime implementation lives under `goark.dev/orm/internal/runtime`; external code must not depend on that internal layout.
- `audit`, `dbkit`, `ormboot`, `ormgen`, and `ormtest` are independent subpackages with narrow responsibilities.
- Public identity is exposed through `ModulePath = "goark.dev/orm"` and `APIVersion = "v1"`.
- Generated mappers depend on public session interfaces, so auto-commit, transaction, batch, routing, and streaming flows share the same generated surface.

### Engineering Quality

- Dynamic SQL expressions use cached plans and avoid per-execution parsing.
- SQL placeholder compilation, SQL tail scanning, ResultMap fallback, association mapping, and query scanning paths are covered by focused benchmarks.
- The `REUSE` executor uses a bounded prepared-statement cache to avoid unlimited final-SQL retention in long-lived sessions.
- The generator supports build-tag-aware scanning, atomic writes, unchanged-file skipping, `--check`, and `--diff`.

### Verified

The release was validated with:

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go test -race -count=1 ./...
GOWORK=off go vet ./...
gofmt -l .
git diff --check
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --check
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --diff
powershell -ExecutionPolicy Bypass -File scripts\verify-bench.ps1 -EnforceTime
powershell -ExecutionPolicy Bypass -File scripts\verify-real-db.ps1
```

The release host did not retain private database credentials. Real database acceptance should be rerun by configuring the DSN variables documented in [docs/database-matrix.md](docs/database-matrix.md).

### Install

```bash
go get goark.dev/orm@v0.0.1
```

### Known Boundaries

- Goark ORM does not manage schema migrations or DDL lifecycle.
- Core packages do not import concrete database drivers.
- Dynamic SQL expressions are a safe Go-native subset, not full OGNL.
- `${}` raw SQL substitution accepts only explicit `RawSQLToken` values.
- `v0.0.x` is pre-1.0; the public surface is organized around the V1 compatibility policy but can still evolve before `v1.0.0`.
