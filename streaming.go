package orm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// ResultHandler 逐条消费查询结果。
type ResultHandler[T any] func(context.Context, T) error

// CursorQuerySession 描述支持游标查询的 Session。
type CursorQuerySession interface {
	QueryCursor(ctx context.Context, statement string, args NamedArgs) (*RowCursor, error)
}

// Cursor 是类型安全的查询游标。
type Cursor[T any] struct {
	rows *RowCursor
}

// QueryCursor 打开类型安全游标。游标查询不写入一级缓存和二级缓存。
func QueryCursor[T any](ctx context.Context, session Session, statement string, args NamedArgs) (*Cursor[T], error) {
	if session == nil {
		return nil, fmt.Errorf("goark-orm: session is nil")
	}
	cursorSession, ok := session.(CursorQuerySession)
	if !ok {
		return nil, fmt.Errorf("goark-orm: session does not support cursor query")
	}
	rows, err := cursorSession.QueryCursor(ctx, statement, args)
	if err != nil {
		return nil, err
	}
	return &Cursor[T]{rows: rows}, nil
}

// QueryEach 逐条扫描并回调处理查询结果。
func QueryEach[T any](ctx context.Context, session Session, statement string, args NamedArgs, handler ResultHandler[T]) error {
	if handler == nil {
		return fmt.Errorf("goark-orm: result handler is nil")
	}
	cursor, err := QueryCursor[T](ctx, session, statement, args)
	if err != nil {
		return err
	}
	for {
		item, ok, err := cursor.Next(ctx)
		if err != nil {
			return errors.Join(err, cursor.Close())
		}
		if !ok {
			return cursor.Close()
		}
		if err := handler(ctx, item); err != nil {
			return errors.Join(err, cursor.Close())
		}
	}
}

// RowCursor 暴露非泛型行游标，供框架扩展和自定义扫描使用。
type RowCursor struct {
	session      *SQLSession
	rows         Rows
	columns      []string
	statement    StatementMeta
	resultMap    ResultMapMeta
	hasResultMap bool
	closeOnce    sync.Once
	closeErr     error
	closed       bool
}

// Next 前进到下一行。
func (c *RowCursor) Next() bool {
	if c == nil || c.rows == nil || c.closed {
		return false
	}
	return c.rows.Next()
}

// Scan 扫描当前行到 dest。
func (c *RowCursor) Scan(ctx context.Context, dest any) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.rows == nil || c.session == nil {
		return fmt.Errorf("goark-orm: cursor is nil")
	}
	target, err := destination(dest)
	if err != nil {
		return err
	}
	if target.Kind() == reflect.Slice {
		return fmt.Errorf("goark-orm: cursor destination must not be slice")
	}
	if c.hasResultMap {
		return c.session.scanValueWithResultMap(ctx, c.rows, c.columns, c.statement, c.resultMap, target)
	}
	return c.session.scanValue(ctx, c.rows, c.columns, c.statement, target)
}

// Err 返回底层行集的迭代错误。
func (c *RowCursor) Err() error {
	if c == nil || c.rows == nil {
		return nil
	}
	return c.rows.Err()
}

// Close 关闭游标持有的行集。
func (c *RowCursor) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		c.closed = true
		if c.rows != nil {
			c.closeErr = c.rows.Close()
		}
	})
	return c.closeErr
}

// Next 扫描下一条记录；ok 为 false 表示游标已经耗尽。
func (c *Cursor[T]) Next(ctx context.Context) (T, bool, error) {
	var zero T
	if c == nil || c.rows == nil {
		return zero, false, fmt.Errorf("goark-orm: cursor is nil")
	}
	if !c.rows.Next() {
		if err := c.rows.Err(); err != nil {
			return zero, false, err
		}
		return zero, false, nil
	}
	var out T
	if err := c.rows.Scan(ctx, &out); err != nil {
		return zero, false, err
	}
	return out, true, nil
}

// Close 关闭游标。
func (c *Cursor[T]) Close() error {
	if c == nil || c.rows == nil {
		return nil
	}
	return c.rows.Close()
}

// Err 返回底层行集的迭代错误。
func (c *Cursor[T]) Err() error {
	if c == nil || c.rows == nil {
		return nil
	}
	return c.rows.Err()
}

// QueryCursor 执行查询并返回非泛型行游标。
func (s *SQLSession) QueryCursor(ctx context.Context, statement string, args NamedArgs) (*RowCursor, error) {
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm: context is nil")
	}
	meta, err := s.lookupStatement(statement)
	if err != nil {
		return nil, err
	}
	return s.QueryCursorStatement(ctx, meta, args)
}

// QueryCursorStatement 基于 StatementMeta 打开非泛型行游标。
func (s *SQLSession) QueryCursorStatement(ctx context.Context, meta StatementMeta, args NamedArgs) (*RowCursor, error) {
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm: context is nil")
	}
	if s == nil {
		return nil, fmt.Errorf("goark-orm: session is nil")
	}
	compiled, err := s.compileStatement(ctx, meta, args)
	if err != nil {
		return nil, err
	}
	if meta.Command != StatementCommandSelect {
		return nil, fmt.Errorf("goark-orm: statement %s is %s, not select", meta.FullName, meta.Command)
	}
	resultMap, hasResultMap := s.resultMap(meta)
	if hasResultMap && len(resultMap.Collections) > 0 {
		return nil, fmt.Errorf("goark-orm: cursor query does not support collection resultMap %s", resultMap.ID)
	}
	if err := s.flushStatementCaches(ctx, meta); err != nil {
		return nil, err
	}
	rows, err := s.querySQL(ctx, compiled)
	if err != nil {
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.Join(err, rows.Close())
	}
	return &RowCursor{
		session:      s,
		rows:         rows,
		columns:      columns,
		statement:    meta,
		resultMap:    resultMap,
		hasResultMap: hasResultMap,
	}, nil
}

func (s *TxSession) QueryCursor(ctx context.Context, statement string, args NamedArgs) (*RowCursor, error) {
	if err := s.ensureActive(); err != nil {
		return nil, err
	}
	return s.session.QueryCursor(ctx, statement, args)
}

func (s *BatchSession) QueryCursor(ctx context.Context, statement string, args NamedArgs) (*RowCursor, error) {
	if _, err := s.Flush(ctx); err != nil {
		return nil, err
	}
	cursorSession, ok := s.session.(CursorQuerySession)
	if !ok {
		return nil, fmt.Errorf("goark-orm: batch delegate does not support cursor query")
	}
	return cursorSession.QueryCursor(ctx, statement, args)
}
