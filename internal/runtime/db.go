package runtime

import "context"

// Db 是显式绑定 Session 的快捷操作门面，不持有全局会话。
type Db struct {
	session Session
}

// NewDb 创建显式绑定 Session 的快捷操作门面。
func NewDb(session Session) (Db, error) {
	if session == nil {
		return Db{}, configurationErrorf("db session is nil")
	}
	return Db{session: session}, nil
}

// List 查询多行记录。
func (db Db) List(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if db.session == nil {
		return configurationErrorf("db session is nil")
	}
	return db.session.Query(ctx, statement, args, dest)
}

// GetOne 查询单行记录。
func (db Db) GetOne(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if db.session == nil {
		return configurationErrorf("db session is nil")
	}
	return db.session.QueryOne(ctx, statement, args, dest)
}

// Exec 执行写语句。
func (db Db) Exec(ctx context.Context, statement string, args NamedArgs) (Result, error) {
	if db.session == nil {
		return Result{}, configurationErrorf("db session is nil")
	}
	return db.session.Exec(ctx, statement, args)
}
