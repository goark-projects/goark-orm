# Provider 示例

默认文档语言为英文：[README.md](README.md)。本文件是中文镜像。

本 package 展示 Provider 驱动的 SQL 构造。Provider 适用于 SQL 依赖运行期上下文、功能开关、租户路由，或静态 Mapper SQL 难以表达的计算型查询形态。

## 文件

| 文件 | 职责 |
| --- | --- |
| [provider_test.go](provider_test.go) | Provider 注册、SQL Builder 输出和校验行为的可执行示例。 |

## 覆盖内容

- `Registry.RegisterSQLProviderDescriptor`。
- Provider 命令和 statement 约束。
- 带 SQL、命名参数和 cache key 的 `SQLSource`。
- 缺失表、缺失列和不安全写语句的 Builder 校验。
- 通过运行期执行方言占位符编译。

## 测试

从仓库根目录执行：

```bash
GOWORK=off go test -count=1 ./examples/provider
```

完整 Provider 参考见 [docs/provider-builder.zh-CN.md](../../docs/provider-builder.zh-CN.md)。
