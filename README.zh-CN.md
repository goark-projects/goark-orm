# Goark ORM

Goark ORM 是面向 `database/sql` 的 Go 原生数据映射模块。它使用显式元数据注册、稳定代码生成和小型运行期契约，提供 Session、事务、类型处理器、SQL 构建、结果映射、路由、缓存和真实数据库验证能力。

默认文档为英文：[README.md](README.md)。案例说明分别在 [docs/examples.md](docs/examples.md) 和 [docs/examples.zh-CN.md](docs/examples.zh-CN.md)。

## 模块

```text
module goark.dev/orm
```

`orm.APIVersion` 当前为 `v1`。

## 设计边界

- 运行期只使用显式注册的元数据，不扫描 Mapper、XML 或实体。
- 生成 Mapper 只依赖 `orm.Session` 接口，自动提交、事务、路由、批处理和流式查询可以复用同一套生成代码。
- core 不导入具体数据库驱动，真实数据库验证由调用方显式导入驱动并通过环境变量开启。
- Migration 和 DDL 生命周期不属于本模块。
- `${}` 只接受显式 `RawSQLToken`，例如 `RawIdentifier` 和 `RawOrderBy`。
- JSON 处理统一经过内部 JSON codec，底层使用 ByteDance Sonic。

## 当前能力

- `//goark-orm:entity` 和严格 `goark-orm` struct tag 实体元数据。
- `//goark-orm:mapper`、方法级 SQL 注解和 XML Mapper 元数据。
- 生成元数据注册、实体 RowScanner、Mapper 实现、类型化字段常量、`BaseMapper` 工厂和 `Service` 工厂。
- XML 动态 SQL：`sql/include`、`bind`、`if`、`where`、`set`、`trim`、`foreach`、`choose/when/otherwise`。
- 安全表达式执行，覆盖布尔逻辑、比较、算术、三元表达式、集合判断、`empty`、`in/not in` 和白名单只读方法。
- PostgreSQL 和 MySQL 是当前真实支持数据库；MariaDB、SQLite、SQL Server、Oracle 和问号占位符仅保留 SQL 生成方言能力。
- Statement 级 timeout、fetch size、result set type、result ordered、key column、缓存策略和拦截器忽略配置。
- Callable statement，支持 IN、OUT、INOUT 参数、`sql.Out` 绑定和多结果集扫描。
- ResultMap constructor、association、collection、discriminator、nested select、显式 Lazy、column prefix 和 not-null guard。
- Registry / Session 级 TypeHandler，内置 JSON、time、decimal、string、bool、bytes 处理器。
- `BaseMapper`、`Service`、`QueryWrapper`、`UpdateWrapper`、类型化字段、分页、批处理、逻辑删除、乐观锁、主键生成和自动填充。
- SQL Provider 描述注册和 select/insert/update/delete SQL Builder。
- SQLSession middleware 与拦截器，覆盖执行链路、分页、租户、数据权限、动态表、SQL 观察、全表写保护、非法 SQL、只读会话和自定义治理规则。
- 一级缓存、namespace 二级缓存、LRU、并发 miss 合并、缓存统计和事务提交后发布语义。
- 多数据源路由 Session 和路由工厂。
- `ormgen` schema 读取、反向工程、模板渲染、schema drift 和 schema compatibility helper。
- `ormtest` 真实数据库套件，覆盖 ping、setup/cleanup、查询、分页、写语句、批处理、TypeHandler、UPSERT、生成主键回读、行锁和 callable。

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

生成代码：

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
```

注册元数据并创建 Session：

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

使用生成的字段常量和通用 Mapper：

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
```

## CLI 配置

多包生成可以使用可提交的 JSON 配置：

```json
{
  "databaseId": "postgres",
  "typeHandlers": ["json", "decimal"],
  "buildTags": ["enterprise"],
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

CLI 配置只负责源码扫描和文件输出，不连接数据库，也不生成迁移文件。

## 运行期配置

`Configuration` 是直接运行期模型：

```go
config := orm.DefaultConfiguration().
	WithLocalCache(true).
	WithSecondLevelCache(true).
	WithMapUnderscoreToCamelCase(true)

