# Goark ORM Examples

This guide shows the common runtime and generator paths that are implemented in the current codebase. Examples keep driver registration, DDL, and private SQL outside the core repository.

## Minimal Generated Mapper

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

```bash
GOWORK=off go run ./cmd/goark-orm generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
```

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

## XML Mapper

```go
//goark-orm:mapper(namespace="example.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
	List(ctx context.Context, status string) ([]User, error)
}
```

```xml
<mapper namespace="example.user.UserMapper">
  <resultMap id="UserResult" type="User">
    <id property="ID" column="id"/>
    <result property="Name" column="name"/>
    <result property="Status" column="status"/>
  </resultMap>

  <select id="FindByID" resultMap="UserResult">
    select id, name, status
    from sys_user
    where id = #{id}
  </select>

  <select id="List" resultMap="UserResult">
    select id, name, status
    from sys_user
    <where>
      <if test="status != nil and status != ''">
        status = #{status}
      </if>
    </where>
  </select>
</mapper>
```

Use `affectData="true"` for select-style statements that modify data, such as database-specific `RETURNING` workflows. The runtime treats them as data-affecting statements for cache flushing and audit middleware defaults.

```xml
<select id="ArchiveReturning" resultMap="UserResult" affectData="true" flushCache="true" useCache="false">
  update sys_user
  set status = 'ARCHIVED'
  where id = #{id}
  returning id, name, status
</select>
```

Named `resultSets` can map nested objects without Java-style runtime proxies:

```xml
<resultMap id="UserWithRoles" type="User">
  <id property="ID" column="id"/>
  <result property="Name" column="name"/>
  <collection property="Roles" ofType="Role" resultSet="roles" column="id" foreignColumn="user_id">
    <id property="ID" column="id"/>
    <result property="Name" column="name"/>
  </collection>
</resultMap>

<select id="FindWithRoles" resultMap="UserWithRoles" resultSets="users,roles">
  select id, name from sys_user where id = #{id};
  select id, user_id, name from sys_role where user_id = #{id};
</select>
```

## BaseMapper And Service

Generated single-primary-key entities receive `New<Entity>BaseMapper` and `New<Entity>Service` factories.

```go
baseMapper, err := NewUserBaseMapper(session)
if err != nil {
	return err
}

records, err := baseMapper.SelectList(
	ctx,
	orm.NewQueryWrapper[User]().
		Eq(UserFields.Status, "ACTIVE").
		OrderByDesc(UserFields.ID),
)
if err != nil {
	return err
}
_ = records
```

```go
service, err := NewUserService(session)
if err != nil {
	return err
}

users, err := service.
	ChainQuery().
	Eq(UserFields.Status, "ACTIVE").
	OrderByAsc(UserFields.ID).
	List(ctx)
if err != nil {
	return err
}
_ = users
```

## Partial Update

```go
rows, err := baseMapper.UpdateWithWrapper(
	ctx,
	orm.NewUpdateWrapper[User]().
		SetTyped(UserTypedFields.Name, "Alice").
		EqTyped(UserTypedFields.ID, int64(7)),
)
if err != nil {
	return err
}
_ = rows
```

## Field Strategies And Fill

```go
type User struct {
	ID        int64     `goark-orm:"column='id';primary-key=true;id-type='ASSIGN_ID'"`
	Name      string    `goark-orm:"column='name';insert-strategy='not-empty';update-strategy='not-empty'"`
	Version   int64     `goark-orm:"column='version';version=true"`
	Deleted   bool      `goark-orm:"column='deleted';soft-delete=true"`
	CreatedAt time.Time `goark-orm:"column='created_at';fill='insert'"`
	UpdatedAt time.Time `goark-orm:"column='updated_at';fill='insert_update'"`
}
```

```go
type auditFillHandler struct{}

func (auditFillHandler) InsertFill(ctx context.Context, meta *orm.MetaObject) error {
	if err := meta.StrictInsertFill("CreatedAt", time.Now()); err != nil {
		return err
	}
	return meta.StrictInsertFill("UpdatedAt", time.Now())
}

func (auditFillHandler) UpdateFill(ctx context.Context, meta *orm.MetaObject) error {
	return meta.StrictUpdateFill("UpdatedAt", time.Now())
}
```

## Provider And SQL Builder

