# Goark ORM 示例

默认文档语言为英文：[README.md](README.md)。本文件是中文镜像。

这些示例都是真实 Go package，并被测试和生成器检查使用。core 模块不会保存 DSN、私有 SQL、凭据或数据库驱动导入。

## 示例地图

| 路径 | 目的 | 验证 |
| --- | --- | --- |
| [minimal](minimal) | 最小生成 Mapper 和实体契约。 | 在本目录执行 `GOWORK=off go run ../cmd/goark-orm generate orm --dir minimal --check`，或从仓库根目录执行 release gate。 |
| [provider](provider) | 通过 package 测试展示 Provider 和 SQL Builder。 | 从仓库根目录执行 `GOWORK=off go test -count=1 ./examples/provider`。 |
| [production](production) | 面向生产组织方式的账号模块和应用装配 demo。 | 从仓库根目录执行 `GOWORK=off go test -count=1 ./examples/production/...`。 |

## 边界

- 示例只导入 `goark.dev/orm` 相关包。
- 具体数据库驱动属于调用方二进制或测试 harness。
- Schema migration 和 DDL 生命周期不放入示例。
- 生成文件已提交，方便 `--check` 检测元数据是否过期。

## 继续阅读

- [docs/examples.zh-CN.md](../docs/examples.zh-CN.md)
- [docs/production-demo.zh-CN.md](../docs/production-demo.zh-CN.md)
- [docs/annotations.zh-CN.md](../docs/annotations.zh-CN.md)
