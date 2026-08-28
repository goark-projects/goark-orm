# 生产级 Demo

生产级 demo 位于 [examples/production](../examples/production)。它是真实 Go package 组织，可以在没有具体数据库驱动和私有凭据的情况下编译和测试。

## 目录结构

| 路径 | 职责 |
| --- | --- |
| `examples/production/goark-orm.json` | 生成器配置。 |
| `examples/production/goark-orm-runtime.json` | 无 DSN 的严格运行期 JSON 配置。 |
| `examples/production/account` | 实体、Mapper 契约、XML Mapper、SQL Provider、生成元数据和填充处理器。 |
| `examples/production/app` | 应用装配、运行期所有权、服务层校验、超时控制和测试。 |

## 展示内容

- 显式实体和 Mapper 元数据。
- XML result map、namespace cache、动态 SQL、statement options 和 `affectData` returning 语句。
- Provider 方式组合 SQL Builder，并做参数类型校验和 cache key。
- 通过运行期配置注册 JSON TypeHandler。
- 通过 `RuntimeConfig` 和 `RuntimeAssembly` 做运行期装配。
- 可选审计 middleware 和 SQL observation hook。
- 服务层资源保护：租户校验、正整数 ID 校验、page size 上限、邮箱查询 limit 上限和 context timeout。
- 测试覆盖生成元数据、Provider SQL 编译、运行期配置装配和应用服务行为。

## 生成

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json --check
```

## 测试

```bash
GOWORK=off go test -count=1 ./examples/production/...
GOWORK=off go test -count=1 ./...
```

## 连接真实数据库

demo 不保存 DSN。调用方拥有的 binary 或测试 harness 应该：

1. 导入具体 `database/sql` 驱动。
2. 打开并调优 `*sql.DB`。
3. 在仓库外执行 migration 或 setup SQL。
4. 调用 `app.Assemble(ctx, app.RuntimeOptions{DB: db, ConfigPath: "examples/production/goark-orm-runtime.json"})`。
5. shutdown 时关闭返回的 runtime。

```go
runtime, err := app.Assemble(ctx, app.RuntimeOptions{
	DB:         db,
	ConfigPath: "examples/production/goark-orm-runtime.json",
})
if err != nil {
	return err
}
defer runtime.Close()

users, err := runtime.Users.ListUsers(ctx, "tenant-a", account.UserStatusActive, orm.NewPageRequest(1, 20))
if err != nil {
	return err
}
_ = users
```

数据库矩阵验证见 [database-matrix.md](database-matrix.md) 和仓库脚本：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db-bench.ps1 -BenchTime 1s
```
