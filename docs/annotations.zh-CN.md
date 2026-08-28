# 注解、Tag 与 XML Mapper 参考

默认文档语言为英文：[annotations.md](annotations.md)。本文件是中文镜像。

本文是生成器输入的映射契约参考。运行期代码不会扫描 Go 文件或 XML 文件；`ormgen` 只在生成期读取这些输入，并输出确定性的 Go 元数据注册代码。

## 来源模型

| 来源 | 作用域 | 运行期效果 |
| --- | --- | --- |
| `//goark-orm:*` 注释 | Go 类型和 Mapper 方法 | 由 `ormgen` 解析，并转换为生成的 `EntityMeta`、`MapperMeta` 和 `StatementMeta`。 |
| `goark-orm` struct tag | 实体字段 | 由 `ormgen` 解析，并转换为生成的 `ColumnMeta`。 |
| XML Mapper 文件 | Mapper 级 result map、SQL、动态 SQL、缓存元数据 | 由 `ormgen` 解析，并嵌入生成元数据。 |
| 生成 Go 文件 | package 级注册函数和类型化辅助对象 | 运行期通过显式 `RegisterGoarkORMMetadata` 调用使用。 |

生成器会按确定性顺序排序实体和 Mapper。Mapper namespace 必须显式声明并全局唯一。

## 注解语法

注解使用带 `goark-orm:` 前缀的行注释：

```go
//goark-orm:entity(table="sys_user")
//goark-orm:mapper(namespace="example.user.UserMapper", xml="mapper/user_mapper.xml")
//goark-orm:select(sql="select id from sys_user where id = #{id}")
```

规则：

- `goark-orm:` 后必须声明注解名称。
- 参数可选，使用括号内 `key=value` 形式。
- 参数之间使用逗号分隔。
- 双引号字符串按 Go string literal 规则解码。
- 重复参数 key 会导致生成失败。
- 参数列表格式非法会导致生成失败。

## Entity 注解

| 注解 | 作用域 | 属性 | 必填 |
| --- | --- | --- | --- |
| `//goark-orm:entity` | struct 类型 | `table`, `keySequence`, `key-sequence` | `table`，除非生成器命名策略可以推导 |

示例：

```go
//goark-orm:entity(table="sys_user", keySequence="sys_user_id_seq")
type User struct {
	ID int64 `goark-orm:"column='id';primary-key=true;id-type='ASSIGN_ID'"`
}
```

规则：

- 目标类型必须是 struct。
- 每个持久化字段都必须声明 `goark-orm` tag。
- 实体至少需要一个主键字段。
- `keySequence` 和 `key-sequence` 是别名。

## Mapper 注解

| 注解 | 作用域 | 属性 | 必填 |
| --- | --- | --- | --- |
| `//goark-orm:mapper` | interface 类型 | `namespace`, `xml` | `namespace` |

示例：

```go
//goark-orm:mapper(namespace="example.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
```

规则：

- 目标类型必须是 interface。
- `namespace` 必须显式声明并全局唯一。
- 存在 `xml` 时，XML 根节点 namespace 必须与 Go Mapper namespace 完全一致。
- 每个 Mapper 方法必须且只能解析到一个 statement 来源：注解 SQL/provider 或 XML。
- XML statement 必须匹配 Mapper 方法名；未被使用的 XML statement 会导致生成失败。
- 支持嵌入具名 interface；循环嵌入和重复嵌入方法会导致生成失败。

## 方法 SQL 注解

| 注解 | 命令 | 必需来源 | 常用属性 |
| --- | --- | --- | --- |
| `//goark-orm:select` | `select` | `sql` 或 `provider` | `statementType`, `affectData`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, `parameters`, `resultSets` |
| `//goark-orm:insert` | `insert` | `sql` 或 `provider` | 通用属性，以及 `useGeneratedKeys`, `keyProperty` |
| `//goark-orm:update` | `update` | `sql` 或 `provider` | 通用属性 |
| `//goark-orm:delete` | `delete` | `sql` 或 `provider` | 通用属性 |
| `//goark-orm:call` | `call` | `sql` 或 `provider` | 通用属性，以及 callable 参数/result-set 元数据 |

Statement 来源规则：

