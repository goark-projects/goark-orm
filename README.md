# Goark ORM

Goark ORM 是可独立使用的数据映射模块，同时也可以接入 Goark 生态。目标是提供 Go 原生实体映射、查询构建、事务集成和可测试数据访问边界。

## 当前状态

本仓库已落地第一版 ORM 元数据与生成器基础能力，`orm.APIVersion` 当前为 `v1`。V1 公共契约采用兼容优先策略：已导出的运行时接口、元数据结构、生成器输入模型和 CLI 主命令保持向后兼容；新增能力优先通过可选字段、可选参数、独立适配层或新接口扩展。详细边界见 `docs/api-compatibility.md`。已支持：

- 实体 `//goark-orm:entity` 与严格 `goark-orm` struct tag 解析。
- Mapper `//goark-orm:mapper`、`select`、`insert`、`update`、`delete`、`call` 方法注解扫描，注解 SQL 支持 `<script>` 动态节点和显式 SQL Provider。
- XML Mapper 静态语句、动态 SQL 基础节点、`call`、`parameter`、`resultSet`、`bind`、`selectKey`、`databaseId`、`resultMap`、`constructor/idArg/arg`、`association`、`collection`、`extends`、`autoMapping`、`discriminator`、`columnPrefix`、`notNullColumn` 元数据和 namespace/类型一致性校验。
- XML 与注解在同一个 Mapper 接口中混用。
- 生成 `RegisterGoarkORMMetadata`、实体 RowScanner、Mapper 实现、分页 Mapper 签名、Cursor/ResultHandler 流式 Mapper 签名、BaseMapper/Service 工厂和 `orm.Session` 调用代码。
- Mapper 接口支持本包内接口嵌入，生成期会展平公共方法并按当前 Mapper namespace 绑定 Statement。
- 独立 `goark-orm` CLI，可不安装 Goark 主 CLI 直接生成代码；支持 `--config` JSON 配置文件批量生成多 package。
- `ormgen` 提供 `TemplateRenderer`、`SchemaIntrospector` 和 `ReverseEngineer` 扩展 SPI，可由外部数据库适配层做 schema 反向工程或自定义模板；core 不直接依赖数据库驱动。
- `ormtest` 提供环境变量门控的真实数据库兼容性测试套件，调用方在自己的测试二进制中显式 blank import 驱动后即可复用 ping、setup/cleanup、查询、分页、写语句和 callable statement 用例。
- `database/sql` Session、独立 `Configuration`、MyBatis-Plus 风格 `GlobalConfig` / `DbConfig`、`Dialect`、`ExecutorType.SIMPLE/REUSE`、`#{name}` / `#{user.name}` 安全参数编译、MyBatis 风格 `param1` / `_parameter` / `list` 别名、生成主键回填和显式注册 RowScanner 优先的基础结果扫描。
- MyBatis 风格 statement 级 `timeout`、`fetchSize`、`resultSetType`、`resultOrdered` 和 `keyColumn` 执行选项；语句级声明优先于全局默认值，并通过可选执行器接口传递给驱动适配层。
- MyBatis 风格 `MyBatisConfig`、`MyBatisSettings`、`MyBatisEnvironment`、`TypeAlias` 和 `MapperRef` Go 化配置模型，可显式构建运行期 `Configuration`。
- MyBatis `${}` 原样替换的 Go 化安全版本：默认拒绝普通字符串，只允许 `RawSQLToken`，内置 `RawIdentifier` 和 `RawOrderBy` 白名单 token。
- XML 动态 SQL 支持 `sql/include`、`bind`、`if`、`where`、`set`、`trim`、`foreach`、`choose/when/otherwise`；`test` 表达式支持安全 OGNL：括号、`and/or`、`not/!`、比较别名、四则运算、取模、三元表达式、`in/not in`、列表字面量、`empty`、确定性参数路径和白名单只读方法。
- 存储过程支持 `StatementCommandCall`、`StatementTypeCallable`、IN/OUT/INOUT 参数、`sql.Out` 绑定、按声明顺序扫描多个结果集，以及生成 Mapper 侧 `orm.Call` 调用代码。
- MyBatis-Plus 风格 `BaseMapper` 通用 CRUD、`QueryWrapper` 条件构造器和 `Page` 分页模型。
- MyBatis-Plus 风格 `Service`、`QueryChain` 和 `UpdateChain`，覆盖常用 `IService` / chain wrapper 操作。
- MyBatis-Plus 风格 `SQLInjector`、`Db` 快捷门面和 `EnumValuer` 枚举入库值接口。
- MyBatis-Plus 风格 `IDType` 主键策略：`AUTO`、`INPUT`、`ASSIGN_ID`、`ASSIGN_UUID`。
- `DbConfig` 支持全局主键策略、tablePrefix、schema、logicDeleteField、logicDeleteValue、logicNotDeleteValue、insertStrategy、updateStrategy 和 whereStrategy。
- 实体字段支持 `key-column`、`numeric-scale`、`condition`、`select=false`、`insert-strategy`、`update-strategy` 和 `where-strategy` 元数据；`BaseMapper` 会按字段/全局 insert/update 策略过滤通用 INSERT/UPDATE 列。
- `BaseMapper` 支持 `SelectCount`、`SelectMaps`、`SelectObjs`、`DeleteBatchIDs` 和 `SaveOrUpdate`。
- `BaseMapper` / `Service` 支持实体条件查询：`SelectListByEntity`、`SelectCountByEntity`、`DeleteByEntity`、`ListByEntity`、`CountByEntity` 和 `RemoveByEntity`，字段 `condition` / `where-strategy` 会参与 WHERE 构造。
- `BaseMapper` 已支持逻辑删除、`UpdateByID` 乐观锁、`created-at` / `updated-at` 自动时间字段。
- MyBatis-Plus 风格 `MetaObjectHandler` 自动填充，支持 `fill='insert'`、`fill='update'`、`fill='insert_update'`，可用于 BaseMapper 和普通 Mapper 写语句。
- `QueryWrapper` / `UpdateWrapper` 支持嵌套条件、`EXISTS` / `NOT EXISTS`、`Apply`、`Last`、`Between`、`NotBetween`、`NotLike`、`LikeLeft`、`LikeRight`、`NotIn`，查询 Wrapper 额外支持 `GroupBy` / `Having` / `Select` / `AllEq` / 条件化 `OrderBy`。
- `UpdateWrapper`、`TypedField` 和生成期 `UserTypedFields` 字段常量，支持局部更新、类型化字段引用、`SetSQL`、`SetIncrBy` 和 `SetDecrBy`。
- Registry / Session 级 `TypeHandler` SPI，内建 `json`、`time`、`decimal` 处理器。
- `SQLSession` 执行器/StatementHandler/ParameterHandler/ResultSetHandler SPI、拦截器链，以及 BlockAttack、SQL Observer、租户条件/INSERT 字段注入、数据权限条件、动态表名、分页和实体语义内置拦截器。
- `SQLSession` 支持 `StatementExecutor`、`StatementHandler`、`ParameterHandler`、`ResultSetHandler` 四层 middleware，业务可用 decorator 方式扩展执行、编译、参数绑定和结果映射链路。
- `Registry.RegisterRowScanner` 支持显式注册生成式实体行扫描器，普通实体查询、存储过程多结果集和无 TypeHandler/嵌套结构的简单 ResultMap 会先走 RowScanner，复杂 ResultMap、TypeHandler 字段和未注册实体保持反射 fallback。
- 非观测 SQL 治理拦截器：`SQLGuardRule` / `NewSQLGuardInterceptor` 可组合业务规则，`NewIllegalSQLInterceptor` 默认拒绝多语句、顶层 `SELECT *` 和无 WHERE 写语句，`NewReadOnlyInterceptor` 可保护只读会话。
- Mapper 注解和 XML 语句支持 `interceptorIgnore`，可按语句跳过 `block-attack`、`tenant`、`data-permission`、`dynamic-table`、`pagination`、`entity-semantic`、`sql-guard`、`illegal-sql`、`read-only` 或 `all`。
- 独立 `SQLSessionFactory`、`Transaction`、`TransactionFactory`、`TxSession` 和 `InTx` 回调事务模型。
- 多数据源路由层：`RoutingSession`、`RoutingSessionFactory`、`WithDataSource`、`ReadWriteDataSourceResolver` 和 `StatementDataSourceResolver` 可把生成 Mapper 调用转发到不同逻辑数据源。
- Go 原生错误层级：`ErrConfiguration`、`ErrRegistry`、`ErrStatementNotFound`、`ErrBinding`、`ErrMapping`、`ErrExecutor`、`ErrTooManyResults` 均可通过 `errors.Is` 归类，通过 `errors.As` 读取结构化上下文。
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
GOWORK=off go test -run '^$' -bench . -benchmem ./
```

真实数据库兼容性套件默认跳过；需要时设置 `GOARK_ORM_INTEGRATION_DRIVER` 和 `GOARK_ORM_INTEGRATION_DSN`，并在调用方测试二进制中显式注册数据库驱动。数据库方言和验证矩阵见 `docs/database-matrix.md`。

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

多包生成建议提交 JSON 配置文件：

```json
{
  "databaseId": "postgres",
  "typeHandlers": ["json", "decimal"],
  "packages": [
    {
      "dir": "internal/user",
      "output": "internal/user/zz_goark_orm_user_gen.go"
    },
    {
      "dir": "internal/order"
    }
  ]
}
```

```bash
goark-orm generate orm --config goark-orm.json
```

需要接入数据库反向工程时，应在业务侧或独立 adapter 实现 `ormgen.SchemaIntrospector`，再把 schema 中间模型交给 `ormgen.ReverseEngineer` / `ormgen.Render`；`goark-orm` core 不连接数据库、不提交 schema 脚本。

真实数据库测试建议放在业务侧或本地 Mac 测试包中，由测试包显式注册驱动，再复用 `ormtest` 套件。setup/cleanup SQL 可通过环境变量传入 JSON 字符串数组或 `GOARK_ORM_INTEGRATION_SQL_SEPARATOR` 分隔文本，仓库不保存私有 SQL：

```go
package user_test

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"goark.dev/orm/ormtest"
)

