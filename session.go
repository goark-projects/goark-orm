package orm

import "context"

// NamedArgs 保存已经按名称绑定的 SQL 参数。
type NamedArgs map[string]any

// Result 表示写操作结果。
type Result struct {
	RowsAffected int64
	LastInsertID int64
}

// Session 是生成 Mapper 代码依赖的最小执行边界。
type Session interface {
	Query(ctx context.Context, statement string, args NamedArgs, dest any) error
	QueryOne(ctx context.Context, statement string, args NamedArgs, dest any) error
	Exec(ctx context.Context, statement string, args NamedArgs) (Result, error)
}

// ManagedSession 描述具备提交、回滚和关闭生命周期的 Session。
type ManagedSession interface {
	Session
	Commit() error
	Rollback() error
	Close() error
}
