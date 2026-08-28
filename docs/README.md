# Goark ORM Documentation

This directory is the reference documentation set for Goark ORM. English is the default language. Chinese mirrors use the `*.zh-CN.md` suffix.

## Read First

- [Repository README](../README.md): project overview, quick start, runtime assembly, and verification commands.
- [Feature Reference](features.md): implemented runtime, generator, mapper, caching, routing, schema, and real database features.
- [Configuration Reference](configuration.md): every generator field, runtime JSON field, Go-only assembly field, accepted value, default, and ownership rule.
- [Annotation, Tag, And XML Mapper Reference](annotations.md): every generator annotation, struct-tag attribute, XML mapper element, statement option, and dynamic SQL node.
- [Examples Guide](examples.md): focused usage examples for generated mappers, XML mapping, wrappers, providers, runtime config, routing, audit, and real database verification.
- [Production Demo](production-demo.md): production-oriented package layout with generator config, runtime config, mapper/provider code, service validation, and tests.

## Operations

- [Database Matrix](database-matrix.md): dialect behavior, compatibility suite coverage, environment variables, and benchmark harnesses.
- [Release Gates](release-gates.md): local build, test, vet, generation, diff, and benchmark gates.
- [API Compatibility](api-compatibility.md): V1 public contracts and evolution rules.
- [Architecture Notes](goark-orm-v1-design.md): design boundaries, metadata flow, runtime package responsibilities, and key decisions.
- [Provider And SQL Builder](provider-builder.md): provider registration, builder APIs, cache keys, upsert, and row locks.
- [Changelog](../CHANGELOG.md): released versions and unreleased documentation changes.
- [Example Workspace](../examples/README.md): example package map and verification commands.

## Documentation Rules

- Public examples use `RuntimeConfig`, `RuntimeAssembly`, and `LoadAndAssembleRuntimeConfig`.
- Documentation does not store DSNs, passwords, private SQL, or generated environment files.
- Core examples keep concrete database driver imports in caller-owned test harnesses.
- The ORM remains Go-native: explicit metadata, generated registration, and small runtime interfaces.
- Each documented capability must point back to source, generated examples, or package tests; speculative features stay out of public docs.
