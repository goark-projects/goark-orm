# Goark ORM V1 设计方案

## 状态

Accepted for initial implementation planning.

## 背景

`goark-orm` 是可独立使用的数据映射模块，同时可以接入 Goark 生态。目标对齐 MyBatis / MyBatis-Plus 的工程体验，但保持 Go-native、显式、可生成、低反射的实现路线。

Goark 核心框架已经明确采用编译期生成注册代码，不做 Java 风格 classpath 扫描。ORM 即使单独使用，也遵守这个方向：源码、XML 和结构体 tag 只作为生成器输入；运行时使用已生成的静态元数据和绑定函数执行 SQL。

## 目标

- 支持 MyBatis 风格 XML Mapper。
- 支持 MyBatis 注解风格 Annotation Mapper。
- XML Mapper 和 Annotation Mapper 可以在同一个接口中混用。
- 统一实体元数据、Statement 元数据、参数绑定、结果映射、事务、Configuration、GlobalConfig/DbConfig、一级缓存、Mapper namespace 二级缓存和执行器内核。
- 使用独立 `goark-orm generate orm` 生成 Mapper 实现、分页签名、流式签名和静态元数据。
- 支持 Go Mapper 接口在本包内通过接口嵌入复用公共方法。
- Goark 主 CLI 不包含 ORM 子命令，也不依赖 `goark.dev/orm`。
- 保持运行时热路径低反射或无反射。

## 非目标

- 不实现独立 Repository 模式。
- 不实现 Hibernate/JPA 的持久化上下文、脏检查、透明懒加载代理和隐式 flush。
- 不做运行时 Mapper 扫描、运行时 XML 扫描或运行时实体建模。
- 不生成 SQL 迁移脚本，不提交临时 SQL 文件。
- `${}` 原样替换默认拒绝普通字符串，仅允许显式 `RawSQLToken` 白名单 token。

## 总体架构

```text
goark-orm generate orm
        │
        ├─ 扫描 //goark-orm:entity 和 goark-orm struct tag
        ├─ 扫描 //goark-orm:mapper
        ├─ 解析 Mapper XML
        ├─ 解析方法级 SQL 注解
        ▼
zz_goark_orm_<package>_gen.go
        │
        ▼
EntityMeta / StatementMeta / Mapper Impl / Binding Func
        │
        ▼
Session / Executor / Dialect / TypeHandler / Interceptor
        │
        ▼
database/sql
```

核心原则：XML 和注解只是 Statement 来源不同，进入运行时前必须统一编译为 `StatementMeta`。

## 注解命名空间

ORM 注解统一使用 `//goark-orm:xxx`，不复用核心 DI 的 `//goark:xxx`。

注解参数遵循 Goark 注解语法：参数使用 `key=value`，字符串使用双引号。

```go
//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
}
```

## Struct Tag 规范

实体字段使用 Go 标准 struct tag，tag key 固定为 `goark-orm`。

```go
ID int64 `json:"id" goark-orm:"column='id';primary-key=true;auto-increment=true"`
```

tag value 规则：

```text
1. 多个属性使用分号分隔。
2. 每个属性必须是 key=value。
3. 字符串值必须使用单引号。
4. 布尔值只能使用 true 或 false。
5. 整数和小数不加引号。
6. 同一个字段只能出现一个 goark-orm tag。
```

合法示例：

```go
Name string `goark-orm:"column='name';size=64;nullable=false"`
```

非法示例：

```go
Name string `goark-orm:"column=name"`
Name string `goark-orm:"column='name';not-null"`
Name string `goark-orm:"column='name',nullable=false"`
```

生成器必须在编译前拒绝非法 tag。

## 实体映射

实体类型必须使用 `//goark-orm:entity` 声明表名。

