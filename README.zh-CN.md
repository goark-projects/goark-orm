# Goark ORM

Goark ORM 是面向 `database/sql` 的 Go 原生数据映射模块。它使用显式元数据、确定性代码生成、小型运行期契约和可复用真实数据库验证工具，服务于生产级 Go 应用。

默认文档语言是英文：[README.md](README.md)。中文文档作为镜像维护在本文件和 `*.zh-CN.md` 指南中。

## 模块

```text
module goark.dev/orm
```

`orm.APIVersion` 当前为 `v1`。

## 设计规则

- 运行期元数据通过生成代码显式注册。
- 源文件、XML Mapper 文件和 struct tag 是生成器输入；运行期不扫描这些文件。
- 生成 Mapper 实现只依赖 `orm.Session`，自动提交 Session、事务 Session、批处理 Session、路由 Session 和流式签名共用同一生成接口。
- core 包不导入具体数据库驱动。应用和测试 harness 负责驱动导入、DSN、连接池、schema setup 和 cleanup。
- Schema migration 和 DDL 生命周期不属于本模块。
- `${}` 原始 SQL 替换只接受显式 `RawSQLToken`，例如 `RawIdentifier` 和 `RawOrderBy`。
- JSON 处理统一经过内部 Sonic-backed codec。

## 能力地图

| 领域 | 当前能力 |
| --- | --- |
| 实体映射 | `//goark-orm:entity`、严格 `goark-orm` struct tag、类型化字段常量、生成 RowScanner |
| Mapper 映射 | `//goark-orm:mapper`、方法级 SQL 注解、XML Mapper 文件、Provider 语句 |
| 动态 SQL | `sql/include`、`bind`、`if`、`where`、`set`、`trim`、`foreach`、`choose/when/otherwise`、安全表达式执行 |
| CRUD 辅助 | `BaseMapper`、`Service`、链式查询/更新 API、类型化 wrapper、分页、字段值查询、ID 列表 |
| 写入语义 | 批处理写入、生成主键、insert/update/where 字段策略、自动填充、乐观锁、逻辑删除 |
| 结果映射 | Constructor arg、association、collection、discriminator、nested select、命名 result set、显式懒加载 |
| 运行期扩展 | TypeHandler、SQL Provider、拦截器、handler middleware、审计 middleware、缓存 SPI |
| 治理 | 全表写保护、非法 SQL 规则、只读 Session、租户条件、数据权限、动态表名、SQL 观察 |
| 缓存 | Session 一级缓存、namespace 二级缓存、LRU、TTL、并发 miss 合并、事务感知发布 |
| 路由 | 显式数据源选择、读写分离、按语句路由 |
| 方言 | PostgreSQL、MySQL、MariaDB、SQLite、SQL Server、Oracle，以及问号占位符 SQL 生成方言 |
| 工具 | CLI 生成、多 package 生成配置、schema introspection、反向工程、drift 检查、真实数据库套件和 benchmark |

## 快速开始

声明实体和 Mapper：

```go
package user

import (
	"context"

	orm "goark.dev/orm"
)

//goark-orm:entity(table="sys_user")
type User struct {
	ID     int64  `goark-orm:"column='id';primary-key=true;auto-increment=true"`
	Name   string `goark-orm:"column='name';size=64;nullable=false"`
	Status string `goark-orm:"column='status';size=32;nullable=false"`
}

//goark-orm:mapper(namespace="example.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id, name, status from sys_user where id = #{id}")
	FindByID(ctx context.Context, id int64) (*User, error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListByStatus(ctx context.Context, status string, page orm.PageRequest) (orm.Page[User], error)
}
```

生成 Mapper 代码：

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
```

注册元数据并使用 Session：

```go
registry := orm.NewRegistry()
if err := RegisterGoarkORMMetadata(registry); err != nil {
	return err
}
if err := registry.Validate(); err != nil {
	return err
}

session, err := orm.NewSQLSession(registry, db, orm.NewPostgresDialect())
if err != nil {
	return err
}

mapper := NewUserMapper(session)
user, err := mapper.FindByID(ctx, 7)
if err != nil {
	return err
}
_ = user
```

使用生成字段常量和通用 Mapper：

```go
baseMapper, err := NewUserBaseMapper(session)
if err != nil {
	return err
}

page, err := baseMapper.SelectPage(
	ctx,
	orm.NewPageRequest(1, 20),
	orm.NewQueryWrapper[User]().
		Eq(UserFields.Status, "ACTIVE").
		OrderByAsc(UserFields.ID),
)
if err != nil {
	return err
}
_ = page

names, err := orm.SelectFieldValues(
	ctx,
	baseMapper,
	UserTypedFields.Name,
	orm.NewQueryWrapper[User]().EqTyped(UserTypedFields.Status, "ACTIVE"),
)
if err != nil {
	return err
}
_ = names
```

## 生成器配置

生成可由可提交 JSON 文件驱动。解码器为严格模式，并使用内部 Sonic-backed JSON codec。

```json
{
  "databaseId": "postgres",
  "typeHandlers": ["json", "decimal"],
  "buildTags": ["enterprise"],
  "naming": {
    "table": "snake_case",
    "column": "snake_case",
    "tablePrefix": "sys_"
  },
  "packages": [
    {
      "dir": "internal/user",
      "output": "internal/user/zz_goark_orm_user_gen.go"
    },
    {
      "dir": "internal/order"
    }
  ]
}
```

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json --check
GOWORK=off go run ./cmd/goark-orm generate orm --config goark-orm.json --diff
```

