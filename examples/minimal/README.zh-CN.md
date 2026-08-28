# 最小示例

默认文档语言为英文：[README.md](README.md)。本文件是中文镜像。

本 package 展示最小可用的 Goark ORM 工作流：

- 用 `//goark-orm:entity` 声明实体。
- 用显式 namespace 声明 Mapper interface。
- 使用方法注解 SQL。
- 提交生成元数据 `zz_goark_orm_minimal_gen.go`。
- 通过 `--check` 和 package 测试验证生成输出。

## 文件

| 文件 | 职责 |
| --- | --- |
| [mapper.go](mapper.go) | 实体、Mapper interface、注解 SQL 和字段 tag。 |
| [zz_goark_orm_minimal_gen.go](zz_goark_orm_minimal_gen.go) | 生成元数据、Mapper 实现、字段 helper 和 RowScanner。 |
| [minimal_test.go](minimal_test.go) | 生成元数据和 Mapper 行为 smoke test。 |

## 生成

从仓库根目录执行：

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --check
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --diff
```

重新写入生成文件：

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir examples/minimal --output examples/minimal/zz_goark_orm_minimal_gen.go
```

## 测试

```bash
GOWORK=off go test -count=1 ./examples/minimal
```