- `sql` 和 `provider` 互斥。
- 同一方法不能声明多个 SQL 注解。
- 同一 Mapper 方法不能同时由注解 SQL 和 XML SQL 定义。
- 注解 SQL 可以包含 `<script>...</script>` 动态 SQL。
- `provider` 引用已注册 SQL provider descriptor 或函数名。

Statement option 属性：

| 属性 | 类型 | 说明 |
| --- | --- | --- |
| `statementType` | string | `PREPARED` 或 `CALLABLE`；`call` 默认是 `CALLABLE`。 |
| `timeout` | duration 或整数秒 | Statement timeout 元数据；必须非负。 |
| `timeoutDuration` | duration 或整数秒 | `timeout` 的别名，并且优先级更高。 |
| `fetchSize` | integer | Fetch hint；必须非负。 |
| `resultSetType` | string | `DEFAULT`, `FORWARD_ONLY`, `SCROLL_INSENSITIVE`, 或 `SCROLL_SENSITIVE`。 |
| `resultOrdered` | bool | ResultMap 有序嵌套结果行提示。 |
| `keyColumn` | string | 生成主键回读列。 |
| `affectData` | bool | 将 `select` 标记为会影响数据，用于缓存和审计语义。 |
| `useGeneratedKeys` | bool | 为写语句启用生成主键行为。 |
| `keyProperty` | string | 接收生成主键数据的实体或参数属性。 |
| `interceptorIgnore` | list | 要跳过的 interceptor 名称，支持逗号、分号或空白分隔。 |

Callable 属性：

| 属性 | 语法 | 说明 |
| --- | --- | --- |
| `parameters` | `name[:mode[:jdbcType[:typeHandler]]]` 列表 | 声明 callable 参数。`mode` 支持 `IN`、`OUT` 或 `INOUT`；默认是 `IN`。 |
| `out` | name 列表 | 将参数标记为 `OUT`。 |
| `inout` | name 列表 | 将参数标记为 `INOUT`。 |
| `resultSets` | `name[:resultType[:resultMap]]` 列表 | 按返回顺序声明命名 result set。 |

方法签名规则：

- 第一个参数必须是 `context.Context`。
- 所有参数都必须具名。
- `orm.PageRequest` 参数会被识别为分页查询参数。
- `orm.ResultHandler[T]` 参数会被识别为流式回调，并要求方法只返回 `error`。
- `select` 返回 `(T, error)`、`([]T, error)`、`(orm.Page[T], error)`、`(*orm.Cursor[T], error)`，或使用 `orm.ResultHandler[T]`。
- `insert`、`update` 和 `delete` 返回 `(int64, error)`。
- `call` 返回 `error` 或 `(orm.CallResult, error)`。
- `OUT` 和 `INOUT` callable 参数必须映射到指针方法参数。
- 当方法暴露 callable result set 时，result set 必须映射到 slice 指针方法参数。

## Struct Tag 语法

Struct tag 使用 `goark-orm` key：

```go
Name string `goark-orm:"column='name';size=64;nullable=false;insert-strategy='not-empty'"`
```

规则：

- 属性之间使用分号分隔。
- 每个属性都必须使用 `key=value`。
- 空属性会导致生成失败。
- 重复属性会导致生成失败。
- 不支持的属性会导致生成失败。
- 字符串值必须使用单引号。
- 布尔值必须严格是 `true` 或 `false`。
- 整数值必须是十进制整数。

## Struct Tag 属性

