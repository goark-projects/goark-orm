# Provider 与 SQL Builder

## 目的

Provider 承载过于复杂或强依赖上下文、无法只靠静态 SQL 表达的运行期 SQL 构造逻辑。Provider 在 `Registry` 中显式注册；运行期扫描和代理式发现不属于 core 设计。

## Provider 注册

函数注册：

```go
err := registry.RegisterSQLProvider("UserSQL.List", provider)
```

Descriptor 注册可附加 command 和 statement 约束：

```go
err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
	"UserSQL.List",
	provider,
	orm.WithSQLProviderCommands(orm.StatementCommandSelect),
	orm.WithSQLProviderStatements("example.user.UserMapper.List"),
))
```

当 Provider 被不支持的 statement 或 command 调用时，descriptor validation 会拒绝执行。该错误分类为 `ErrBinding`。

生成元数据和 Provider 注册完成后，调用：

```go
if err := registry.Validate(); err != nil {
	return err
}
```

`Registry.Validate` 会在第一次 SQL 请求前校验 Provider 引用、result map、type handler、cache ref、select-key 元数据和 nested select 引用。

## Provider 参数

Provider 可以返回 `SQLSource.Args`。Provider 参数会在 type handling、raw token rendering 和 placeholder compilation 之前与 Mapper 参数合并。

合并规则：

- 参数名会被 trim。
- 空参数名会被拒绝。
- Provider 不能静默覆盖值不同的 Mapper 参数。
- 值仍使用 `#{}` 做参数绑定。
- `${}` 只接受 `RawSQLToken` 值。

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

表名和列名会转换为安全 raw identifier token。值会变成命名参数，并由所选方言编译。

## 写入 Builder

`NewInsertSQLBuilder`、`NewUpdateSQLBuilder`、`NewDeleteSQLBuilder` 返回可从 Provider 使用的 `SQLSource`。`UpdateSQLBuilder` 和 `DeleteSQLBuilder` 可以调用 `RequireWhere`，拒绝没有 WHERE 条件的写语句。

```go
source, err := orm.NewUpdateSQLBuilder().
	Table("sys_user").
	Set("status", "LOCKED").
	WhereEq("id", int64(7)).
	RequireWhere().
	Build()
```

## Cache Key

`SQLSource.CacheKey` 添加业务特定缓存维度。一级缓存和二级缓存已经包含最终 SQL 和最终参数。当同一 SQL 与参数仍可能因为租户、数据域或动态表上下文产生不同结果时，使用自定义 cache key。

## 方言辅助函数

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

这些辅助函数只生成 SQL 或执行计划。它们不创建 schema、不执行 migration，也不导入数据库驱动。
