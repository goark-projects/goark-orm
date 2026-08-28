# Goark ORM 文档

本目录是 Goark ORM 参考文档集合。默认文档语言为英文。中文镜像使用 `*.zh-CN.md` 后缀。

## 优先阅读

- [仓库 README](../README.md)：项目概览、快速开始、运行期装配和验证命令。
- [功能参考](features.zh-CN.md)：已实现的运行期、生成器、Mapper、缓存、路由、schema 和真实数据库能力。
- [配置参考](configuration.zh-CN.md)：每个生成器和运行期 JSON 字段、可选值、默认值和所有权边界。
- [案例指南](examples.zh-CN.md)：生成 Mapper、XML 映射、Wrapper、Provider、运行期配置、路由、审计和真实数据库验证示例。
- [生产级 Demo](production-demo.zh-CN.md)：生产导向 package 组织、生成器配置、运行期配置、Mapper/Provider 代码、服务校验和测试。

## 运维与发布

- [Database Matrix](database-matrix.md)：方言行为、兼容套件覆盖、环境变量和 benchmark harness。
- [Release Gates](release-gates.md)：本地 build、test、vet、generation、diff 和 benchmark 门禁。
- [API Compatibility](api-compatibility.md)：V1 公共契约和演进规则。
- [Architecture Notes](goark-orm-v1-design.md)：设计边界、元数据流、运行期职责和关键决策。
- [Provider And SQL Builder](provider-builder.md)：Provider 注册、Builder API、cache key、upsert 和行锁。

## 文档规则

- 公共示例使用 `RuntimeConfig`、`RuntimeAssembly` 和 `LoadAndAssembleRuntimeConfig`。
- 文档不保存 DSN、密码、私有 SQL 或生成环境文件。
- core 示例把具体数据库驱动导入保留在调用方测试 harness。
- ORM 保持 Go 原生：显式元数据、生成注册和小型运行期接口。