| 属性 | 类型 | 作用 |
| --- | --- | --- |
| `column` | string | 数据库列名。除非生成器命名策略可以推导，否则必填。 |
| `type` | string | 数据库类型元数据，用于生成 schema model 和兼容性检查。 |
| `default` | string | 列默认值元数据。 |
| `id-type` | string | 主键策略：`auto`, `input`, `assign_id`, `assign_uuid`, `none`，或空值。 |
| `fill` | string | 自动填充时机：`insert`, `update`, 或 `insert_update`。 |
| `type-handler` | string | 本字段使用的命名 type handler。名称必须已注册或被生成配置接受。 |
| `key-column` | string | 数据库生成主键回读列。 |
| `update` | string | 自定义 update 表达式。 |
| `update-expression` | string | 自定义 update 表达式别名；不能与 `update` 同时使用。 |
| `condition` | string | Wrapper 生成 SQL 时使用的自定义条件模板。 |
| `insert-strategy` | string | 生成 insert 时的字段参与策略。 |
| `update-strategy` | string | 生成 update 时的字段参与策略。 |
| `where-strategy` | string | 生成 where 条件时的字段参与策略。 |
| `primary-key` | bool | 标记主键字段。 |
| `auto-increment` | bool | 标记数据库生成主键；要求 `primary-key=true`。 |
| `nullable` | bool | 列可空元数据。 |
| `select` | bool | `false` 表示从默认 select 列表中排除该字段。 |
| `version` | bool | 乐观锁版本字段；每个实体最多一个。 |
| `soft-delete` | bool | 逻辑删除标记字段；每个实体最多一个。 |
| `created-at` | bool | 创建时间元数据；每个实体最多一个。 |
| `updated-at` | bool | 更新时间元数据；每个实体最多一个。 |
| `order-by` | bool | 将字段加入生成的默认排序元数据。 |
| `order-desc` | bool | 使生成的默认排序使用降序。 |
| `transient` | bool | 将字段排除出持久化元数据。 |
| `size` | int | 字符或二进制长度元数据。 |
| `numeric-scale` | int | Decimal scale 元数据。 |
| `order-priority` | int | 生成默认排序时的排序优先级。 |

校验规则：

- `auto-increment=true` 要求 `primary-key=true`。
- `id-type` 要求 `primary-key=true`。
- `auto-increment=true` 与除空值、`none`、`auto` 外的 `id-type` 冲突。
- 实体必须至少有一个主键。
- `version`、`soft-delete`、`created-at` 和 `updated-at` 每类最多允许一个字段。

## XML Mapper 根节点

```xml
<mapper namespace="example.user.UserMapper">
  ...
</mapper>
```

规则：

- 根元素必须是 `mapper`。
- `namespace` 必填。
- 只接受受支持的子元素。
- XML include 在生成期展开。
- 缺失 include、同一 database specificity 下的重复 statement id、include 循环都会导致生成失败。

## XML Cache 元素

| 元素 | 属性 | 说明 |
| --- | --- | --- |
| `cache` | `eviction`, `size`, `flushInterval`, `readOnly`, `blocking` | 启用 Mapper namespace 二级缓存。 |
| `cache-ref` | `namespace` | 复用另一个 namespace 的缓存。 |

XML 布尔属性必须是 `true` 或 `false`。XML 数字属性必须是十进制整数。

## XML Result Map

| 元素 | 属性 | 说明 |
| --- | --- | --- |
| `resultMap` | `id`, `type`, `extends`, `autoMapping` | 声明一个结果对象的映射。 |
| `constructor` | none | 容纳 `idArg` 和 `arg` 映射。 |
| `idArg`, `arg` | `name`, `property`, `column`, `typeHandler` | 构造参数映射。 |
| `id`, `result` | `property`, `column`, `typeHandler` | 标量属性映射。 |
| `association` | `property`, `type`, `javaType`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | 嵌套对象映射、nested select 映射或命名 result set 映射。 |
| `collection` | `property`, `ofType`, `type`, `javaType`, `column`, `resultSet`, `foreignColumn`, `columnPrefix`, `notNullColumn`, `select`, `fetchType` | 嵌套集合映射。 |
| `discriminator` | `column`, `type`, `javaType`, `typeHandler` | 分支选择列。 |
| `case` | `value`, `resultMap`, `resultType`, `type` | Discriminator 分支，可以包含内联子映射。 |

规则：

- `resultMap.id` 必填，并且在当前 Mapper 内唯一。
- `extends` 解析当前 Mapper 内 result map，并检测循环继承。
- 父 result map 字段先合并，子字段后合并。
- `notNullColumn` 是逗号分隔列表。
- `resultSet` 和 `foreignColumn` 用于从命名多结果集映射嵌套对象。
- `select` 和 `fetchType` 描述显式 nested select 和 lazy-loading 元数据。

## XML Statement 元素

