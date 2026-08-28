# Annotation, Tag, And XML Mapper Reference

English is the default language. The Chinese mirror is [annotations.zh-CN.md](annotations.zh-CN.md).

This guide is the mapping contract reference for generator inputs. Runtime code never scans Go files or XML files; `ormgen` reads them at generation time and emits deterministic Go metadata registration code.

## Source Model

| Source | Scope | Runtime effect |
| --- | --- | --- |
| `//goark-orm:*` comments | Go types and mapper methods | Parsed by `ormgen`; converted into generated `EntityMeta`, `MapperMeta`, and `StatementMeta`. |
| `goark-orm` struct tags | Entity fields | Parsed by `ormgen`; converted into generated `ColumnMeta`. |
| XML mapper files | Mapper-level result maps, SQL, dynamic SQL, cache metadata | Parsed by `ormgen`; embedded into generated metadata. |
| Generated Go file | Package-level registration functions and typed helpers | Used by runtime through explicit `RegisterGoarkORMMetadata` calls. |

The generator sorts entities and mappers deterministically. Mapper namespaces must be explicit and globally unique.

## Annotation Syntax

Annotations use line comments with the `goark-orm:` prefix:

```go
//goark-orm:entity(table="sys_user")
//goark-orm:mapper(namespace="example.user.UserMapper", xml="mapper/user_mapper.xml")
//goark-orm:select(sql="select id from sys_user where id = #{id}")
```

Rules:

- The name is required after `goark-orm:`.
- Arguments are optional and use `key=value` inside parentheses.
- Arguments are comma-separated.
- Double-quoted values are unquoted with Go string literal rules.
- Duplicate argument keys fail generation.
- A malformed argument list fails generation.

## Entity Annotation

| Annotation | Scope | Attributes | Required |
| --- | --- | --- | --- |
| `//goark-orm:entity` | struct type | `table`, `keySequence`, `key-sequence` | `table` unless generator naming derives it |

Example:

```go
//goark-orm:entity(table="sys_user", keySequence="sys_user_id_seq")
type User struct {
	ID int64 `goark-orm:"column='id';primary-key=true;id-type='ASSIGN_ID'"`
}
```

Rules:

- The target type must be a struct.
- Every persisted field must declare a `goark-orm` tag.
- At least one primary key field is required.
- `keySequence` and `key-sequence` are aliases.

## Mapper Annotation

| Annotation | Scope | Attributes | Required |
| --- | --- | --- | --- |
| `//goark-orm:mapper` | interface type | `namespace`, `xml` | `namespace` |

Example:

```go
//goark-orm:mapper(namespace="example.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
```

Rules:

- The target type must be an interface.
- `namespace` must be explicit and globally unique.
- When `xml` is present, the XML root namespace must exactly match the Go mapper namespace.
- Each mapper method must resolve to exactly one statement source: annotation SQL/provider or XML.
- XML statements must match mapper method names; unused XML statements fail generation.
- Embedded named interfaces are supported; cyclic embedding and duplicate embedded methods fail.

## Method SQL Annotations

| Annotation | Command | Required source | Common attributes |
| --- | --- | --- | --- |
| `//goark-orm:select` | `select` | `sql` or `provider` | `statementType`, `affectData`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, `parameters`, `resultSets` |
| `//goark-orm:insert` | `insert` | `sql` or `provider` | common attributes plus `useGeneratedKeys`, `keyProperty` |
| `//goark-orm:update` | `update` | `sql` or `provider` | common attributes |
| `//goark-orm:delete` | `delete` | `sql` or `provider` | common attributes |
| `//goark-orm:call` | `call` | `sql` or `provider` | common attributes plus callable parameter/result-set metadata |

Statement source rules:

- `sql` and `provider` are mutually exclusive.
- One method cannot declare more than one SQL annotation.
- Annotation SQL and XML SQL cannot both define the same mapper method.
- Annotation SQL can contain `<script>...</script>` dynamic SQL.
- `provider` references a registered SQL provider descriptor or function name.

Statement option attributes:

| Attribute | Type | Description |
| --- | --- | --- |
| `statementType` | string | `PREPARED` or `CALLABLE`; `call` defaults to `CALLABLE`. |
| `timeout` | duration or integer seconds | Statement timeout metadata; must be non-negative. |
| `timeoutDuration` | duration or integer seconds | Alias with priority over `timeout`. |
| `fetchSize` | integer | Fetch hint; must be non-negative. |
| `resultSetType` | string | `DEFAULT`, `FORWARD_ONLY`, `SCROLL_INSENSITIVE`, or `SCROLL_SENSITIVE`. |
| `resultOrdered` | bool | ResultMap hint for ordered nested result rows. |
| `keyColumn` | string | Generated-key readback column. |
| `affectData` | bool | Marks a `select` as data-affecting for cache/audit behavior. |
| `useGeneratedKeys` | bool | Enables generated key behavior for write statements. |
| `keyProperty` | string | Entity or argument property receiving generated key data. |
| `interceptorIgnore` | list | Comma, semicolon, or whitespace separated interceptor names to skip. |