```go
//goark-orm:entity(table="sys_user")
type User struct {
	ID        int64     `json:"id" goark-orm:"column='id';primary-key=true;auto-increment=true"`
	Name      string    `json:"name" goark-orm:"column='name';size=64;nullable=false"`
	Status    string    `json:"status" goark-orm:"column='status';type='varchar(32)';default='ACTIVE'"`
	Profile   Profile   `json:"profile" goark-orm:"column='profile';type='jsonb';type-handler='json'"`
	Version   int64     `json:"version" goark-orm:"column='version';version=true"`
	Deleted   bool      `json:"deleted" goark-orm:"column='deleted';soft-delete=true"`
	CreatedAt time.Time `json:"createdAt" goark-orm:"column='created_at';created-at=true;fill='insert'"`
	UpdatedAt time.Time `json:"updatedAt" goark-orm:"column='updated_at';updated-at=true;fill='insert_update'"`
	Temp      string    `json:"-" goark-orm:"transient=true"`
}
```

实体规则：

```text
1. table 必填。
2. 持久化字段必须声明 column。
3. 非持久化字段必须显式声明 transient=true。
4. 一个实体至少需要一个 primary-key=true 字段。
5. auto-increment=true 只能用于主键字段。
6. id-type 只能用于主键字段，auto-increment=true 只能与 id-type=AUTO 或未声明 id-type 共用。
7. version=true 在同一个实体内最多只能出现一次。
8. soft-delete=true 在同一个实体内最多只能出现一次。
9. created-at=true 和 updated-at=true 允许各出现一次。
10. fill 支持 insert、update、insert_update，运行期由 MetaObjectHandler 执行严格填充。
```

常用字段属性：

| 属性 | 类型 | 说明 |
| --- | --- | --- |
| `column` | string | 数据库列名 |
| `primary-key` | bool | 主键字段 |
| `auto-increment` | bool | 数据库自增主键 |
| `id-type` | string | 主键策略：AUTO、INPUT、ASSIGN_ID、ASSIGN_UUID |
| `type` | string | 数据库列类型描述 |
| `size` | int | 字符串长度或精度辅助信息 |
| `nullable` | bool | 是否允许空值 |
| `default` | string | 默认值描述 |
| `type-handler` | string | 类型处理器名称 |
| `version` | bool | 乐观锁版本字段 |
| `soft-delete` | bool | 逻辑删除字段 |
| `created-at` | bool | 创建时间字段 |
| `updated-at` | bool | 更新时间字段 |
| `fill` | string | 自动填充策略：insert、update、insert_update |
| `transient` | bool | 不参与 ORM 映射 |

## Mapper 声明

Mapper 只能声明为接口。

```go
//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListByStatus(ctx context.Context, status string) ([]User, error)

	//goark-orm:insert(sql="insert into sys_user(name, status) values(#{Name}, #{Status})", useGeneratedKeys=true, keyProperty="ID")
	Insert(ctx context.Context, user *User) (int64, error)

	//goark-orm:update(sql="update sys_user set name = #{Name}, status = #{Status} where id = #{ID}")
	Update(ctx context.Context, user *User) (int64, error)

	//goark-orm:delete(sql="delete from sys_user where id = #{id}")
	Delete(ctx context.Context, id int64) (int64, error)
}
```

Mapper 规则：

```text
1. namespace 必填。
2. namespace 必须手动指定，禁止生成器自动推断。
3. namespace 在整个 ORM Registry 内必须唯一。
4. xml 可选。
5. XML 和注解可以在同一个 Mapper 中混用。
6. 每个 Mapper 方法必须绑定且只能绑定一个 Statement。
7. 同一个方法不能同时由 XML 和注解声明 Statement。
8. Statement 全名统一为 namespace + "." + methodName。
9. Mapper 方法第一个参数必须是 context.Context。
10. Mapper 方法最后一个返回值必须是 error。
11. Mapper 可以嵌入本包内命名接口，生成器会展平方法并按当前 Mapper namespace 绑定 Statement。
```

namespace 推荐按业务域命名，不建议强绑定 Go 包路径：

```text
system.user.UserMapper
iam.role.RoleMapper
timesheet.entry.EntryMapper
```

## 方法级 SQL 注解

V1 只保留四个 SQL 注解：

```text
//goark-orm:select
//goark-orm:insert
//goark-orm:update
//goark-orm:delete
```

