# Provider 与 SQL Builder

## 目标

Provider 用于承载运行期复杂 SQL 组织逻辑。`goark-orm` 保持 Go 原生路线：Provider 必须显式注册，不做运行期扫描、不做代理、不做字符串魔法。

## Provider 描述

业务可以继续使用函数式注册：

```go
err := registry.RegisterSQLProvider("UserSQL.List", provider)
```

需要更强约束时使用描述式注册：

```go
err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
	"UserSQL.List",
	provider,
	orm.WithSQLProviderCommands(orm.StatementCommandSelect),
	orm.WithSQLProviderStatements("system.user.UserMapper.List"),
))
```

描述式注册会在执行前校验 Provider 是否允许服务当前 Statement，错误进入 `ErrBinding` 分类。应用启动完成元数据注册后，可以调用 `registry.Validate()` 或 `orm.ValidateRegistry(registry)` 提前校验 Provider、ResultMap、TypeHandler、cache-ref、selectKey 和 nested select 引用，避免首次 SQL 请求才暴露装配错误。

## SQLSource 参数

Provider 可以返回 `SQLSource.Args`。这些参数会和 Mapper 入参合并，再进入 TypeHandler、`${}` 安全 token 渲染和 `#{}` 占位符编译。

合并规则：

- 参数名会去除首尾空白。
- 空参数名会拒绝。
- 和 Mapper 入参同名且值不同会拒绝，避免 Provider 静默覆盖业务参数。
- SQL 最终仍只通过 `#{}` 绑定值，通过 `${}` 绑定 `RawSQLToken`。

## SQL Builder

`NewSelectSQLBuilder`、`NewInsertSQLBuilder`、`NewUpdateSQLBuilder` 和 `NewDeleteSQLBuilder` 会返回可直接用于 Provider 的 `SQLSource`。

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

Builder 的表名和列名会转成 `${}` 安全标识符 token，最终由方言决定引号风格；值会转成 `#{}` 参数，占位符由方言编译。写语句 Builder 支持 `Returning`，`UpdateSQLBuilder` 和 `DeleteSQLBuilder` 可以通过 `RequireWhere` 显式拒绝无 WHERE 写语句。

## 缓存 Key

`SQLSource.CacheKey` 是 Provider 额外缓存维度。一级缓存和二级缓存仍会包含最终 SQL 与最终参数；当 SQL 和参数相同但业务上下文不同，例如租户、数据域、动态表策略时，可以设置 `CacheKey` 避免缓存串用。

## 方言 SQL 辅助

方言能力矩阵已提供可调用 API：

- `BuildUpsertSQL`：按方言生成 PostgreSQL/SQLite `ON CONFLICT` 或 MySQL/MariaDB `ON DUPLICATE KEY UPDATE`。
- `RowLockClause`：按方言生成 `FOR UPDATE`、`SKIP LOCKED`、`NOWAIT` 或 SQL Server 锁提示。
- `NewGeneratedKeyPlan`：返回当前方言的主键回读计划。

这些 API 只生成 SQL 或执行计划，不负责建表、迁移、DDL 生命周期，也不引入具体数据库驱动。
