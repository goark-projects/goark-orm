# Goark ORM 本地发布门禁

## 范围

发布门禁由维护者在本地工作站或发布主机执行。仓库没有为本模块定义远程 CI。门禁校验 core runtime、generator、examples、API contracts 和性能 smoke tests。

门禁不执行 migration，不保存私有 DSN，也不把具体数据库驱动导入 core packages。

## 标准门禁

```bash
GOWORK=off ./scripts/verify-release.sh
```

该脚本会执行：

- 对已跟踪且未被忽略的 Go 文件运行 `gofmt -l`。
- 运行 `goark-orm generate orm --dir examples/minimal --check`，确认生成示例文件保持最新。
- `go test -count=1 ./...`。
- `go vet ./...`。
- `git diff --check`。
- 在 `./internal/runtime` 上使用固定 `-benchtime=100x` 的 core 性能 smoke tests。

需要更长性能运行时：

```bash
GOARK_ORM_BENCHTIME=1s GOWORK=off ./scripts/verify-release.sh
```

## 性能阈值门禁

当变更涉及 SQL 编译、动态 SQL、wrapper、扫描、TypeHandler、cache key 生成或生成 Mapper 代码时，使用 PowerShell 阈值门禁：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-bench.ps1
```

阈值文件为 [../scripts/benchmark-thresholds.json](../scripts/benchmark-thresholds.json)。门禁始终强制校验 `B/op` 和 `allocs/op`。在稳定本地或发布主机上可追加 `-EnforceTime`，同时强制校验 `ns/op`：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-bench.ps1 -EnforceTime
```

当前阈值集合运行在 `./internal/runtime` 上，并包含动态 SQL 渲染、SQLSession 扫描路径、生成 row scanner 和 TypeHandler-backed result mapping 的明确分配预算。

## 真实数据库验证

真实数据库验证不属于默认门禁。使用 `scripts/verify-real-db.ps1` 创建临时驱动 harness，并在不把具体驱动导入 core packages 的前提下运行 PostgreSQL、MySQL、MariaDB、SQLite、SQL Server 和 Oracle 兼容性矩阵。

使用 `scripts/verify-real-db-bench.ps1` 运行对应的真实数据库 benchmark 矩阵。Benchmark 套件测量 prepared query reuse、生成 row scanner、ResultMap JSON mapping、单行 insert、多行 insert、batch insert 和原生 upsert。`ns/op` 受环境影响明显，因为网络、存储、数据库配置和驱动行为会主导绝对延迟；请在同一发布主机上做重复运行对比。

配置 DSN 后，PostgreSQL 和 MySQL 会进入标准本地矩阵。MariaDB 默认使用 MySQL-compatible driver 路径。当 `GOARK_ORM_SQLSERVER_DSN` 为空时，SQL Server 可以用 `GOARK_ORM_SQLSERVER_ADMIN_DSN` 创建目标数据库。SQLite 是可选项，必须同时提供 `GOARK_ORM_SQLITE_DSN` 和 `GOARK_ORM_SQLITE_IMPORT`；其标准套件跳过 callable statements，因为 Go `database/sql` SQLite drivers 不暴露可移植存储过程模型。

标准矩阵和环境变量见 [database-matrix.zh-CN.md](database-matrix.zh-CN.md)。
