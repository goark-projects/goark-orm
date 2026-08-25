# Goark ORM

Goark ORM 是可独立使用的数据映射模块，同时也可以接入 Goark 生态。目标是提供 Go 原生实体映射、查询构建、事务集成和可测试数据访问边界。

## 当前状态

本仓库已落地第一版 ORM 元数据与生成器基础能力，当前尚未承诺稳定公共 API。已支持：

- 实体 `//goark-orm:entity` 与严格 `goark-orm` struct tag 解析。
- Mapper `//goark-orm:mapper`、`select`、`insert`、`update`、`delete` 方法注解扫描，注解 SQL 支持 `<script>` 动态节点和显式 SQL Provider。
- XML Mapper 静态语句、动态 SQL 基础节点、`bind`、`selectKey`、`databaseId`、`resultMap`、`constructor/idArg/arg`、`association`、`collection`、`extends`、`autoMapping`、`discriminator`、`columnPrefix`、`notNullColumn` 元数据和 namespace/类型一致性校验。
- XML 与注解在同一个 Mapper 接口中混用。
- 生成 `RegisterGoarkORMMetadata`、Mapper 实现、分页 Mapper 签名、Cursor/ResultHandler 流式 Mapper 签名、BaseMapper/Service 工厂和 `orm.Session` 调用代码。
- Mapper 接口支持本包内接口嵌入，生成期会展平公共方法并按当前 Mapper namespace 绑定 Statement。
- 独立 `goark-orm` CLI，可不安装 Goark 主 CLI 直接生成代码。
- `database/sql` Session、独立 `Configuration`、`Dialect`、`ExecutorType.SIMPLE/REUSE`、`#{name}` / `#{user.name}` 安全参数编译、MyBatis 风格 `param1` / `_parameter` / `list` 别名、生成主键回填和基础结果扫描。
- XML 动态 SQL 支持 `sql/include`、`bind`、`if`、`where`、`set`、`trim`、`foreach`、`choose/when/otherwise`。
- MyBatis-Plus 风格 `BaseMapper` 通用 CRUD、`QueryWrapper` 条件构造器和 `Page` 分页模型。
- MyBatis-Plus 风格 `Service`、`QueryChain` 和 `UpdateChain`，覆盖常用 `IService` / chain wrapper 操作。
- MyBatis-Plus 风格 `IDType` 主键策略：`AUTO`、`INPUT`、`ASSIGN_ID`、`ASSIGN_UUID`。
- `BaseMapper` 支持 `SelectCount`、`SelectMaps`、`SelectObjs`、`DeleteBatchIDs` 和 `SaveOrUpdate`。
- `BaseMapper` 已支持逻辑删除、`UpdateByID` 乐观锁、`created-at` / `updated-at` 自动时间字段。
- `QueryWrapper` / `UpdateWrapper` 支持嵌套条件、`EXISTS` / `NOT EXISTS`、`Apply`、`Last`、`Between`、`NotBetween`、`NotLike`、`LikeLeft`、`LikeRight`、`NotIn`，查询 Wrapper 额外支持 `GroupBy` / `Having` / `Select` / `AllEq` / 条件化 `OrderBy`。
- `UpdateWrapper`、`TypedField` 和生成期 `UserTypedFields` 字段常量，支持局部更新、类型化字段引用、`SetSQL`、`SetIncrBy` 和 `SetDecrBy`。
- Registry / Session 级 `TypeHandler` SPI，内建 `json`、`time`、`decimal` 处理器。
- `SQLSession` 执行器/StatementHandler/ParameterHandler/ResultSetHandler SPI、拦截器链，以及 BlockAttack、SQL Observer、租户条件、数据权限条件、动态表名、分页和实体语义内置拦截器。
- 独立 `SQLSessionFactory`、`Transaction`、`TransactionFactory`、`TxSession` 和 `InTx` 回调事务模型。
- `BatchSession` 批处理执行器、`Flush` / `Clear`、事务内批处理和查询前自动 flush。
- Session/Statement 级一级缓存配置，写操作、提交、回滚和关闭时自动失效。
- Mapper namespace 级二级缓存 SPI、默认内存 LRU 缓存、XML `<cache>` / `<cache-ref>`，以及 statement 级 `useCache` / `flushCache` 策略。
- ResultMap 支持 `constructor` 字段映射、显式/自动映射开关、内联 association/collection 扫描、`columnPrefix`、`notNullColumn`、collection 多行聚合、`discriminator` 运行期 case 分派，以及 `select` 嵌套查询 eager 和显式 lazy 回填；嵌套查询支持复合列参数并在单次父查询内复用相同参数结果，降低 N+1 重复查询。
- `ResultHandler`、`QueryCursor`、`QueryEach` 和 `RowCursor` 支持逐行流式查询；游标查询不写入一级缓存和二级缓存，并拒绝需要多行聚合的 collection resultMap。生成 Mapper 可直接声明 Cursor 返回或 ResultHandler 回调签名。
- `Lazy[T]` 和 `LazySlice[T]` 支持 `fetchType="lazy"` 的 Go 化显式延迟加载；业务必须调用 `Load(ctx)` 触发查询，不使用透明代理。
- 二级缓存采用 MyBatis 风格事务语义：自动提交直接生效，事务内查询缓存和写入失效都延迟到 `Commit` 后发布，`Rollback` 丢弃待发布变更。