func TestORMDatabaseCompatibility(t *testing.T) {
	ormtest.RunDatabaseSuiteFromEnv(t)
}
```

存储过程使用显式 `call` 语句，不通过运行时扫描发现 Mapper。OUT / INOUT 参数必须绑定到指针参数，多结果集必须声明名称并绑定到对应的 slice 指针：

```go
//goark-orm:call(sql="call report_users(#{status}, #{total})", parameters="status:IN,total:OUT:BIGINT", resultSets="users:User,roles:Role")
ReportUsers(ctx context.Context, status string, total *int64, users *[]User, roles *[]Role) error
```

等价 XML 写法：

```xml
<call id="ReportUsers" statementType="CALLABLE">
  call report_users(#{status}, #{total})
  <parameter property="status" mode="IN"/>
  <parameter property="total" mode="OUT" jdbcType="BIGINT"/>
  <resultSet name="users" resultType="User"/>
  <resultSet name="roles" resultType="Role"/>
</call>
```

语句级执行选项可在注解或 XML 中直接声明。`timeout` 兼容 MyBatis 的秒数，也支持 Go duration 字符串：

```go
//goark-orm:select(sql="select id, name from sys_user where id = #{id}", timeout="2s", fetchSize=128, resultSetType="FORWARD_ONLY")
FindByID(ctx context.Context, id int64) (*User, error)
```

```xml
<select id="FindByID" resultType="User" timeout="2" fetchSize="128" resultSetType="FORWARD_ONLY">
  select id, name from sys_user where id = #{id}
</select>
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

`Db` 提供无实体生成代码时的显式 Session 快捷入口：

```go
dbKit, err := orm.NewDb(session)
if err != nil {
	return err
}
result, err := dbKit.Exec(ctx, "system.user.UserMapper.UpdateName", orm.NamedArgs{
	"id":   int64(7),
	"name": "Alice",
})
if err != nil {
	return err
}
_ = result
```

自定义通用 SQL 可以通过 `SQLInjector` 显式注入到 Registry：

```go
err := orm.RegisterInjectedStatements(
	registry,
	"system.user.UserExtraMapper",
	userEntityMeta,
	orm.LogicDeleteByIDInjector{},
	orm.WithInjectDialect(orm.NewPostgresDialect()),
)
if err != nil {
	return err
}
```

枚举字段可实现 `EnumValuer` 暴露入库值：

```go
type UserStatus string

func (s UserStatus) EnumValue() any {
	return string(s)
}
```

枚举值转换需要读取调用方上下文时，可以实现 `EnumValuerContext`；`SQLSession` 会在编译参数时透传当前 `context.Context`。

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
		orm.NewIllegalSQLInterceptor(),
		orm.NewBlockAttackInterceptor(),
	),
)
if err != nil {
	return err
}

