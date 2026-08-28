# MyBatis / MyBatis-Plus Go 化迁移矩阵

## 定位

Goark ORM 对标 MyBatis 和 MyBatis-Plus 的核心体验，但实现边界保持 Go-native：显式元数据、生成代码、`database/sql`、可组合接口和可测真实库套件。运行期不做 Java 式包扫描、动态代理、Hibernate/JPA 持久化上下文，也不开放无限制 OGNL 或任意 `${}` 字符串替换。

## 能力映射

| Java 侧能力 | Goark ORM 对应能力 | 说明 |
| --- | --- | --- |
| MyBatis Mapper 接口 | `//goark-orm:mapper` + 生成 Mapper 实现 | namespace 必须显式指定，XML 和注解统一进入 `StatementMeta` |
| XML 动态 SQL | `DynamicSQLNode` 生成模型 | 支持 `if/where/set/trim/foreach/choose/sql/include/bind` |
| MyBatis TypeHandler | `orm.TypeHandler` | 内置 JSON/time/decimal/string/bool/bytes，JSON 统一走 Sonic-backed `internal/jsoncodec` |
| ResultMap | `ResultMapMeta` + 生成/注册元数据 | 支持 constructor、association、collection、discriminator、nested select、resultSets |
| SqlSession | `orm.SQLSession` / `orm.SQLSessionFactory` | 自动提交、事务、批处理、路由和流式查询共享生成 Mapper 接口 |
| BaseMapper | `orm.BaseMapper[T, ID]` | 覆盖 CRUD、分页、逻辑删除、乐观锁、自动填充、批处理、upsert |
| IService | `orm.Service[T, ID]` | 提供 Service 层布尔语义和链式查询/更新入口 |
| DbKit / SimpleQuery | `goark.dev/orm/dbkit` | 轻量门面，不引入全局状态 |
| LambdaQueryWrapper | 生成的 `TypedFields` + `QueryWrapper` | Go 编译期约束字段所属实体；值类型用 `EqTypedValue/InTypedValues` 等自由函数约束 |
| listObjs/getObj | `SelectFieldValues/ListFieldValues/GetFieldValue` | 返回强类型 `[]V` 或 `V`，避免业务层 `[]any` 断言 |
| listByIds/selectObjs | `ListByIDs`、`SelectIDs/ListIDs/QueryChain.IDs` | ID 返回 `[]ID`，无需手动转换 |
| saveBatch | `BatchSession` 或 `InsertBatchSize` | 保留显式 flush，便于错误定位和事务控制 |
| 高吞吐批量写入 | `NewMultiRowInsertSQLBuilder` | 真实库 benchmark 中与顺序 batch 分开测量 |
| 分页插件 | `QueryPageStatement` / `SelectPage` / `Page` | 方言负责 limit/offset 或 SQL Server fetch 语法 |
| tenant/data permission/dynamic table | Session interceptor / middleware | 插件职责独立，按 Session 显式安装 |
| mybatis-spring 装配 | `ormboot` | 适配器只负责 ORM 装配；驱动、`*sql.DB` 生命周期和事务管理器归应用 |

## 迁移建议

1. 先为每个 Java Mapper 明确 namespace，并在 Go 侧使用 `//goark-orm:mapper(namespace="...")` 保持唯一性。
2. 实体 tag 使用 `goark-orm:"column='...';primary-key=true"` 形式，不使用缩写 flag。
3. 能生成的字段常量、RowScanner、BaseMapper/Service 工厂全部生成，不在运行期反射扫描。
4. Java `LambdaQueryWrapper<User>::getName` 迁移为 `UserTypedFields.Name`。
5. Java `listObjs` 迁移为 `orm.SelectFieldValues(ctx, mapper, UserTypedFields.Name, wrapper)` 或 `orm.ListFieldValues(ctx, service, ...)`。
6. JSON 字段统一注册 `orm.NewJSONTypeHandler()`；真实库列优先使用数据库原生 JSON 能力或 JSON 校验约束。
7. 性能验证按层执行：`go test ./...` 验证逻辑，`scripts/verify-bench.ps1 -EnforceTime` 验证本地热点，`scripts/verify-real-db.ps1` 和 `scripts/verify-real-db-bench.ps1` 验证驱动与数据库真实行为。

## 性能边界

- 热路径优先使用生成 RowScanner、参数预编译、显式 TypeHandler 和低反射映射。
- 动态 SQL 表达式采用安全子集和缓存后的执行计划，避免每次运行完整解析。
- 多行插入优先使用 `NewMultiRowInsertSQLBuilder`，顺序 `BatchSession` 保留给需要逐条语义或驱动限制的路径。
- 核心模块不导入驱动，不持有全局连接池；连接池大小、网络拓扑、数据库参数和索引由应用与测试 harness 控制。
- 真实库 benchmark 的绝对耗时取决于网络和数据库配置，应在同一机器、同一数据库版本和同一驱动版本下比较。

## 已知非目标

- 不提供 Java 动态代理式 Mapper 扫描。
- 不支持无限制 OGNL，也不允许普通字符串进入 `${}`。
- 不内置迁移工具或 DDL 生命周期管理。
- 不实现 JPA 一级持久化上下文、脏检查或透明懒加载。