Goark 生态集成后续按类似 `mybatis-spring` 的适配层推进，`goark-orm` core 不直接依赖 `goark/db`。

## 模块路径

```text
module goark.dev/orm
```

## 规划边界

- 实体映射、字段元数据和查询构建边界
- 独立 Session 生命周期、事务抽象、错误分类和上下文取消
- 未来通过独立适配层对接 `goark/db`
- 可生成、可测试、低反射的数据访问路径

## 非目标

- 不克隆 JPA/Hibernate 的运行时代理和复杂持久化上下文
- 不提交临时 SQL、迁移草稿或数据库私有脚本

## 快速检查

```bash
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

生成示例：

```bash
goark-orm generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
```

生成文件会包含实体字段常量、类型化字段常量、BaseMapper 工厂和 Service 工厂。业务代码可以直接使用字段常量，不需要手写列名：

```go
userMapper, err := NewUserBaseMapper(session)
if err != nil {
	return err
}

page, err := userMapper.SelectPage(
	ctx,
	orm.NewPageRequest(1, 20),
	orm.NewQueryWrapper[User]().
		Eq(UserFields.Status, "ACTIVE").
		OrderByAsc(UserFields.ID),
)
if err != nil {
	return err
}
_ = page
```

局部更新可以使用 `UpdateWrapper`：

```go
rows, err := userMapper.UpdateWithWrapper(
	ctx,
	orm.NewUpdateWrapper[User]().
		SetTyped(UserTypedFields.Name, "Alice").
		EqTyped(UserTypedFields.ID, int64(7)),
)
if err != nil {
	return err
}
_ = rows
```

Service 层可以直接对齐 MyBatis-Plus 常用业务写法：

```go
userService, err := NewUserService(session)
if err != nil {
	return err
}

users, err := userService.
	ChainQuery().
	Eq(UserFields.Status, "ACTIVE").
	OrderByDesc(UserFields.ID).
	List(ctx)
if err != nil {
	return err
}
_ = users
```

通用 SQLSession 可以按需启用 MP 风格插件：

```go
session, err := orm.NewSQLSession(
	registry,
	db,
	orm.NewPostgresDialect(),
	orm.WithInterceptors(
		orm.NewEntitySemanticInterceptor(registry),
		orm.NewDynamicTableInterceptor(map[string]string{"sys_user": "sys_user_2026"}),
		orm.NewTenantInterceptor("tenant_id", tenantID),
		orm.NewDataPermissionInterceptor(permissionProvider),
		orm.NewPaginationInterceptor(),
		orm.NewBlockAttackInterceptor(),
	),
)
if err != nil {
	return err
}

ctx = orm.WithPageRequest(ctx, orm.NewPageRequest(1, 20))
```

`SQLSession` 可通过独立配置对象统一设置运行期行为：

```go
config := orm.DefaultConfiguration().
	WithLocalCache(true).
	WithSecondLevelCache(true).
	WithMapUnderscoreToCamelCase(true)
config.Dialect = orm.NewPostgresDialect()
config.LocalCacheScope = orm.LocalCacheScopeSession
config.DefaultExecutorType = orm.ExecutorTypeReuse