ctx = orm.WithPageRequest(ctx, orm.NewPageRequest(1, 20))
```

只读链路可以单独启用写保护：

```go
readSession, err := orm.NewSQLSession(
	registry,
	db,
	orm.NewPostgresDialect(),
	orm.WithInterceptors(orm.NewReadOnlyInterceptor()),
)
if err != nil {
	return err
}
_ = readSession
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
config.GlobalConfig.DbConfig.IDType = orm.IDTypeAssignID
config.GlobalConfig.DbConfig.TablePrefix = "sys_"
config.GlobalConfig.DbConfig.Schema = "tenant_01"
config.GlobalConfig.DbConfig.LogicDeleteValue = int64(1)
config.GlobalConfig.DbConfig.LogicNotDeleteValue = int64(0)
config.GlobalConfig.DbConfig.InsertStrategy = orm.FieldStrategyNotEmpty
config.GlobalConfig.DbConfig.UpdateStrategy = orm.FieldStrategyNotEmpty
config.GlobalConfig.MetaObjectHandler = auditFillHandler{}

session, err := orm.NewSQLSession(registry, db, nil, orm.WithConfiguration(config))
if err != nil {
	return err
}
```

也可以使用 MyBatis 命名风格的 Go 化配置模型构建运行期配置：

```go
cacheEnabled := false
mybatisConfig := orm.MyBatisConfig{
	Settings: orm.MyBatisSettings{
		CacheEnabled:             &cacheEnabled,
		MapUnderscoreToCamelCase: true,
		DefaultExecutorType:      orm.ExecutorTypeReuse,
		DatabaseID:               "mysql8",
	},
	Environment: orm.MyBatisEnvironment{
		ID:     "prod",
		DbType: orm.DbTypeMySQL,
	},
	TypeAliases: []orm.TypeAlias{{Alias: "User", TypeName: "system.User"}},
	Mappers:     []orm.MapperRef{{Resource: "mapper/user_mapper.xml", Namespace: "system.user.UserMapper"}},
}