| 元素 | 命令 | 属性 |
| --- | --- | --- |
| `select` | `select` | `id`, `resultMap`, `resultType`, `parameterType`, `databaseId`, `affectData`, `useCache`, `flushCache`, `statementType`, `timeout`, `timeoutDuration`, `fetchSize`, `resultSetType`, `resultOrdered`, `keyColumn`, `interceptorIgnore`, `resultSets` |
| `insert` | `insert` | 通用属性，以及 `useGeneratedKeys`, `keyProperty` |
| `update` | `update` | 通用属性 |
| `delete` | `delete` | 通用属性 |
| `call` | `call` | 通用属性，默认 `statementType` 为 `CALLABLE` |

规则：

- `id` 必填。
- `resultMap` 和 `resultType` 互斥。
- `databaseId` 在生成期选择数据库专用 statement。精确匹配优先于默认 statement；重复 specificity 会失败。
- `useCache` 和 `flushCache` 接受 `true` 或 `false`，并成为显式 statement cache policy。
- `timeout` 和 `timeoutDuration` 接受 Go duration 或整数秒。
- `fetchSize` 必须非负。
- `interceptorIgnore` 接受逗号、分号或空白分隔的名称。

嵌套 statement 元数据：

| 元素 | 属性 | 说明 |
| --- | --- | --- |
| `selectKey` | `keyProperty`, `resultType`, `order` | 生成主键查询。`order` 接受 `BEFORE` 或 `AFTER`；默认是 `AFTER`。 |
| `parameter` | `property`, `name`, `mode`, `jdbcType`, `type`, `typeHandler` | Callable 参数。`property` 和 `name` 互为别名；`type` 是 `jdbcType` 的别名。 |
| `resultSet` | `name`, `property`, `resultMap`, `resultType` | Callable result-set 元数据。 |

## 动态 SQL 节点

动态 SQL 可用于 XML statement 和注解 `<script>` 块。

| 节点 | 属性 | 行为 |
| --- | --- | --- |
| `sql` | `id` | 在 `mapper` 下声明 XML 片段；由 `include` 在生成期展开。 |
| `include` | `refid`, `refId` | 插入命名 `sql` 片段。 |
| `if` | `test` | 表达式为 true 时渲染子节点。 |
| `where` | none | 添加 `WHERE`，并移除前置布尔连接符。 |
| `set` | none | 添加 `SET`，并移除尾随逗号。 |
| `trim` | `prefix`, `suffix`, `prefixOverrides`, `suffixOverrides` | 通用前缀/后缀包装器。 |
| `foreach` | `collection`, `item`, `index`, `open`, `close`, `separator`, `nullable` | 展开 slice、array、map 或受支持集合。 |
| `choose` | none | 渲染第一个匹配的 `when`，否则渲染 `otherwise`。 |
| `when` | `test` | `choose` 下的条件分支。 |
| `otherwise` | none | fallback 分支。 |
| `bind` | `name`, `value` | 通过安全表达式引擎创建命名值。 |

表达式引擎是确定性且有边界的。它支持布尔逻辑、比较、算术、集合包含、三元表达式、字面量、列表字面量、参数路径，以及内置集合/字符串 helper。它不能调用任意 Go 函数，也不能修改值。

## SQL 占位符

| 占位符 | 行为 |
| --- | --- |
| `#{name}` | 编译为所选方言占位符，并作为 driver 参数绑定。 |
| `${name}` | 仅当值实现 `RawSQLToken` 时渲染。普通字符串会被拒绝。 |

使用 `NewRawIdentifier`、`NewRawOrderItem` 和 `NewRawOrderBy` 为已知 identifier/order-by 场景创建安全 raw SQL token。

## 生成产物契约

默认生成文件名为 `zz_goark_orm_<package>_gen.go`。

生成 package 包含：

- `RegisterGoarkORMMetadata(registry *orm.Registry) error`
- Mapper 构造函数，例如 `NewUserMapper(session orm.Session) UserMapper`
- 对具备受支持主键元数据的实体生成 BaseMapper 构造函数
- 为生成 BaseMapper 提供 Service 构造函数
- 用于 wrapper 和字段值辅助函数的实体字段常量和 typed fields
- 用于快速实体映射的 RowScanner

当注解、struct tag、XML Mapper 文件、provider 引用或生成器配置变化后，需要重新生成。