config.Dialect = orm.NewPostgresDialect()
config.LocalCacheScope = orm.LocalCacheScopeSession
config.DefaultExecutorType = orm.ExecutorTypeReuse
config.GlobalConfig.DbConfig.IDType = orm.IDTypeAssignID
config.GlobalConfig.DbConfig.TablePrefix = "sys_"
config.GlobalConfig.DbConfig.InsertStrategy = orm.FieldStrategyNotEmpty
config.GlobalConfig.DbConfig.UpdateStrategy = orm.FieldStrategyNotEmpty

session, err := orm.NewSQLSession(registry, db, nil, orm.WithConfiguration(config))
if err != nil {
	return err
}
```

JSON 配置解码是严格模式，并使用内部 Sonic JSON codec。`LoadAndAssembleMyBatisConfig` 可以一站式读取配置并装配运行期对象：

```go
assembled, err := orm.LoadAndAssembleMyBatisConfig("orm-runtime.json", orm.MyBatisAssembly{
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

## Provider SQL

Provider 必须显式注册，可以组合 SQL Builder：

```go
err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
	"UserSQL.ListByStatus",
	func(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs) (orm.SQLSource, error) {
		return orm.NewSelectSQLBuilder().
			Select("id", "name", "status").
			From("sys_user").
			WhereEq("status", args["status"]).
			OrderByAsc("id").
			Limit(args["limit"]).
			CacheKey("tenant:" + args["tenant"].(string)).
			Build()
	},
	orm.WithSQLProviderCommands(orm.StatementCommandSelect),
	orm.WithSQLProviderStatements("example.user.UserMapper.ListByStatus"),
))
if err != nil {
	return err
}
```

## 事务与批处理

```go
factory, err := orm.NewSQLSessionFactory(registry, db, orm.NewPostgresDialect())
if err != nil {
	return err
}

err = factory.InTx(ctx, nil, func(ctx context.Context, session orm.Session) error {
	batch, err := orm.NewBatchSession(session)
	if err != nil {
		return err
	}
	baseMapper, err := NewUserBaseMapper(batch)
	if err != nil {
		return err
	}
	_, err = baseMapper.UpdateWithWrapper(
		ctx,
		orm.NewUpdateWrapper[User]().
			SetTyped(UserTypedFields.Status, "LOCKED").
			EqTyped(UserTypedFields.ID, int64(7)),
	)
	if err != nil {
		return err
	}
	_, err = batch.Flush(ctx)
	return err
})
```

## Schema 工具

`ormgen` 可以通过 `database/sql` 读取真实 schema，构造生成模型，渲染 Go 源码，并可选对比已注册元数据：

```go
report, err := ormgen.ValidateSQLSchemaCompatibility(ctx, ormgen.SQLSchemaCompatibilityConfig{
	DBType:      orm.DbTypePostgres,
	SQLQueryer: db,
	Schema:     "public",
	Tables:     []string{"sys_user"},
	PackageName: "user",
	Registry:   registry,
})
if err != nil {
	return err
}
_ = report.Source
```

## 真实数据库兼容性

真实库套件只有在调用方提供驱动和 DSN 后才执行。标准套件当前只支持 PostgreSQL 和 MySQL：

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

标准 PostgreSQL/MySQL 兼容矩阵已经包含 callable statement 覆盖，详见 [docs/database-matrix.md](docs/database-matrix.md)。

## 本地验证

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
git diff --check
```

维护者发布前可以执行本地门禁：

```bash
GOWORK=off ./scripts/verify-release.sh
```

## 更多文档

- [案例说明](docs/examples.zh-CN.md)
- [API Compatibility](docs/api-compatibility.md)
- [Database Matrix](docs/database-matrix.md)
- [Provider And SQL Builder](docs/provider-builder.md)
- [Architecture Notes](docs/goark-orm-v1-design.md)
- [Release Gates](docs/release-gates.md)

## 许可证

Apache License 2.0。