session, err := orm.NewSQLSession(registry, db, nil, orm.WithConfiguration(config))
if err != nil {
	return err
}
```

大结果集可以使用游标逐条消费：

```go
err = orm.QueryEach[User](ctx, session, "system.user.UserMapper.ListAll", nil, func(ctx context.Context, user User) error {
	return exportUser(ctx, user)
})
if err != nil {
	return err
}
```

生成 Mapper 也可以直接声明流式查询签名：

```go
ListCursor(ctx context.Context, status string) (*orm.Cursor[User], error)
ListEach(ctx context.Context, status string, handler orm.ResultHandler[User]) error
```

注解 Mapper 可以使用 MyBatis 风格 `<script>`，复杂 SQL 也可以通过显式注册的 Provider 运行期生成：

```go
//goark-orm:select(sql="<script>select id, name from sys_user <where><if test=\"status != nil and status != ''\">status = #{status}</if></where></script>")
List(ctx context.Context, status string) ([]User, error)

//goark-orm:select(provider="UserSQL.ListByStatus")
ListByProvider(ctx context.Context, status string) ([]User, error)
```

```go
err := registry.RegisterSQLProvider("UserSQL.ListByStatus", func(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs) (orm.SQLSource, error) {
	return orm.SQLSource{SQL: "select id, name from sys_user where status = #{status}"}, nil
})
if err != nil {
	return err
}
```

MyBatis 风格事务可以通过 `SQLSessionFactory` 使用：

```go
factory, err := orm.NewSQLSessionFactory(registry, db, orm.NewPostgresDialect())
if err != nil {
	return err
}

err = factory.InTx(ctx, nil, func(ctx context.Context, session orm.Session) error {
	userMapper := NewUserMapper(session)
	_, err := userMapper.ListByStatus(ctx, "ACTIVE")
	return err
})
if err != nil {
	return err
}
```

批处理可以独立使用，也可以放入事务回调：

```go
err = factory.InTx(ctx, nil, func(ctx context.Context, session orm.Session) error {
	batch, err := orm.NewBatchSession(session)
	if err != nil {
		return err
	}
	userMapper := NewUserMapper(batch)
	if _, err := userMapper.UpdateStatus(ctx, int64(7), "LOCKED"); err != nil {
		return err
	}
	_, err = batch.Flush(ctx)
	return err
})
```

XML Mapper 可以声明 namespace 二级缓存：

```xml
<mapper namespace="system.user.UserMapper">
  <cache eviction="LRU" size="1024" flushInterval="60000"/>
  <select id="FindByID" useCache="true" flushCache="false">
    select id, name from sys_user where id = #{id}
  </select>
</mapper>
```

多个 Mapper 可以通过 `cache-ref` 共享同一个 namespace 缓存：

```xml
<cache-ref namespace="system.user.UserMapper"/>
```

生成 Mapper 可以声明分页签名：

```go
ListPage(ctx context.Context, status string, page orm.PageRequest) (orm.Page[User], error)
```

XML 嵌套查询可以使用显式 Lazy 字段：

```go
type Order struct {
	ID    int64
	User  orm.Lazy[User]
	Items orm.LazySlice[OrderItem]
}

user, err := order.User.Load(ctx)
items, err := order.Items.Load(ctx)
```

Mapper 公共接口可以通过 Go 接口嵌入复用：

```go
type UserQueryMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	UserQueryMapper
}
```

实体字段可通过 tag 开启通用 Mapper 语义：

```go
Version   int64     `goark-orm:"column='version';version=true"`
Deleted   bool      `goark-orm:"column='deleted';soft-delete=true"`
CreatedAt time.Time `goark-orm:"column='created_at';created-at=true"`
UpdatedAt time.Time `goark-orm:"column='updated_at';updated-at=true"`
```

Goark 主 CLI 不包含 ORM 子命令，也不依赖 `goark.dev/orm`。ORM 代码生成统一使用本仓库自带的独立 `goark-orm` 命令。

## 工程约定

- Go 版本跟随 Goark 生态主线，当前模块声明为 `go 1.25`。
- 代码、脚本、配置和文档统一使用 UTF-8 与 LF。
- Go 代码注释使用标准简体中文，只解释非显而易见的设计意图、边界和失败语义。
- 公共 API 优先显式接口、小结构体和可组合选项，不使用 Java 风格运行时扫描或重代理模型。
- 功能实现必须包含边界条件、错误处理、上下文取消和并发安全复核。

## 许可证

本项目使用 Apache License 2.0。
