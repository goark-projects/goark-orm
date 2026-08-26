# Goark ORM V1 公共 API 兼容策略

## 状态

`goark.dev/orm` 当前公共 API 主版本为 `v1`，运行时通过 `orm.APIVersion` 暴露。

## 稳定范围

以下内容进入 V1 兼容范围：

- `goark.dev/orm` module path。
- `Session`、`ManagedSession`、`StatementSession`、`SQLSession`、`SQLSessionFactory`、`TxSession`、`BatchSession`、`BaseMapper`、`Service`、`QueryWrapper`、`UpdateWrapper`、`Page`、`Lazy`、`Cache`、`TypeHandler`、`StatementInterceptor`、`StatementHandler`、`ParameterHandler`、`ResultSetHandler`、`IdentifierGenerator`、`MetaObjectHandler` 等导出运行时接口和类型。
- `EntityMeta`、`ColumnMeta`、`MapperMeta`、`StatementMeta`、`ResultMapMeta`、`DynamicSQLNode` 等导出元数据结构。
- `ormgen.GenerateSpec`、`ormgen.PackageModel`、`ormgen.EntityModel`、`ormgen.MapperModel`、`ormgen.StatementModel` 和 `goark-orm generate orm` 主命令。
- `//goark-orm:entity`、`//goark-orm:mapper`、`//goark-orm:select`、`//goark-orm:insert`、`//goark-orm:update`、`//goark-orm:delete` 注解前缀和 `goark-orm` struct tag key。

## 演进规则

- 只做向后兼容扩展：新增导出类型、方法、可选字段、可选配置和新适配层。
- 不删除 V1 已导出标识符。
- 不改变 V1 已有函数签名、接口方法签名和错误分类语义。
- 不改变已生成代码的主要入口名称，包括 `RegisterGoarkORMMetadata`、`New<Entity>Mapper`、`New<Entity>BaseMapper` 和 `New<Entity>Service`。
- 不把 Goark core、boot、CLI 或具体数据库驱动加入 `goark.dev/orm` core 的强依赖。

## 允许变化

- 新增 struct 字段，但必须保持零值安全。
- 新增可选配置，但默认值必须保持既有行为。
- 新增错误上下文字段，但 `errors.Is` 和 `errors.As` 分类语义不能破坏。
- 新增生成代码内容，但已存在入口和语义保持兼容。

## 非兼容变化处理

确实需要破坏 V1 契约时，必须进入新的主版本或新增并行 API，不能直接修改 V1 行为。
