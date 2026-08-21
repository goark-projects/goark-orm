# Goark ORM

Goark ORM 是可独立使用的数据映射模块，同时也可以接入 Goark 生态。目标是提供 Go 原生实体映射、查询构建、事务集成和可测试数据访问边界。

## 当前状态

本仓库已落地第一版 ORM 元数据与生成器基础能力，当前尚未承诺稳定公共 API。已支持：

- 实体 `//goark-orm:entity` 与严格 `goark-orm` struct tag 解析。
- Mapper `//goark-orm:mapper`、`select`、`insert`、`update`、`delete` 方法注解扫描。
- XML Mapper 静态语句、动态 SQL 基础节点、`resultMap`、namespace 一致性校验。
- XML 与注解在同一个 Mapper 接口中混用。
- 生成 `RegisterGoarkORMMetadata`、Mapper 实现和 `orm.Session` 调用代码。
- 独立 `goark-orm` CLI，可不安装 Goark 主 CLI 直接生成代码。
- `database/sql` Session、`Dialect`、`#{name}` 安全参数编译和基础结果扫描。
- XML 动态 SQL 支持 `sql/include`、`if`、`where`、`set`、`trim`、`foreach`、`choose/when/otherwise`。

`goark-database` 事务对接仍按设计文档分阶段推进。

## 模块路径

```text
module goark.dev/orm
```

## 规划边界

- 实体映射、字段元数据和查询构建边界
- 事务集成、错误分类和上下文取消
- 与 goark-database 的连接/事务底座对齐
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

如果项目已经使用 Goark 主 CLI，也可以使用可选包装：

```bash
goark generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
```

## 工程约定

- Go 版本跟随 Goark 生态主线，当前模块声明为 `go 1.25`。
- 代码、脚本、配置和文档统一使用 UTF-8 与 LF。
- Go 代码注释使用标准简体中文，只解释非显而易见的设计意图、边界和失败语义。
- 公共 API 优先显式接口、小结构体和可组合选项，不使用 Java 风格运行时扫描或重代理模型。
- 功能实现必须包含边界条件、错误处理、上下文取消和并发安全复核。

## 许可证

本项目使用 Apache License 2.0。
