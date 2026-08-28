# Goark ORM 文档

本目录是 Goark ORM 参考文档集合。默认文档语言为英文。中文镜像使用 `*.zh-CN.md` 后缀。

## 优先阅读

- [仓库 README](../README.md)：项目概览、快速开始、运行期装配和验证命令。
- [功能参考](features.zh-CN.md)：已实现的运行期、生成器、Mapper、缓存、路由、schema 和真实数据库能力。
- [配置参考](configuration.zh-CN.md)：每个生成器字段、运行期 JSON 字段、Go-only 装配字段、可选值、默认值和所有权边界。
- [注解、Tag 与 XML Mapper 参考](annotations.zh-CN.md)：每个生成器注解、struct tag 属性、XML Mapper 元素、语句选项和动态 SQL 节点。
- [案例指南](examples.zh-CN.md)：生成 Mapper、XML 映射、Wrapper、Provider、运行期配置、路由、审计和真实数据库验证示例。
- [生产级 Demo](production-demo.zh-CN.md)：生产导向 package 组织、生成器配置、运行期配置、Mapper/Provider 代码、服务校验和测试。

## 运维与发布

- [版本变更说明](../CHANGELOG.zh-CN.md)：已发布版本的能力、验证范围和已知边界。
- [数据库矩阵](database-matrix.zh-CN.md)：方言行为、兼容套件覆盖、环境变量和 benchmark harness。
- [Release Gates](release-gates.zh-CN.md)：本地 build、test、vet、generation、diff 和 benchmark 门禁。
- [API 兼容性](api-compatibility.zh-CN.md)：V1 公共契约和演进规则。
- [架构说明](goark-orm-v1-design.zh-CN.md)：设计边界、元数据流、运行期职责和关键决策。
- [Provider 与 SQL Builder](provider-builder.zh-CN.md)：Provider 注册、Builder API、cache key、upsert 和行锁。
- [英文版本变更说明](../CHANGELOG.md)：英文默认 changelog。
- [示例工作区](../examples/README.zh-CN.md)：示例 package 地图和验证命令。

## 文档规则

- 公共示例使用 `RuntimeConfig`、`RuntimeAssembly` 和 `LoadAndAssembleRuntimeConfig`。
- 文档不保存 DSN、密码、私有 SQL 或生成环境文件。
- core 示例把具体数据库驱动导入保留在调用方测试 harness。
- ORM 保持 Go 原生：显式元数据、生成注册和小型运行期接口。
- 每个公开文档能力都必须能回到源码、生成示例或 package 测试；不把猜测中的功能写进公共文档。
