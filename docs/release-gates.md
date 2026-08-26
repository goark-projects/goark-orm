# Goark ORM 本地发布门禁

## 目标

`goark-orm` 的发布门禁由维护者在本地或发布机显式执行，不在仓库内维护远程流水线。门禁只验证 ORM core、生成器、示例工程和 benchmark smoke，不负责数据库迁移，也不保存私有 DSN。

## 标准命令

```bash
GOWORK=off ./scripts/verify-release.sh
```

脚本执行以下检查：

- `gofmt -l` 检查所有已跟踪和未忽略的 Go 文件。
- `goark-orm generate orm --dir examples/minimal --check` 验证示例工程生成文件未过期。
- `go test -count=1 ./...` 跑全量单元测试和示例 smoke。
- `go vet ./...` 跑标准静态检查。
- `git diff --check` 检查当前工作树空白问题。
- 固定 `-benchtime=100x` 跑核心 benchmark smoke，验证 benchmark 可编译、可运行。

需要更长 benchmark 可通过环境变量覆盖：

```bash
GOARK_ORM_BENCHTIME=1s GOWORK=off ./scripts/verify-release.sh
```

## 真实数据库验证

真实数据库验证不进入默认发布脚本，避免在 core 仓库固化驱动、凭据或私有 SQL。需要验证 PG/MySQL 时，在临时测试包中 blank import 对应 driver，并通过 `ormtest.RunCompatibilitySuiteFromEnv` 显式执行。标准环境变量和可回滚兼容矩阵见 `docs/database-matrix.md`。