不提供通用 `query` 注解。SQL 命令类型由注解名决定。

方法级 SQL 注解规则：

```text
1. 一个方法最多只能有一个 SQL 注解。
2. select、insert、update、delete 互斥。
3. sql 和 provider 二选一必填，不能同时声明。
4. sql 可以使用 `<script>` 包裹 MyBatis 风格动态 SQL 节点。
5. provider 是运行期 SQL Provider 名称，必须由业务显式注册到 `orm.Registry`。
6. insert 可以声明 useGeneratedKeys 和 keyProperty。
7. select 返回实体、实体指针、实体切片、分页结果或标量。
8. insert、update、delete 返回受影响行数或生成主键时必须符合生成器支持的签名。
```

示例：

```go
//goark-orm:select(sql="select id, name from sys_user where id = #{id}")
FindByID(ctx context.Context, id int64) (*User, error)

//goark-orm:insert(sql="insert into sys_user(name) values(#{Name})", useGeneratedKeys=true, keyProperty="ID")
Insert(ctx context.Context, user *User) (int64, error)

//goark-orm:update(sql="update sys_user set name = #{Name} where id = #{ID}")
Update(ctx context.Context, user *User) (int64, error)

//goark-orm:delete(sql="delete from sys_user where id = #{id}")
Delete(ctx context.Context, id int64) (int64, error)
```

Annotation Mapper 支持 `<script>` 动态 SQL，但复杂、可复用 SQL 仍建议放入 XML，便于 review 和复用。

```go
//goark-orm:select(sql="<script>select id, name from sys_user <where><if test=\"status != nil and status != ''\">status = #{status}</if></where></script>")
List(ctx context.Context, status string) ([]User, error)

//goark-orm:select(provider="UserSQL.ListByStatus")
ListByProvider(ctx context.Context, status string) ([]User, error)
```

## XML Mapper

XML Mapper 使用显式 namespace，必须与 Go Mapper 注解中的 namespace 完全一致。

```xml
<mapper namespace="system.user.UserMapper">
  <resultMap id="UserResult" type="User">
    <id property="ID" column="id"/>
    <result property="Name" column="name"/>
    <result property="Status" column="status"/>
    <result property="Profile" column="profile" typeHandler="json"/>
  </resultMap>

  <select id="FindByID" resultMap="UserResult">
    select id, name, status, profile
    from sys_user
    where id = #{id}
  </select>
</mapper>
```

XML 规则：

```text
1. mapper.namespace 必填。
2. XML namespace 必须与 Go Mapper namespace 完全一致。
3. select、insert、update、delete 的 id 必须匹配接口方法名。
4. resultMap 引用必须存在。
5. #{param} 走安全参数绑定。
6. `${}` 仅允许绑定 `RawSQLToken`，用于表名、列名、排序字段等白名单场景。
7. XML 中定义的方法如果又在接口方法上声明 SQL 注解，生成器必须报错。
8. `<cache>` 和 `<cache-ref>` 只允许二选一，缓存配置进入生成后的静态 MapperMeta。
9. statement 可声明 `useCache` 和 `flushCache`，未声明时使用 MyBatis 默认策略。
10. statement 可声明 `databaseId`，生成期优先选择匹配 `GenerateSpec.DatabaseID` 的同名语句。
```

V1 支持节点：

| 节点 | 说明 |
| --- | --- |
| `mapper` | XML 根节点 |
| `cache` | 当前 namespace 二级缓存 |
| `cache-ref` | 复用其他 namespace 二级缓存 |
| `resultMap` | 结果映射 |
| `id` | 主键结果映射 |
| `result` | 普通结果映射 |
| `association` | 嵌套对象映射 |
| `collection` | 嵌套集合映射 |
| `discriminator` / `case` | 判别器元数据 |
| `select` | 查询语句 |
| `insert` | 插入语句 |
| `update` | 更新语句 |
| `delete` | 删除语句 |
| `sql` | SQL 片段 |
| `include` | 片段引用 |
| `bind` | 动态变量绑定 |
| `if` | 条件 SQL |
| `where` | WHERE 包装 |
| `set` | SET 包装 |
| `trim` | 前后缀修剪 |
| `foreach` | 集合展开 |
| `choose` / `when` / `otherwise` | 分支 SQL |

