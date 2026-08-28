# 生产级 Demo

默认文档语言为英文：[README.md](README.md)。本文件是中文镜像。

本 demo 是面向生产组织方式的账号模块 package 布局。它可以在没有具体数据库驱动和私有凭据的情况下编译和测试。调用方应用负责提供 `*sql.DB`、导入所选驱动、执行迁移，并拥有关闭顺序。

## 目录结构

| 路径 | 职责 |
| --- | --- |
| [goark-orm.json](goark-orm.json) | 账号 package 的生成器配置。 |
| [goark-orm-runtime.json](goark-orm-runtime.json) | 运行期配置：settings、environment、global DB config、typeHandlers、mappers 和 plugins。 |
| [account/model.go](account/model.go) | 实体模型，包含 ID 生成、租户字段、JSON profile、乐观锁、逻辑删除和填充元数据。 |
| [account/mapper.go](account/mapper.go) | Mapper 契约，包含显式 namespace、XML 绑定、分页、`affectData` 和 provider 方法。 |
| [account/mapper/user_mapper.xml](account/mapper/user_mapper.xml) | XML resultMap、namespace 缓存、动态 SQL、语句选项和 PostgreSQL returning 语句。 |
| [account/provider.go](account/provider.go) | Provider 注册和带类型化参数校验的 SQL Builder。 |
| [account/fill.go](account/fill.go) | 用于 insert/update 审计字段的 `MetaObjectHandler`。 |
| [account/zz_goark_orm_account_gen.go](account/zz_goark_orm_account_gen.go) | 生成元数据和 Mapper 实现。 |
| [app/runtime.go](app/runtime.go) | 从显式元数据、运行期 JSON、TypeHandler、审计 middleware 和 SQL 观察完成运行期装配。 |
| [app/user_service.go](app/user_service.go) | 应用级参数校验、超时控制、分页大小上限和 limit 上限。 |
| [app/options.go](app/options.go) | 调用方拥有的运行期和服务选项。 |
| [app/config.go](app/config.go) | Demo 默认值。 |

## 生成

从仓库根目录执行：

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json --check
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json --diff
```

重新写入生成元数据：

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config examples/production/goark-orm.json
```

## 测试

```bash
GOWORK=off go test -count=1 ./examples/production/...
```

## 运行期所有权

Demo 有意不打开真实数据库连接。生产应用应：

1. 在应用二进制或测试 harness 中导入具体 `database/sql` 驱动。
2. 打开并调优 `*sql.DB`。
3. 在本仓库之外执行 schema migration。
4. 使用调用方拥有的 options 调用 `app.Assemble`。
5. 在停机时关闭返回的 runtime。
6. ORM Session 关闭后，由调用方关闭 `*sql.DB`。

```go
runtime, err := app.Assemble(ctx, app.RuntimeOptions{
	DB:         db,
	ConfigPath: "examples/production/goark-orm-runtime.json",
})
if err != nil {
	return err
}
defer runtime.Close()
```

完整文档见 [docs/production-demo.zh-CN.md](../../docs/production-demo.zh-CN.md)。
