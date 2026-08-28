# Goark ORM V1 架构说明

## 状态

当前 V1 实现已接受。

## 背景

Goark ORM 是独立的 Go 数据映射模块。运行期使用确定性生成元数据和显式注册。源码文件、XML mapper 文件和 struct tag 是生成器输入；运行期不扫描这些文件。

## 目标

- Mapper namespace 在 registry 内保持显式且全局唯一。
- 使用一套元数据模型支持 annotation SQL、XML SQL、provider SQL、生成通用 CRUD、batch execution、streaming query、callable statements、transactions、routing sessions 和 cache behavior。
- 保持热路径低反射。存在生成 row scanner 时优先使用；受控 fallback path 处理高级 result map 和 type-handler 场景。
- 数据库驱动注册保留在 core packages 之外。
- Schema 生命周期保留在 ORM 边界之外。
- Go API 保持显式、小型、可测试。
- JSON 处理保持在内部 Sonic-backed codec 路径上。

## 非目标

- 不做运行期 Mapper 扫描。
- 不做运行期 XML 扫描。
- 不做运行期实体建模。
- 不做透明 lazy-loading proxies。
- 不提供带隐式 dirty checking 的 persistent context。
- 不生成 migration，不管理 DDL 生命周期。
- Core packages 不导入具体数据库驱动。
- `goark.dev/orm` core 不依赖 Goark core、boot 或 CLI packages。

## 数据流

```text
goark-orm generate orm
        |
        |-- scan //goark-orm:entity and goark-orm struct tags
        |-- scan //goark-orm:mapper and method SQL annotations
        |-- parse XML mapper files
        |-- validate mapper, entity, statement, result map, and parameter contracts
        v
zz_goark_orm_<package>_gen.go
        |
        |-- RegisterGoarkORMMetadata
        |-- EntityMeta / MapperMeta / StatementMeta
        |-- generated RowScanner functions
        |-- generated Mapper implementations
        |-- BaseMapper and Service factories
        v
Registry
        |
        v
Session / Executor / Dialect / TypeHandler / Interceptor / Cache
        |
        v
database/sql
```

## 注解和 Tag 契约

注解使用 `//goark-orm:xxx` 前缀：

```go
//goark-orm:mapper(namespace="example.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface{}
```

实体字段使用 Go struct tag key `goark-orm`：

```go
ID int64 `goark-orm:"column='id';primary-key=true;auto-increment=true"`
```

规则：

- Mapper namespace 必填。
- Mapper namespace 必须显式声明。
- Mapper namespace 在 registry 内必须唯一。
- Entity table name 必填，除非由生成器命名规则推导。
- Persistent fields 需要 `column` tag，除非由生成器命名规则推导。
- Non-persistent fields 需要 `transient=true`。
- 被映射实体至少需要一个 primary key，才能生成通用 CRUD。
- SQL 方法注解互斥：`select`、`insert`、`update`、`delete` 或 `call`。
- 同一方法不能同时绑定 XML 和 annotation SQL。

## 运行期包

| 区域 | 职责 |
| --- | --- |
| dialect | SQL placeholders、identifier quoting、pagination、capability metadata |
| metadata | Entity、mapper、statement、result map、cache 和 dynamic SQL metadata |
| statement | SQL source、provider、raw token、dynamic SQL 和 parameter binding |
| executor | query、query-one、exec、callable、batch 和 result mapping |
| session | SQLSession、factory、transaction session、routing session 和 caches |
| typehandler | JSON、time、decimal、string、bool、bytes 和 custom conversion SPI |
| interceptor | pagination、tenant、data permission、dynamic table、guard、read-only、observer |
| ormgen | source scanning、XML parsing、model validation、rendering、schema tools |
| ormtest | 调用方拥有的真实数据库测试套件 |

## 运行期配置

`Configuration` 是直接运行期配置模型。它控制 dialect、database id、cache defaults、local cache scope、underscore-to-camel mapping、default executor type、default statement timeout、fetch size 和全局实体行为。

JSON 配置加载器是严格模式，并通过内部 Sonic-backed JSON codec 解码。`AssembleRuntimeConfig` 保持装配显式：调用方传入 registry、可选 database handle、named type handlers、custom plugins 和 session options。它会在创建 session factory 之前校验配置、type handler names、mapper namespaces 和 registry metadata。

## 事务

`SQLSessionFactory` 基于 `database/sql` 创建 auto-commit sessions 和 transaction sessions。生成 Mapper 只要求 `orm.Session`，因此 transaction sessions 和 batch sessions 不需要生成额外 Mapper 代码即可使用。

```go
err := factory.InTx(ctx, nil, func(ctx context.Context, session orm.Session) error {
	baseMapper, err := NewUserBaseMapper(session)
	if err != nil {
		return err
	}
	_, err = baseMapper.UpdateWithWrapper(
		ctx,
		orm.NewUpdateWrapper[User]().
			SetTyped(UserTypedFields.Status, "LOCKED").
			EqTyped(UserTypedFields.ID, int64(7)),
	)
	return err
})
```

`BatchSession` 按顺序排队写语句，按顺序 flush，并在读取前自动 flush。

## 缓存

Local cache 默认 scoped to session，并在 write、commit、rollback 和 close 时失效。Statement scope 可用于单语句缓存。

Second-level cache 按 namespace 分区。它支持 LRU eviction、TTL expiration、blocking miss coalescing、cache references 和 stats snapshots。Transaction sessions 只在 commit 后发布 cache writes 和 invalidations；rollback 会丢弃 pending cache changes。

## 结果映射

简单实体映射优先使用生成 row scanner。Result maps 覆盖 constructor mapping、associations、collections、discriminator cases、nested selects、column prefixes、not-null guards 和显式 lazy loading。高级 result maps 使用受控 fallback paths，因为它们需要运行期聚合或 nested query 行为。

## Interceptors 和 Middleware

`StatementInterceptor` 在动态 SQL 渲染后、方言 placeholder 编译前包装 SQL。内置 interceptors 包括：

- 全表写保护
- SQL observation
- tenant condition 和 insert-field injection
- data permission condition injection
- dynamic table name rewriting
- pagination
- entity semantic rewriting
- SQL governance rules
- illegal SQL protection
- read-only protection

Middleware contracts 允许用 decorator 风格扩展 statement execution、statement handling、parameter handling 和 result set handling。

## 路由

`RoutingSession` 将生成 Mapper 调用委派到命名 sessions。解析顺序：

1. 显式 `WithDataSource(ctx, key)`
2. 配置的 `DataSourceResolver`
3. 默认 data source

路由层不创建跨数据库事务语义。原子工作必须使用单一数据源事务或外部 transaction coordinator。

## Schema Tools

`ormgen.SQLSchemaIntrospector` 通过已经注册的 `database/sql` 连接读取数据库元数据。反向工程会构造 package models 并渲染 Go source。Drift helpers 对比 registry entity metadata 和 schema model。`ValidateSQLSchemaCompatibility` 串联 introspection、model build、render smoke 和可选 drift validation。

## 关键决策

| 决策 | 结果 |
| --- | --- |
| Mapper namespace | 显式且全局唯一 |
| Runtime metadata | 生成并显式注册 |
| Runtime discovery | 不扫描 mapper、XML 或 entity |
| Database drivers | 只由调用方或 test harness 导入 |
| Migrations | 在 ORM core 之外 |
| Raw SQL substitution | 限制为显式安全 token |
| JSON codec | 内部 codec，基于 ByteDance Sonic |
| Goark ecosystem adapter | 不属于 core runtime |

## 验证

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
git diff --check
```
