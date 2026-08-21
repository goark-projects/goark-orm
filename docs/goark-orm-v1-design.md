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
- 统一实体元数据、Statement 元数据、参数绑定、结果映射、事务和执行器内核。
- 使用独立 `goark-orm generate orm` 生成 Mapper 实现和静态元数据。
- 在 Goark 生态中可选使用 `goark generate orm` 作为薄包装入口。
- 保持运行时热路径低反射或无反射。

## 非目标

- 不实现独立 Repository 模式。
- 不实现 Hibernate/JPA 的持久化上下文、脏检查、透明懒加载代理和隐式 flush。
- 不做运行时 Mapper 扫描、运行时 XML 扫描或运行时实体建模。
- 不生成 SQL 迁移脚本，不提交临时 SQL 文件。
- V1 不支持 `${}` 原样字符串替换。

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
goark-database / database/sql
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
	CreatedAt time.Time `json:"createdAt" goark-orm:"column='created_at';created-at=true"`
	UpdatedAt time.Time `json:"updatedAt" goark-orm:"column='updated_at';updated-at=true"`
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
6. version=true 在同一个实体内最多只能出现一次。
7. soft-delete=true 在同一个实体内最多只能出现一次。
8. created-at=true 和 updated-at=true 允许各出现一次。
```

常用字段属性：

| 属性 | 类型 | 说明 |
| --- | --- | --- |
| `column` | string | 数据库列名 |
| `primary-key` | bool | 主键字段 |
| `auto-increment` | bool | 数据库自增主键 |
| `type` | string | 数据库列类型描述 |
| `size` | int | 字符串长度或精度辅助信息 |
| `nullable` | bool | 是否允许空值 |
| `default` | string | 默认值描述 |
| `type-handler` | string | 类型处理器名称 |
| `version` | bool | 乐观锁版本字段 |
| `soft-delete` | bool | 逻辑删除字段 |
| `created-at` | bool | 创建时间字段 |
| `updated-at` | bool | 更新时间字段 |
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
3. sql 参数必填。
4. insert 可以声明 useGeneratedKeys 和 keyProperty。
5. select 返回实体、实体指针、实体切片、分页结果或标量。
6. insert、update、delete 返回受影响行数或生成主键时必须符合生成器支持的签名。
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

Annotation Mapper V1 主要服务简单静态 SQL。复杂动态 SQL 优先放入 XML。

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
6. ${} V1 默认禁止。
7. XML 中定义的方法如果又在接口方法上声明 SQL 注解，生成器必须报错。
```

V1 支持节点：

| 节点 | 说明 |
| --- | --- |
| `mapper` | XML 根节点 |
| `resultMap` | 结果映射 |
| `id` | 主键结果映射 |
| `result` | 普通结果映射 |
| `select` | 查询语句 |
| `insert` | 插入语句 |
| `update` | 更新语句 |
| `delete` | 删除语句 |
| `sql` | SQL 片段 |
| `include` | 片段引用 |
| `if` | 条件 SQL |
| `where` | WHERE 包装 |
| `set` | SET 包装 |
| `trim` | 前后缀修剪 |
| `foreach` | 集合展开 |
| `choose` / `when` / `otherwise` | 分支 SQL |

动态 SQL 当前使用标准库 `encoding/xml.Decoder` 解析，保留文本节点和元素顺序。V1 表达式只实现安全最小子集：

```text
1. 支持 `and` / `or` 组合。
2. 支持 `==` / `!=`。
3. 支持 `nil`、布尔值、字符串字面量和已命名参数。
4. 不执行脚本，不支持任意函数调用，不支持 `${}`。
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
2. 单结构体参数可以使用字段名绑定，例如 #{ID}、#{Name}。
3. 多参数方法必须使用参数名绑定，例如 #{id}、#{status}。
4. 不存在的参数或字段生成期报错。
5. TypeHandler 在入库和出库两个方向都必须参与。
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
Count(ctx context.Context) (int64, error)
Insert(ctx context.Context, user *User) (int64, error)
Update(ctx context.Context, user *User) (int64, error)
Delete(ctx context.Context, id int64) (int64, error)
```

分页结果可以后续加入：