```go
err := registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
	"UserSQL.ListByStatus",
	func(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs) (orm.SQLSource, error) {
		return orm.NewSelectSQLBuilder().
			Select("id", "name", "status").
			From("sys_user").
			WhereIn("status", args["statuses"]).
			WhereIsNull("deleted_at").
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

## Safe Raw SQL Tokens

```go
table, err := orm.NewRawIdentifier("tenant_01.sys_user")
if err != nil {
	return err
}
orderName, err := orm.NewRawOrderItem("name", false)
if err != nil {
	return err
}

compiled, err := orm.CompileSQL(
	"select id, name from ${table} order by ${orderBy} limit #{limit}",
	orm.NamedArgs{
		"table":   table,
		"orderBy": orm.NewRawOrderBy(orderName),
		"limit":   20,
	},
	orm.NewPostgresDialect(),
)
if err != nil {
	return err
}
_ = compiled
```

## Callable Statement

```go
//goark-orm:call(sql="call report_users(#{status}, #{total})", parameters="status:IN,total:OUT:BIGINT", resultSets="users:User")
ReportUsers(ctx context.Context, status string, total *int64, users *[]User) error
```

```xml
<call id="ReportUsers" statementType="CALLABLE">
  call report_users(#{status}, #{total})
  <parameter property="status" mode="IN"/>
  <parameter property="total" mode="OUT" jdbcType="BIGINT"/>
  <resultSet name="users" resultType="User"/>
</call>
```

## Transactions And Batch

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

## Streaming Query

```go
err := orm.QueryEach[User](ctx, session, "example.user.UserMapper.ListAll", nil, func(ctx context.Context, user User) error {
	return exportUser(ctx, user)
})
if err != nil {
	return err
}
```

Generated mappers can also declare streaming signatures:

```go
ListCursor(ctx context.Context, status string) (*orm.Cursor[User], error)
ListEach(ctx context.Context, status string, handler orm.ResultHandler[User]) error
```

## Routing Session

```go
routing, err := orm.NewRoutingSession(
	map[orm.DataSourceKey]orm.Session{
		"primary": primarySession,
		"replica": replicaSession,
	},
	orm.ReadWriteDataSourceResolver("replica", "primary"),
	orm.WithRoutingDefaultDataSource("primary"),
)
if err != nil {
	return err
}

ctx = orm.WithDataSource(ctx, "primary")
mapper := NewUserMapper(routing)
_ = mapper
```

## Audit Middleware

The audit package is optional and lives outside the root package. It records write operations and `affectData` selects by default.

```go
recorder := audit.RecorderFunc(func(ctx context.Context, event audit.Event) error {
	return writeAuditEvent(ctx, event)
})

session, err := orm.NewSQLSession(
	registry,
	db,
	orm.NewPostgresDialect(),
	orm.WithStatementExecutorMiddleware(audit.NewMiddleware(recorder, audit.WithQueryEvents(false))),
)
if err != nil {
	return err
}
_ = session
```

## Runtime JSON Configuration

```json
{
  "settings": {
    "cacheEnabled": true,
    "localCacheScope": "SESSION",
    "mapUnderscoreToCamelCase": true,
    "defaultExecutorType": "REUSE",
    "defaultStatementTimeout": "2s",
    "defaultResultSetType": "FORWARD_ONLY",
    "nullableOnForEach": true,
    "shrinkWhitespacesInSql": true,
    "jdbcTypeForNull": "OTHER",
    "autoMappingBehavior": "FULL",
    "autoMappingUnknownColumnBehavior": "NONE",
    "databaseId": "postgres"
  },
  "environment": {
    "id": "local",
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
  "typeAliases": [
    {
      "alias": "User",
      "typeName": "example.user.User"
    }
  ],
  "typeHandlers": [
    {
      "name": "json"
    }
  ],
  "mappers": [
    {
      "namespace": "example.user.UserMapper"
    }
  ]
}
```

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

## Schema Compatibility

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
_ = report.Model
_ = report.Source
```

## Real Database Test Harness

Keep driver imports and credentials in the caller's test harness. The standard reusable suite currently supports PostgreSQL, MySQL, MariaDB, SQLite, SQL Server, and Oracle:

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

Use `scripts/verify-real-db.ps1` for the local matrix and `scripts/verify-real-db-bench.ps1` for the matching benchmark harness. Concrete drivers are imported only in the temporary harness; SQLite additionally requires `GOARK_ORM_SQLITE_IMPORT` when `GOARK_ORM_SQLITE_DSN` is set.