Callable attributes:

| Attribute | Syntax | Description |
| --- | --- | --- |
| `parameters` | `name[:mode[:jdbcType[:typeHandler]]]` list | Declares callable parameters. `mode` accepts `IN`, `OUT`, or `INOUT`; default is `IN`. |
| `out` | name list | Marks parameters as `OUT`. |
| `inout` | name list | Marks parameters as `INOUT`. |
| `resultSets` | `name[:resultType[:resultMap]]` list | Declares named result sets in returned order. |

Method signature rules:

- The first parameter must be `context.Context`.
- All parameters must be named.
- `orm.PageRequest` parameters are recognized for paged selects.
- `orm.ResultHandler[T]` parameters are recognized for streaming callbacks and require the method to return only `error`.
- `select` returns `(T, error)`, `([]T, error)`, `(orm.Page[T], error)`, `(*orm.Cursor[T], error)`, or uses `orm.ResultHandler[T]`.
- `insert`, `update`, and `delete` return `(int64, error)`.
- `call` returns `error` or `(orm.CallResult, error)`.
- `OUT` and `INOUT` callable parameters must map to pointer method parameters.
- Callable result sets must map to pointer-to-slice method parameters when the method exposes them.

## Struct Tag Syntax

Struct tags use the `goark-orm` key:

```go
Name string `goark-orm:"column='name';size=64;nullable=false;insert-strategy='not-empty'"`
```

Rules:

- Attributes are separated by semicolons.
- Every attribute must use `key=value`.
- Empty attributes fail generation.
- Duplicate attributes fail generation.
- Unsupported attributes fail generation.
- String values must use single quotes.
- Boolean values must be exactly `true` or `false`.
- Integer values must be decimal integers.

## Struct Tag Attributes

| Attribute | Type | Effect |
| --- | --- | --- |
| `column` | string | Database column name. Required unless generator naming derives it. |
| `type` | string | Database type metadata used by generated schema models and compatibility checks. |
| `default` | string | Column default metadata. |
| `id-type` | string | Primary key strategy: `auto`, `input`, `assign_id`, `assign_uuid`, `none`, or empty. |
| `fill` | string | Auto-fill timing: `insert`, `update`, or `insert_update`. |
| `type-handler` | string | Named type handler for this field. The name must be registered or accepted by generation config. |
| `key-column` | string | Database generated-key readback column. |
| `update` | string | Custom update expression. |
| `update-expression` | string | Alias for custom update expression; cannot be used together with `update`. |
| `condition` | string | Custom condition template for wrapper-generated SQL. |
| `insert-strategy` | string | Field inclusion strategy for generated inserts. |
| `update-strategy` | string | Field inclusion strategy for generated updates. |
| `where-strategy` | string | Field inclusion strategy for generated where clauses. |
| `primary-key` | bool | Marks a primary key field. |
| `auto-increment` | bool | Marks a database-generated primary key; requires `primary-key=true`. |
| `nullable` | bool | Column nullability metadata. |
| `select` | bool | `false` removes the field from default select lists. |
| `version` | bool | Optimistic-lock version field; at most one per entity. |
| `soft-delete` | bool | Logical-delete marker field; at most one per entity. |
| `created-at` | bool | Created-at metadata; at most one per entity. |
| `updated-at` | bool | Updated-at metadata; at most one per entity. |
| `order-by` | bool | Adds the field to generated default ordering metadata. |
| `order-desc` | bool | Makes the generated default order descending. |
| `transient` | bool | Excludes the field from persistence metadata. |
| `size` | int | Character or binary size metadata. |
| `numeric-scale` | int | Decimal scale metadata. |
| `order-priority` | int | Sort priority for generated default ordering metadata. |

Validation rules:

- `auto-increment=true` requires `primary-key=true`.
- `id-type` requires `primary-key=true`.
- `auto-increment=true` conflicts with `id-type` values other than empty, `none`, or `auto`.
- An entity must have one or more primary keys.
- `version`, `soft-delete`, `created-at`, and `updated-at` allow at most one field each.

## XML Mapper Root

```xml
<mapper namespace="example.user.UserMapper">
  ...
</mapper>
```

Rules:

- The root element must be `mapper`.
- `namespace` is required.
- Only supported child elements are accepted.
- XML includes are expanded at generation time.
- Missing includes, duplicate statement ids with the same database specificity, and include cycles fail generation.

## XML Cache Elements

| Element | Attributes | Description |
| --- | --- | --- |
| `cache` | `eviction`, `size`, `flushInterval`, `readOnly`, `blocking` | Enables mapper namespace second-level cache. |
| `cache-ref` | `namespace` | Reuses another namespace cache. |

Boolean XML attributes must be `true` or `false`. Numeric XML attributes must be base-10 integers.

## XML Result Maps

