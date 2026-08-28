# Goark ORM Documentation

This directory is the reference documentation set for Goark ORM. English is the default language. Chinese mirrors use the `*.zh-CN.md` suffix.

## Read First

- [Repository README](../README.md): project overview, quick start, runtime assembly, and verification commands.
- [Feature Reference](features.md): implemented runtime, generator, mapper, caching, routing, schema, and real database features.
- [Configuration Reference](configuration.md): every generator and runtime JSON field, accepted values, defaults, and ownership rules.
- [Examples Guide](examples.md): focused usage examples for generated mappers, XML mapping, wrappers, providers, runtime config, routing, audit, and real database verification.

## Operations

- [Database Matrix](database-matrix.md): dialect behavior, compatibility suite coverage, environment variables, and benchmark harnesses.
- [Release Gates](release-gates.md): local build, test, vet, generation, diff, and benchmark gates.
- [API Compatibility](api-compatibility.md): V1 public contracts and evolution rules.
- [Architecture Notes](goark-orm-v1-design.md): design boundaries, metadata flow, runtime package responsibilities, and key decisions.
- [Provider And SQL Builder](provider-builder.md): provider registration, builder APIs, cache keys, upsert, and row locks.

## Documentation Rules

- Public examples use `RuntimeConfig`, `RuntimeAssembly`, and `LoadAndAssembleRuntimeConfig`.
- Documentation does not store DSNs, passwords, private SQL, or generated environment files.
- Core examples keep concrete database driver imports in caller-owned test harnesses.
- The ORM remains Go-native: explicit metadata, generated registration, and small runtime interfaces.