动态 SQL 当前使用标准库 `encoding/xml.Decoder` 解析，保留文本节点和元素顺序。V1 表达式实现安全受控子集：

```text
1. 支持 `and` / `or` 组合。
2. 支持括号分组和 `not` / `!` 取反。
3. 支持 `==` / `!=` / `>` / `>=` / `<` / `<=`。
4. 支持 `nil` / `null`、布尔值、数值、字符串字面量和已命名参数。
5. 支持 `user.name`、`items[0]`、`map.key` 这类确定性参数路径。
6. 支持集合或字符串的 `size` / `length` / `size()` / `length()` 和 Go 化 `len(value)` 只读长度表达式。
7. 不执行脚本，不支持任意函数调用；`${}` 仅允许显式 `RawSQLToken`。
```

动态 SQL 示例：

```xml
<select id="List" resultMap="UserResult">
  select id, name, status
  from sys_user
  <where>
    <if test="status != nil and status != ''">
      status = #{status}
    </if>
    <if test="name != nil and name != ''">
      and name like concat('%', #{name}, '%')
    </if>
  </where>
</select>
```

## 参数绑定

参数绑定统一使用 `#{name}`。

```text
1. 方法参数名由生成器从接口方法 AST 中读取。
2. 单结构体参数可以使用字段名绑定，例如 `#{ID}`、`#{Name}`，也可以使用 Go 参数路径，例如 `#{user.ID}`、`#{user.name}`。
3. 多参数方法会生成 MyBatis 风格 `param1`、`param2` 别名，同时保留 Go 参数名。
4. 单参数会生成 `_parameter` 别名；单 slice/array 参数会生成 `collection`、`list`、`array` 别名。
5. 参数路径支持结构体导出字段、lower-camel 字段别名、map key 和 slice/array 下标。
6. 不存在的参数或字段生成期报错。
7. TypeHandler 在入库和出库两个方向都必须参与；实体路径参数只转换 SQL 实际引用的字段，避免未使用字段触发无关 TypeHandler。
```

示例：

```go
//goark-orm:update(sql="update sys_user set status = #{status} where id = #{id}")
UpdateStatus(ctx context.Context, id int64, status string) (int64, error)
```

## 返回值签名

V1 支持的签名：

```go
Find(ctx context.Context, id int64) (*User, error)
List(ctx context.Context, status string) ([]User, error)
List(ctx context.Context, query UserQuery, page orm.PageRequest) (orm.Page[User], error)
ListCursor(ctx context.Context, status string) (*orm.Cursor[User], error)
ListEach(ctx context.Context, status string, handler orm.ResultHandler[User]) error
Count(ctx context.Context) (int64, error)
Insert(ctx context.Context, user *User) (int64, error)
Update(ctx context.Context, user *User) (int64, error)
Delete(ctx context.Context, id int64) (int64, error)
```

分页 Mapper 签名由生成器识别 `orm.Page[T]` 返回值和 `orm.PageRequest` 参数，并生成 `orm.QueryPage[T]` 调用。`PageRequest` 不参与 SQL 参数绑定。
流式 Mapper 签名由生成器识别 `*orm.Cursor[T]` 返回值或 `orm.ResultHandler[T]` 参数，并分别生成 `orm.QueryCursor[T]` 和 `orm.QueryEach[T]` 调用。`ResultHandler` 不参与 SQL 参数绑定。

V1 不支持隐式无 `error` 返回值。

## Runtime 包结构

```text
dialect       数据库方言、占位符、分页、标识符引用
mapping       EntityMeta、ColumnMeta、ResultMap
statement     StatementMeta、Provider、动态 SQL AST、参数绑定计划
executor      Query、QueryOne、Exec、Batch、结果映射
session       Session、Configuration、事务上下文、一级缓存
typehandler   JSON、Time、Decimal、自定义类型处理器
interceptor   SQL 日志、分页、租户、乐观锁、逻辑删除、指标
xmlmapper     XML 解析模型
ormgen        生成器模型、校验器、代码渲染
```

关键运行时接口：

```go
type Executor interface {
	Query(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error
	QueryOne(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error
	Exec(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs) (Result, error)
}
```

```go
type TypeHandler interface {
	ToDB(ctx context.Context, value any) (any, error)
	FromDB(ctx context.Context, value any, target any) error
}
```

```go
type SQLProvider func(ctx context.Context, statement StatementMeta, args NamedArgs) (SQLSource, error)
```

运行时要求：

```text
1. 所有数据库操作必须接受 context.Context。
2. 不在热路径解析 XML、注解或 struct tag。
3. 不在热路径做全量反射字段扫描。
4. Mapper 实现和行扫描函数由生成器生成。
5. 事务和连接生命周期由 goark-orm 自身抽象承载，默认实现基于 database/sql。
```

## CLI 边界

ORM 必须提供独立 CLI，用户不安装 Goark 主 CLI 也能生成代码：

```bash
goark-orm generate orm ./...
```

Goark 主 CLI 必须保持不依赖其他 Goark 模块，因此不能包装 `goark-orm` 的生成能力。ORM 用户统一安装和执行独立的 `goark-orm` 命令。

实现分层：

```text
goark.dev/orm/cmd/goark-orm
        │
        ├─ 独立命令入口
        ├─ 参数解析
        └─ 文件输出
        │
        ▼
goark.dev/orm/ormgen
        │
        ├─ ORM 注解解析
        ├─ struct tag 解析
        ├─ XML 解析
        ├─ 元数据校验
        └─ 代码生成模型
        │
        ▼
goark.dev/orm
        └─ Runtime 契约
```

依赖方向只能是 `goark.dev/orm/cmd -> goark.dev/orm/ormgen -> goark.dev/orm`。禁止 `goark-orm` 反向依赖 Goark core、boot 或 CLI，也禁止 Goark 主 CLI 依赖 `goark.dev/orm`。

## 生成内容

每个包默认生成：

```text
zz_goark_orm_<package>_gen.go
```

生成文件包含：

```text
1. Entity 静态元数据。
2. ResultMap 静态元数据。
3. Mapper Cache 静态元数据。
4. Statement 静态元数据。
5. Mapper 接口实现类型。
6. 参数绑定函数。
7. 行扫描函数。
8. TypeHandler 引用。
9. 类型安全字段常量和类型化字段常量。
10. 单主键实体的 BaseMapper 工厂。
11. 单主键实体的 Service 工厂。
12. 可选 Goark Bean 注册代码。
```

生成代码必须可读、确定性排序、可提交，并能通过 `go test ./...` 编译。

## 生成期校验

生成器必须在编译前发现并拒绝以下问题：

```text
1. namespace 缺失。
2. namespace 重复。
3. XML namespace 与 Go Mapper namespace 不一致。
4. Mapper 方法没有 Statement。
5. Mapper 方法重复绑定 Statement。
6. 方法级 SQL 注解互斥冲突。
7. goark-orm tag 不是 key=value 格式。
8. tag 字符串值未使用单引号。
9. 持久化字段缺少 column。
10. 主键缺失或主键规则冲突。
11. type-handler 未注册。
12. resultMap 不存在。
13. SQL 参数路径找不到对应方法参数、别名或结构体字段。
14. 返回值签名不支持。
15. XML 或注解使用 `${}` 时，运行期必须绑定 `RawSQLToken`。
```

错误信息必须包含文件、类型、方法或字段定位。

示例：

```text
goark-orm: mapper UserMapper missing required namespace
goark-orm: duplicate mapper namespace "system.user.UserMapper"
goark-orm: XML namespace "system.user.UserXmlMapper" does not match mapper namespace "system.user.UserMapper"
goark-orm: field User.Name has invalid tag "column=name": string value must use single quotes
goark-orm: method UserMapper.FindByID is declared by both XML and annotation
```

## 事务与拦截器

`goark-orm` core 是独立 ORM 框架，自带 MyBatis 风格的 `SQLSessionFactory`、`Transaction`、`TransactionFactory`、`TxSession` 和 `InTx` 回调事务模型。默认事务实现基于 `database/sql` 的 `*sql.DB` / `*sql.Tx`。

生成 Mapper 只依赖 `orm.Session`，所以普通自动提交 Session、事务 Session 和 BatchSession 都可以直接传给生成 Mapper：

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
```

`BatchSession` 对齐 MyBatis `ExecutorType.BATCH` 的核心行为：写语句先进入队列，`Flush(ctx)` 按顺序执行并返回每条语句的 `BatchResult`；查询前会自动 flush，事务批处理可通过 `factory.BeginBatchTx(ctx, opts)` 或在 `InTx` 回调中使用 `orm.NewBatchSession(session)`。

`SQLSession` 默认启用 Session 级一级缓存。缓存 key 由最终编译 SQL 和参数组成，写操作、`Commit`、`Rollback` 和 `Close` 会清空缓存。可通过 `WithLocalCache(false)` 关闭，也可以通过 `Configuration.LocalCacheScope` 设置为 `STATEMENT`，使缓存不跨语句复用。

运行期配置由 `orm.Configuration` 承载，默认通过 `orm.DefaultConfiguration()` 创建，再使用 `orm.WithConfiguration(config)` 应用到 `SQLSession`。当前可配置项包括方言、databaseId、一级缓存开关、一级缓存作用域、二级缓存总开关、下划线转驼峰自动映射、默认执行器类型、默认超时、fetch size 元数据和 `GlobalConfig`。

`GlobalConfig` 对齐 MyBatis-Plus 的全局扩展点，当前承载 `DbConfig`、`IdentifierGenerator` 和 `MetaObjectHandler`。`DbConfig` 已支持全局 `IDType`、`TablePrefix`、`Schema`、`LogicDeleteField`、`LogicDeleteValue` 和 `LogicNotDeleteValue`。BaseMapper 会读取这些配置：主键字段未显式声明 `id-type` 且不是自增时，使用全局 `IDType`；渲染物理表名时应用 tablePrefix/schema；逻辑删除默认值从 DbConfig 读取。显式实体 tag 优先于全局兜底配置。

`MetaObjectHandler` 对齐 MyBatis-Plus 字段填充模型，但采用 Go 显式接口。BaseMapper 会在构造写入参数前填充实体；普通 Mapper 写语句会在拦截器改写后、方言占位符编译前填充运行时命名参数。`StrictInsertFill` 和 `StrictUpdateFill` 会读取 `ColumnMeta.Fill`，并兼容旧的 `created-at` / `updated-at` 语义。

运行期错误提供 MyBatis 风格分层语义和 Go 原生判别方式。`ErrORM` 是根分类；配置、元数据、Statement 查找、参数绑定、结果映射、数据库执行和 QueryOne 多行结果分别对应 `ErrConfiguration`、`ErrRegistry`、`ErrStatementNotFound`、`ErrBinding`、`ErrMapping`、`ErrExecutor`、`ErrTooManyResults`。调用方使用 `errors.Is` 按阶段归类，使用 `errors.As` 提取 `ConfigurationError`、`BindingError`、`MappingError`、`ExecutorError` 等结构化上下文，避免依赖错误字符串。

Mapper namespace 级二级缓存由 `Cache` SPI 承载，默认实现是并发安全的有界内存 LRU 缓存。XML `<cache>` 会为当前 namespace 创建默认二级缓存，`<cache-ref namespace="...">` 会复用目标 namespace 缓存。`select` 默认 `useCache=true`，insert/update/delete 默认 `flushCache=true`，select 默认 `flushCache=false`；显式 `useCache=false` 或 `flushCache=false` 会覆盖默认策略。

二级缓存遵循 MyBatis 风格事务发布语义：自动提交 Session 查询后立即写入缓存，写语句成功后立即清理 namespace 缓存；事务 Session 内的查询缓存写入和写语句缓存清理先进入挂起队列，只有事务 `Commit` 成功后才对共享二级缓存生效，`Rollback` 和未完成 `Close` 会丢弃挂起变更。

未来与 Goark 生态集成时，应新增类似 `mybatis-spring` 的适配层对接 `goark/db`、boot 生命周期和容器装配；`goark-orm` core 不直接依赖 `goark/db`。

拦截器顺序建议：

```text
1. 超时和上下文检查
2. 租户或数据权限条件注入
3. 逻辑删除条件注入
4. 分页和方言改写
5. 乐观锁改写
6. SQL 日志脱敏
7. 指标和 tracing
8. Executor 执行
```

V1 已提供 `StatementInterceptor` around-style SPI，拦截器在动态 SQL 渲染后、方言占位符编译前工作。内置能力包括：

```text
1. BlockAttackInterceptor：拒绝无 WHERE 的 update/delete。
2. SQLObserverInterceptor：观察下游改写后的最终 SQL 模板和命名参数。
3. TenantInterceptor：按列和值注入租户 WHERE 条件，并对显式列清单的 INSERT VALUES 语句追加租户列和值。
4. DataPermissionInterceptor：由业务 Provider 返回数据权限 SQLCondition。
5. DynamicTableInterceptor：按映射改写 from/join/update/into 后的表名。
6. PaginationInterceptor：从 context 读取 PageRequest 并追加方言分页。
```

分页拦截器使用 `WithPageRequest(ctx, page)` 传递分页请求。租户拦截器覆盖 select/update/delete 的条件注入和 insert 的租户字段注入；无显式列清单或非 VALUES 形态的 insert 会 fail-fast，避免租户字段静默漏写。

## 分期计划

### V1.1 Entity 与基础 Runtime

- 实现 `goark-orm` tag parser。
- 实现 Entity metadata。
- 实现 Dialect、Statement、Executor、TypeHandler 基础接口。
- 提供 fake executor / sqlmock 风格单测。

### V1.2 Annotation Mapper

- 实现 `//goark-orm:mapper`。
- 实现 `select`、`insert`、`update`、`delete` 方法注解。
- 生成 Mapper 实现、参数绑定和结果扫描。
- 校验 namespace 唯一性和方法签名。

### V1.3 XML Mapper 静态 SQL

- 解析 XML `mapper`、`resultMap`、`select`、`insert`、`update`、`delete`。
- 校验 XML namespace 与 Go Mapper namespace。
- 支持 XML 与注解混用。
- 禁止同方法重复绑定。

### V1.4 XML 动态 SQL

- 已支持 `if`、`where`、`set`、`trim`、`foreach`、`choose`。
- 已支持 `sql` / `include`，include 在生成期展开。
- 已完成动态 SQL AST 到 Statement 执行计划的编译。

### V1.5 工程能力

- 已支持独立 `SQLSessionFactory`、自动提交 Session、手动 `BeginTx`、`Commit`、`Rollback`、`Close` 和 `InTx` 回调事务。
- 已支持 BaseMapper 分页、生成 Mapper 分页签名、生成 Mapper Cursor/ResultHandler 流式签名、按主键批量查询和生成主键元数据透传。
- 已支持 BatchSession 批处理执行器、自动提交批处理和事务批处理。
- 已支持 Session 级一级缓存及写操作/生命周期失效。
- 已支持 Mapper namespace 级二级缓存 SPI、默认内存 LRU 缓存、XML `<cache>` / `<cache-ref>`、statement `useCache` / `flushCache` 和事务提交发布语义。
- 已支持 ResultMap 的 association 和 collection 嵌套映射，collection 会按根对象 id 聚合多行结果。
- 已支持 ResultMap association/collection 的 `select` 嵌套查询 eager 回填、`Lazy[T]` / `LazySlice[T]` 显式 lazy 延迟加载、复合列参数绑定和单次父查询内相同参数结果复用；非 Lazy 字段保持 eager 行为，避免引入透明代理。
- 已支持 XML resultMap `constructor/idArg/arg`、`columnPrefix`、`notNullColumn`、`extends` 生成期继承合并、`autoMapping` 三态元数据和 `discriminator/case` 运行期分派。
- 已支持 Annotation Mapper 的 `<script>` 动态 SQL 和显式注册 SQL Provider。
- 已支持 `ResultHandler`、`QueryCursor`、`QueryEach` 和 `RowCursor` 逐行流式查询；游标查询绕过缓存写入，并拒绝 collection resultMap 多行聚合场景。
- 已支持独立 `Configuration` API，用于统一配置方言、缓存策略、下划线转驼峰和默认执行器类型。
- 已支持 MyBatis-Plus 风格 `GlobalConfig` / `DbConfig`。
- 已支持 `ExecutorType.REUSE` 预编译语句复用，按最终 SQL 在 Session 内缓存 prepared statement，并在 Session/事务生命周期结束时关闭。
- 已支持 BaseMapper 逻辑删除、`UpdateByID` 乐观锁、`created-at` / `updated-at` 自动时间字段。
- 已支持 `MetaObjectHandler` 自动填充及 `fill` 字段策略。
- 已支持 SQLSession 拦截器 SPI、全表更新/删除保护、SQL 观察、租户条件、数据权限条件、动态表名和分页拦截器。
- 已支持 `UpdateWrapper` 局部更新、常用条件操作符、`TypedField` 字段引用、生成期 `UserTypedFields`、`SetSQL`、`SetIncrBy` 和 `SetDecrBy`。
- 已支持 `IDType` 主键策略、默认 ASSIGN_ID/ASSIGN_UUID 生成器、XML `<bind>` 和 `databaseId` 语句选择。
- 已支持 `QueryWrapper` / `UpdateWrapper` 嵌套条件、EXISTS/NOT EXISTS、Apply、Last、Between/NotBetween、NotLike、LikeLeft/LikeRight、NotIn，以及 QueryWrapper 的 GroupBy/Having/Select/AllEq/条件化 OrderBy。
- 已支持 BaseMapper 的 SelectCount、SelectMaps、SelectObjs、DeleteBatchIDs 和 SaveOrUpdate。
- 已支持 MyBatis-Plus 风格 Service 层、QueryChain、UpdateChain，并由生成器为单主键实体输出 `New<Entity>Service` 工厂。
- 已支持 MyBatis-Plus 风格 `SQLInjector` 显式通用方法注入、无全局会话的 `Db` 快捷门面，以及 `EnumValuer` 枚举入库值接口。
- 已支持 Mapper 本包内接口嵌入展平，公共查询/写入接口可以复用到具体 Mapper。
- 增加 SQL 日志脱敏、慢 SQL、指标和 tracing 的更完整观测实现。

## 关键决策

| 决策 | 结论 | 原因 |
| --- | --- | --- |
| 公开模型 | Mapper 单体系，并提供 BaseMapper 通用 CRUD | XML 和注解覆盖 MyBatis 风格，BaseMapper 覆盖 MyBatis-Plus 常用 CRUD，不引入重复 Repository 概念 |
| SQL 注解 | 只保留 select/insert/update/delete | 比通用 query 更明确，减少误用 |
| namespace | 必填且全局唯一 | 自动推断容易重复，namespace 是稳定公共契约 |
| 实体字段 | 使用 `goark-orm` struct tag | 字段级元数据更符合 Go 生态 |
| tag 格式 | 分号分隔且必须 key=value | 解析确定、可扩展、便于错误定位 |
| 生成方式 | 编译期生成 | 符合 Goark 低反射和显式注册路线 |
| CLI | 只提供独立 `goark-orm generate orm` | ORM 本身必须能独立安装和使用，Goark 主 CLI 不能依赖 ORM 模块 |

## 验证要求

每个实现阶段至少执行：

```bash
go test ./...
go vet ./...
```

生成器相关变更必须覆盖：

```text
1. 合法实体 tag。
2. 非法实体 tag。
3. namespace 缺失和重复。
4. XML 与注解混用。
5. XML 和注解重复绑定同一方法。
6. SQL 参数绑定校验。
7. 返回值签名校验。
```