| Element | Attributes | Description |
| --- | --- | --- |
| `resultMap` | `id`, `type`, `extends`, `autoMapping` | Declares a result mapping for one result object. |
| `constructor` | none | Holds `idArg` and `arg` mappings. |
| `idArg`, `arg` | `name`, `property`, `column`, `typeHandler` | Constructor argument mapping. |
| `id`, `result` | `property`, `column`, `typeHandler` | Scalar property mapping. |
| `association` | `property`, `type`, `javaType`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | Nested object mapping, nested select mapping, or named-result-set mapping. |
| `collection` | `property`, `ofType`, `type`, `javaType`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | Nested collection mapping. |
| `discriminator` | `column`, `type`, `javaType`, `typeHandler` | Branch selector. |
| `case` | `value`, `resultMap`, `resultType`, `type` | Discriminator branch, optionally with inline child mappings. |

Rules:

- `resultMap.id` is required and unique within the mapper.
- `extends` resolves mapper-local result maps and detects cycles.
- Parent result-map fields are merged before child fields.
- `notNullColumn` is a comma-separated list.
- `resultSet` and `foreignColumn` map nested objects from named multi-result sets.
- `select` and `fetchType` describe explicit nested selects and lazy-loading metadata.

## XML Statement Elements

| Element | Command | Attributes |
| --- | --- | --- |
| `select` | `select` | `id`, `resultMap`, `resultType`, `parameterType`, `databaseId`, `affectData`, `useCache`, `flushCache`, `statementType`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, `resultSets` |
| `insert` | `insert` | common attributes plus `useGeneratedKeys`, `keyProperty` |
| `update` | `update` | common attributes |
| `delete` | `delete` | common attributes |
| `call` | `call` | common attributes, defaults `statementType` to `CALLABLE` |

Rules:

- `id` is required.
- `resultMap` and `resultType` are mutually exclusive.
- `databaseId` selects database-specific statements during generation. Exact matches beat default statements; duplicate specificity fails.
- `useCache` and `flushCache` accept `true` or `false` and become explicit statement cache policies.
- `timeout` and `timeoutDuration` accept Go durations or integer seconds.
- `fetchSize` must be non-negative.
- `interceptorIgnore` accepts comma, semicolon, or whitespace separated names.

Nested statement metadata:

| Element | Attributes | Description |
| --- | --- | --- |
| `selectKey` | `keyProperty`, `resultType`, `order` | Generated-key query. `order` accepts `BEFORE` or `AFTER`; default is `AFTER`. |
| `parameter` | `property`, `name`, `mode`, `jdbcType`, `type`, `typeHandler` | Callable parameter. `property` and `name` are aliases; `type` aliases `jdbcType`. |
| `resultSet` | `name`, `property`, `resultMap`, `resultType` | Callable result-set metadata. |

## Dynamic SQL Nodes

Dynamic SQL is accepted in XML statements and annotation `<script>` blocks.

| Node | Attributes | Behavior |
| --- | --- | --- |
| `sql` | `id` | Declares an XML fragment under `mapper`; expanded by `include` during generation. |
| `include` | `refid`, `refId` | Inserts a named `sql` fragment. |
| `if` | `test` | Renders children when the expression is true. |
| `where` | none | Adds `WHERE` and removes leading boolean connectors. |
| `set` | none | Adds `SET` and removes trailing commas. |
| `trim` | `prefix`, `suffix`, `prefixOverrides`, `suffixOverrides` | Generic prefix/suffix wrapper. |
| `foreach` | `collection`, `item`, `index`, `open`, `close`, `separator`, `nullable` | Expands slices, arrays, maps, or supported collections. |
| `choose` | none | Renders first matching `when`, otherwise `otherwise`. |
| `when` | `test` | Conditional branch under `choose`. |
| `otherwise` | none | Fallback branch under `choose`. |
| `bind` | `name`, `value` | Creates a named value from the safe expression engine. |

The expression engine is deterministic and intentionally bounded. It supports boolean logic, comparisons, arithmetic, membership, ternary values, literals, list literals, parameter paths, and built-in collection/string helpers. It cannot call arbitrary Go functions and cannot mutate values.

## SQL Placeholders

| Placeholder | Behavior |
| --- | --- |
| `#{name}` | Compiled into the selected dialect placeholder and bound as a driver parameter. |
| `${name}` | Rendered only when the value implements `RawSQLToken`. Plain strings are rejected. |

Use `NewRawIdentifier`, `NewRawOrderItem`, and `NewRawOrderBy` to create safe raw SQL tokens for known identifier/order-by use cases.

## Generated Output Contract

The default generated file name is `zz_goark_orm_<package>_gen.go`.

Generated packages include:

- `RegisterGoarkORMMetadata(registry *orm.Registry) error`
- Mapper constructors such as `NewUserMapper(session orm.Session) UserMapper`
- BaseMapper constructors for entities with supported primary-key metadata
- Service constructors for generated BaseMappers
- Entity field constants and typed fields for wrappers and field-value helpers
- Row scanners for fast generated entity mapping

Regenerate after changing annotations, struct tags, XML mapper files, provider references, or generator configuration.
