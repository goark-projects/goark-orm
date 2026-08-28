# Goark ORM 版本变更说明

## [Unreleased]

### 文档

- 围绕英文优先的中英双语指南重写文档体系。
- 新增注解、struct tag、XML Mapper 和动态 SQL 的独立参考文档。
- 新增示例工作区、最小示例、Provider 示例和生产级 Demo 的双语 README。
- 扩展生产级 Demo 文档，覆盖配置、运行期装配、服务边界、参数校验和验证命令。
- 移除公共文档中形似凭据的内联 DSN 示例；真实数据库执行只说明通过环境变量注入，不在仓库保存示例密钥。

## [v0.0.1] - 2026-08-28

### 发布定位

`v0.0.1` 是 Goark ORM 的首个公开 Go module 版本，模块路径为 `goark.dev/orm`。

该版本面向生产级 Go 服务的数据访问层，采用显式元数据、确定性代码生成、`database/sql` 标准接口、小型运行期契约和低反射执行路径。运行期不依赖 Goark core、boot 或 CLI，应用侧继续拥有数据库驱动、连接池、事务边界、Schema 迁移和部署配置的控制权。

### 核心能力

- 支持 `//goark-orm:entity`、`//goark-orm:mapper` 和严格的 `goark-orm` struct tag。
- 支持注解 Mapper、XML Mapper、Provider SQL 和统一 `StatementMeta` 执行模型。
- 支持动态 SQL：`sql/include`、`bind`、`if`、`where`、`set`、`trim`、`foreach`、`choose/when/otherwise`。
- 支持 `BaseMapper`、`Service`、链式查询/更新、类型安全 Wrapper、分页、字段值查询和 ID 列表查询。
- 支持批处理、事务 Session、路由 Session、游标流式查询和显式 Lazy 延迟加载。
- 支持 ResultMap constructor、association、collection、discriminator、嵌套查询和多结果集映射。
- 支持一级缓存、Mapper namespace 二级缓存、LRU、TTL、阻塞式缓存击穿合并和事务感知发布。
- 支持 TypeHandler、SQL Provider、拦截器、Handler middleware、审计 middleware 和 SQL 观察扩展。
- 支持 PostgreSQL、MySQL、MariaDB、SQLite、SQL Server、Oracle 和 question-placeholder SQL 生成方言。
- 提供 `ormgen` 生成器、schema 反向工程、schema drift 检测、真实数据库兼容套件和 benchmark harness。

### 架构与包边界

- 根包 `goark.dev/orm` 保持为稳定公共 API 门面。
- 核心运行期实现归入 `goark.dev/orm/internal/runtime`，外部调用方不应依赖内部实现布局。
- `audit`、`dbkit`、`ormboot`、`ormgen`、`ormtest` 保持独立子包职责。
- 公开契约继续使用 `ModulePath = "goark.dev/orm"` 和 `APIVersion = "v1"`。
- 生成的 Mapper 继续只依赖 `orm.Session` 等公开接口，保持自动提交、事务、批处理、路由和流式查询的一致调用面。

### 性能与工程质量

- 动态 SQL 表达式使用缓存计划，避免每次执行重复解析。
- SQL placeholder 编译、SQL tail 扫描、ResultMap fallback、关联映射和查询扫描路径已做分配与热点优化。
- `REUSE` 预编译语句缓存支持边界控制，避免长生命周期 Session 持续保留无限 SQL 形态。
- 生成器支持 build tag 感知扫描、原子写入、未变化文件跳过、`--check` 和 `--diff`。
- 根包运行期实现迁入 `internal/runtime` 后，性能 benchmark 直接覆盖核心热路径。

### 发布前验证

本版本发布前已通过以下本地门禁：

- `GOWORK=off go test -count=1 ./...`
- `GOWORK=off go test -race -count=1 ./...`
- `GOWORK=off go vet ./...`
- `gofmt -l .`
- `git diff --check`
- `GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --check`
- `GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --diff`
- `powershell -ExecutionPolicy Bypass -File scripts\verify-bench.ps1 -EnforceTime`
- `powershell -ExecutionPolicy Bypass -File scripts\verify-real-db.ps1`

说明：当前发布环境未配置真实数据库 DSN，因此 `verify-real-db.ps1` 只验证了临时驱动 harness 的编译与按数据库跳过逻辑。需要真实数据库验收时，应配置 `GOARK_ORM_POSTGRES_DSN`、`GOARK_ORM_MYSQL_DSN`、`GOARK_ORM_MARIADB_DSN`、`GOARK_ORM_SQLITE_DSN`、`GOARK_ORM_SQLSERVER_DSN` 或 `GOARK_ORM_ORACLE_DSN` 后重新运行。

### 安装

```bash
go get goark.dev/orm@v0.0.1
```

### 已知边界

- 该模块不管理 Schema 迁移和 DDL 生命周期。
- core 包不导入具体数据库驱动，驱动导入由应用或测试 harness 负责。
- 动态 SQL 表达式是 Go 原生安全子集，不追求完整 OGNL 兼容。
- `${}` 原始 SQL 替换只接受显式 `RawSQLToken`，普通字符串不会作为原始 SQL 拼接。
- 当前为 `v0.0.x` 初始版本，公共 API 已按 V1 兼容面约束，但仍处于 pre-1.0 演进阶段。
