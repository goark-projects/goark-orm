package orm

import "context"

// TypeHandler 负责 Go 值与数据库值之间的双向转换。
type TypeHandler interface {
	ToDB(ctx context.Context, value any) (any, error)
	FromDB(ctx context.Context, value any, target any) error
}