生成器配置只控制源码扫描和文件输出，不连接数据库，也不生成 migration。

## 运行期配置

`Configuration` 是直接运行期模型。加载 JSON 配置时使用 `RuntimeConfig`：

```json
{
  "settings": {
    "cacheEnabled": true,
    "localCacheEnabled": true,
    "localCacheScope": "SESSION",
    "mapUnderscoreToCamelCase": true,
    "defaultExecutorType": "REUSE",
    "preparedStatementCacheSize": 256,
    "defaultStatementTimeout": "2s",
    "defaultFetchSize": 512,
    "defaultResultSetType": "FORWARD_ONLY",
    "nullableOnForEach": true,
    "shrinkWhitespacesInSql": true,
    "jdbcTypeForNull": "OTHER",
    "autoMappingBehavior": "FULL",
    "autoMappingUnknownColumnBehavior": "NONE",
    "databaseId": "postgres"
  },
  "environment": {
    "id": "production",
    "dbType": "postgres"
  },
  "global": {
    "dbConfig": {
      "idType": "assign_id",
      "tablePrefix": "sys_",
      "logicDeleteField": "Deleted",
      "logicDeleteValue": true,
      "logicNotDeleteValue": false,
      "insertStrategy": "not_empty",
      "updateStrategy": "not_null",
      "whereStrategy": "not_zero"
    }
  },
  "typeHandlers": [
    {
      "name": "json"
    }
  ],
  "mappers": [
    {
      "namespace": "example.user.UserMapper"
    }
  ],
  "plugins": [
    {
      "name": "blockAttack",
      "order": 10
    }
  ]
}
```

```go
assembled, err := orm.LoadAndAssembleRuntimeConfig("orm-runtime.json", orm.RuntimeAssembly{
	Registry: registry,
	DB:       db,
	TypeHandlers: map[string]orm.TypeHandler{
		"json": orm.NewJSONTypeHandler(),
	},
})
if err != nil {
	return err
}
session := assembled.Session
defer session.Close()
_ = session
```

完整生成器和运行期配置见 [docs/configuration.zh-CN.md](docs/configuration.zh-CN.md)。

## Boot 风格装配

`goark.dev/orm/ormboot` 提供小型装配边界，便于接入应用容器或启动生命周期。该适配器不会让 ORM 运行时依赖框架包。

```go
assembler, err := ormboot.New(ormboot.Config{
	DB:            db,
	RuntimeConfig: config,
	MetadataRegistrars: []ormboot.MetadataRegistrar{
		RegisterGoarkORMMetadata,
	},
})
if err != nil {
	return err
}
runtime, err := assembler.Assemble(ctx)
if err != nil {
	return err
}
defer runtime.Close()
factory := runtime.SessionFactory()
_ = factory
```

适配器只管理自己创建的 ORM Session。驱动导入、`*sql.DB` 生命周期和事务管理器集成仍由调用方负责。

## 真实数据库验证

真实数据库套件只有在调用方提供驱动和 DSN 后才执行。标准套件当前支持 PostgreSQL、MySQL、MariaDB、SQLite、SQL Server 和 Oracle。

```go
package user_test

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"goark.dev/orm/ormtest"
)

func TestORMDatabaseCompatibility(t *testing.T) {
	ormtest.RunCompatibilitySuiteFromEnv(t)
}
```

```bash
GOARK_ORM_INTEGRATION_DRIVER=postgres \
GOARK_ORM_INTEGRATION_DSN='postgres://user:pass@127.0.0.1:5432/goark?sslmode=disable' \
GOARK_ORM_INTEGRATION_DBTYPE=postgres \
GOWORK=off go test -run TestORMDatabaseCompatibility ./...
```

矩阵覆盖 CRUD、分页、批处理、TypeHandler 往返、原生 JSON 列、UPSERT、生成主键回读、行锁 smoke path，以及具备可移植驱动行为的 callable statement 路径。详见 [docs/database-matrix.md](docs/database-matrix.md)。

## 本地验证

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
powershell -ExecutionPolicy Bypass -File scripts/verify-bench.ps1 -EnforceTime
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-real-db-bench.ps1 -BenchTime 1s
git diff --check
```

发布维护者可执行本地 release gate：

```bash
GOWORK=off ./scripts/verify-release.sh
```

## 文档

- [英文文档索引](docs/README.md)
- [中文文档索引](docs/README.zh-CN.md)
- [功能参考](docs/features.zh-CN.md)
- [配置参考](docs/configuration.zh-CN.md)
- [案例指南](docs/examples.zh-CN.md)
- [API 兼容性](docs/api-compatibility.md)
- [数据库矩阵](docs/database-matrix.md)
- [Provider 与 SQL Builder](docs/provider-builder.md)
- [架构说明](docs/goark-orm-v1-design.md)
- [Release Gates](docs/release-gates.md)

## 许可证

Apache License 2.0。
