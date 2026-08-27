# Provider And SQL Builder

## Purpose

Providers hold runtime SQL construction logic that is too complex or too contextual for static SQL. Providers are registered explicitly in `Registry`; runtime scanning and proxy-based discovery are not part of the core design.

## Provider Registration

Function registration:

```go
err := registry.RegisterSQLProvider("UserSQL.List", provider)
```

Descriptor registration adds command and statement constraints:

```go
err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
	"UserSQL.List",
	provider,
	orm.WithSQLProviderCommands(orm.StatementCommandSelect),
	orm.WithSQLProviderStatements("example.user.UserMapper.List"),
))
```

Descriptor validation rejects a provider when it is invoked by an unsupported statement or command. The error is classified as `ErrBinding`.

After generated metadata and provider registrations are complete, call:

```go
if err := registry.Validate(); err != nil {
	return err
}
```

`Registry.Validate` verifies provider references, result maps, type handlers, cache refs, select-key metadata, and nested select references before the first SQL request.

## Provider Arguments

A provider may return `SQLSource.Args`. Provider arguments are merged with mapper arguments before type handling, raw token rendering, and placeholder compilation.

Merge rules:

- Argument names are trimmed.
- Empty names are rejected.
- A provider cannot silently overwrite a mapper argument with a different value.
- Values still use `#{}` for parameter binding.
- `${}` accepts only `RawSQLToken` values.

## Select Builder

```go
source, err := orm.NewSelectSQLBuilder().
	Select("id", "name", "status").
	From("sys_user").
	LeftJoin("sys_role", "sys_role.user_id = sys_user.id and sys_role.code = #{role}", orm.NamedArgs{"role": "admin"}).
	WhereEq("status", args["status"]).
	WhereIn("kind", args["kinds"]).
	WhereBetween("id", args["beginID"], args["endID"]).
	WhereIsNull("deleted_at").
	OrderByAsc("id").
	Limit(args["limit"]).
	ForUpdate(orm.NewPostgresDialect(), orm.RowLockOptions{SkipLocked: true}).
	CacheKey("tenant:" + tenantID).
	Build()
```

Table and column names are converted to safe raw identifier tokens. Values become named parameters and are compiled by the selected dialect.

## Write Builders

`NewInsertSQLBuilder`, `NewUpdateSQLBuilder`, and `NewDeleteSQLBuilder` return `SQLSource` values that can be used from providers. `UpdateSQLBuilder` and `DeleteSQLBuilder` can call `RequireWhere` to reject write statements without a WHERE clause.

```go
source, err := orm.NewUpdateSQLBuilder().
	Table("sys_user").
	Set("status", "LOCKED").
	WhereEq("id", int64(7)).
	RequireWhere().
	Build()
```

## Cache Key

`SQLSource.CacheKey` adds a business-specific cache dimension. Local and second-level caches already include final SQL and final arguments. Use a custom cache key when the same SQL and arguments can produce different results because of tenant, data-domain, or dynamic-table context.

## Dialect Helpers

```go
source, err := orm.BuildUpsertSQL(orm.NewPostgresDialect(), orm.UpsertSpec{
	Table:           "sys_user",
	InsertColumns:   []string{"id", "name", "status"},
	ConflictColumns: []string{"id"},
	UpdateColumns:   []string{"name", "status"},
	Values: orm.NamedArgs{
		"id":     int64(7),
		"name":   "Alice",
		"status": "ACTIVE",
	},
})
if err != nil {
	return err
}
_ = source

lockClause, err := orm.RowLockClause(orm.NewPostgresDialect(), orm.RowLockOptions{SkipLocked: true})
if err != nil {
	return err
}
_ = lockClause
```

These helpers generate SQL or execution plans only. They do not create schemas, run migrations, or import database drivers.