```go
List(ctx context.Context, query UserQuery, page orm.PageRequest) (orm.Page[User], error)
```

V1 不支持隐式无 `error` 返回值。

## Runtime 包结构

```text
dialect       数据库方言、占位符、分页、标识符引用
mapping       EntityMeta、ColumnMeta、ResultMap
statement     StatementMeta、动态 SQL AST、参数绑定计划
executor      Query、QueryOne、Exec、Batch、结果映射
session       Session、事务上下文
typehandler   JSON、Time、Decimal、自定义类型处理器
interceptor   SQL 日志、分页、租户、乐观锁、逻辑删除、指标
xmlmapper     XML 解析模型
ormgen        生成器模型、校验器、代码渲染
```

关键运行时接口：

```go
type Executor interface {
	Query(ctx context.Context, statement Statement, args Args, dest any) error
	QueryOne(ctx context.Context, statement Statement, args Args, dest any) error
	Exec(ctx context.Context, statement Statement, args Args) (Result, error)
}
```

```go
type TypeHandler interface {
	ToDB(ctx context.Context, value any) (any, error)
	FromDB(ctx context.Context, value any, target any) error
}
```

运行时要求：

```text
1. 所有数据库操作必须接受 context.Context。
2. 不在热路径解析 XML、注解或 struct tag。
3. 不在热路径做全量反射字段扫描。
4. Mapper 实现和行扫描函数由生成器生成。
5. 事务和连接生命周期对接 goark-database。
```

## CLI 边界

ORM 必须提供独立 CLI，用户不安装 Goark 主 CLI 也能生成代码：

```bash
goark-orm generate orm ./...
```

Goark 主 CLI 可以作为可选生态包装：

```bash
goark generate orm ./...
```

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

goark.dev/cli
        └─ 可选包装，复用 goark-orm/ormgen，不承载 ORM 核心逻辑
```

依赖方向只能是 `goark.dev/orm/cmd -> goark.dev/orm/ormgen -> goark.dev/orm`，以及可选 `cli -> goark.dev/orm/ormgen -> goark.dev/orm`。禁止 `goark-orm` 反向依赖 Goark core、boot 或 CLI。

## 生成内容

每个包默认生成：

```text
zz_goark_orm_<package>_gen.go
```

生成文件包含：

```text
1. Entity 静态元数据。
2. ResultMap 静态元数据。
3. Statement 静态元数据。
4. Mapper 接口实现类型。
5. 参数绑定函数。
6. 行扫描函数。
7. TypeHandler 引用。
8. 可选 Goark Bean 注册代码。
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
13. SQL 参数找不到对应方法参数或结构体字段。
14. 返回值签名不支持。
15. XML 使用 V1 禁止的 ${}。
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

事务由 `goark-database` 提供连接和事务底座，`goark-orm` 只消费事务上下文。

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

V1 可以先保留拦截器接口，不必一次实现全部内置拦截器。

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

- 对接 `goark-database` 事务。
- 支持分页、批量、生成主键。
- 支持逻辑删除、乐观锁、自动时间字段。
- 增加 SQL 日志脱敏、慢 SQL、指标和 tracing 扩展点。

## 关键决策

| 决策 | 结论 | 原因 |
| --- | --- | --- |
| 公开模型 | 只做 Mapper 单体系 | XML 和注解足够覆盖 MyBatis / MyBatis-Plus 风格，不引入重复 Repository 概念 |
| SQL 注解 | 只保留 select/insert/update/delete | 比通用 query 更明确，减少误用 |
| namespace | 必填且全局唯一 | 自动推断容易重复，namespace 是稳定公共契约 |
| 实体字段 | 使用 `goark-orm` struct tag | 字段级元数据更符合 Go 生态 |
| tag 格式 | 分号分隔且必须 key=value | 解析确定、可扩展、便于错误定位 |
| 生成方式 | 编译期生成 | 符合 Goark 低反射和显式注册路线 |
| CLI | `goark-orm generate orm` 为主，`goark generate orm` 为可选包装 | ORM 本身必须能独立安装和使用，Goark 主 CLI 不能承载 ORM 核心逻辑 |

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