config, err := mybatisConfig.BuildConfiguration()
if err != nil {
	return err
}
session, err := orm.NewSQLSession(registry, db, nil, orm.WithConfiguration(config))
if err != nil {
	return err
}
```

自动填充处理器面向实体元数据工作，`StrictInsertFill` / `StrictUpdateFill` 会尊重字段 `fill` 策略：

```go
type auditFillHandler struct{}

func (auditFillHandler) InsertFill(ctx context.Context, meta *orm.MetaObject) error {
	if err := meta.StrictInsertFill("CreatedBy", currentUserID(ctx)); err != nil {
		return err
	}
	return meta.StrictInsertFill("UpdatedBy", currentUserID(ctx))
}

func (auditFillHandler) UpdateFill(ctx context.Context, meta *orm.MetaObject) error {
	return meta.StrictUpdateFill("UpdatedBy", currentUserID(ctx))
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

`${}` 只允许显式安全 token，适合动态表名、列名或排序字段：

```go
table, err := orm.NewRawIdentifier("tenant_01.sys_user")
if err != nil {
	return err
}
orderName, err := orm.NewRawOrderItem("name", false)
if err != nil {
	return err
}
orderID, err := orm.NewRawOrderItem("id", true)
if err != nil {
	return err
}

compiled, err := orm.CompileSQL(
	"select * from ${table} order by ${orderBy} limit #{limit}",
	orm.NamedArgs{
		"table":   table,
		"orderBy": orm.NewRawOrderBy(orderName, orderID),
		"limit":   20,
	},
	orm.NewPostgresDialect(),
)
if err != nil {
	return err
}
_ = compiled
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

多数据源路由保持在独立适配层，不制造跨库事务假象。读写分离可以组合已有 Session：

```go
routing, err := orm.NewRoutingSession(
	map[orm.DataSourceKey]orm.Session{
		"primary": primarySession,
		"replica": replicaSession,
	},
	orm.ReadWriteDataSourceResolver("replica", "primary"),
	orm.WithRoutingDefaultDataSource("primary"),
)
if err != nil {
	return err
}

ctx = orm.WithDataSource(ctx, "primary")
userMapper := NewUserMapper(routing)
_ = userMapper
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
Remark    string    `goark-orm:"column='remark';select=false;insert-strategy='not-empty';update-strategy='not-empty'"`
```

字段策略取值支持 `always`、`not-null`、`not-empty`、`not-zero` 和 `never`。字段级策略优先于 `DbConfig` 全局策略；未声明时保持 V1 旧行为，通用 INSERT/UPDATE 默认包含零值字段。实体条件构造默认使用 Go 化 `not-zero` 策略，避免基础类型零值误参与 WHERE。

实体条件查询会读取字段级 `condition` 和 `where-strategy`：

```go
users, err := userService.ListByEntity(ctx, &User{Name: "%Alice%"})
if err != nil {
	return err
}
_ = users
```

Goark 主 CLI 不包含 ORM 子命令，也不依赖 `goark.dev/orm`。ORM 代码生成统一使用本仓库自带的独立 `goark-orm` 命令。

## 错误分类

运行期错误遵循 Go 原生包装语义。业务代码按分类处理时使用 `errors.Is(err, orm.ErrBinding)`，需要定位信息时使用 `errors.As` 提取 `*orm.BindingError`、`*orm.MappingError`、`*orm.ExecutorError` 等结构体。

```go
if errors.Is(err, orm.ErrTooManyResults) {
	return err
}

var mappingErr *orm.MappingError
if errors.As(err, &mappingErr) {
	_ = mappingErr.Statement
	_ = mappingErr.Column
	_ = mappingErr.Field
}
```

## 工程约定

- Go 版本跟随 Goark 生态主线，当前模块声明为 `go 1.25`。
- 代码、脚本、配置和文档统一使用 UTF-8 与 LF。
- Go 代码注释使用标准简体中文，只解释非显而易见的设计意图、边界和失败语义。
- 公共 API 优先显式接口、小结构体和可组合选项，不使用 Java 风格运行时扫描或重代理模型。
- 功能实现必须包含边界条件、错误处理、上下文取消和并发安全复核。

## 许可证

本项目使用 Apache License 2.0。
