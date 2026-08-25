package orm

import (
	"context"
	"fmt"
	"sync"
)

// BatchResult 表示批处理中单条语句的执行结果。
type BatchResult struct {
	Statement     string
	StatementMeta StatementMeta
	Args          NamedArgs
	Result        Result
}

// BatchError 描述批处理在指定下标语句上的执行失败。
type BatchError struct {
	Index         int
	Statement     string
	StatementMeta StatementMeta
	Err           error
}

func (e *BatchError) Error() string {
	if e == nil {
		return "goark-orm: batch error"
	}
	name := e.Statement
	if name == "" {
		name = e.StatementMeta.FullName
	}
	if name == "" {
		name = "<anonymous>"
	}
	return fmt.Sprintf("goark-orm: batch statement %d %s failed: %v", e.Index, name, e.Err)
}

// Unwrap 返回底层执行错误。
func (e *BatchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type batchItem struct {
	statement string
	meta      StatementMeta
	args      NamedArgs
}

// BatchSession 提供 MyBatis BatchExecutor 风格的批处理 Session。
type BatchSession struct {
	session          Session
	statementSession StatementSession
	managed          ManagedSession
	dialect          Dialect
	mu               sync.Mutex
	items            []batchItem
}

var _ Session = (*BatchSession)(nil)
var _ ManagedSession = (*BatchSession)(nil)
var _ StatementSession = (*BatchSession)(nil)

// NewBatchSession 创建批处理 Session。
func NewBatchSession(session Session) (*BatchSession, error) {
	if session == nil {
		return nil, fmt.Errorf("goark-orm: batch session delegate is nil")
	}
	batch := &BatchSession{
		session: session,
		items:   make([]batchItem, 0),
	}
	if statementSession, ok := session.(StatementSession); ok {
		batch.statementSession = statementSession
	}
	if managed, ok := session.(ManagedSession); ok {
		batch.managed = managed
	}
	if provider, ok := session.(dialectProvider); ok {
		batch.dialect = provider.Dialect()
	}
	return batch, nil
}

// Query 会先刷新待执行写语句，再执行查询。
func (s *BatchSession) Query(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if _, err := s.Flush(ctx); err != nil {
		return err
	}
	return s.session.Query(ctx, statement, args, dest)
}

// QueryOne 会先刷新待执行写语句，再执行单行查询。
func (s *BatchSession) QueryOne(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if _, err := s.Flush(ctx); err != nil {
		return err
	}
	return s.session.QueryOne(ctx, statement, args, dest)
}

// Exec 将命名 Statement 加入批处理队列。
func (s *BatchSession) Exec(ctx context.Context, statement string, args NamedArgs) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("goark-orm: context is nil")
	}
	if s == nil || s.session == nil {
		return Result{}, fmt.Errorf("goark-orm: batch session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, batchItem{
		statement: statement,
		args:      copyNamedArgs(args),
	})
	return Result{}, nil
}

// QueryStatement 会先刷新待执行写语句，再执行查询语句元数据。
func (s *BatchSession) QueryStatement(ctx context.Context, statement StatementMeta, args NamedArgs, dest any) error {
	if _, err := s.Flush(ctx); err != nil {
		return err
	}
	if s.statementSession == nil {
		return fmt.Errorf("goark-orm: batch delegate does not support statement execution")
	}
	return s.statementSession.QueryStatement(ctx, statement, args, dest)
}

// QueryOneStatement 会先刷新待执行写语句，再执行单行查询语句元数据。
func (s *BatchSession) QueryOneStatement(ctx context.Context, statement StatementMeta, args NamedArgs, dest any) error {
	if _, err := s.Flush(ctx); err != nil {
		return err
	}
	if s.statementSession == nil {
		return fmt.Errorf("goark-orm: batch delegate does not support statement execution")
	}
	return s.statementSession.QueryOneStatement(ctx, statement, args, dest)
}

// ExecStatement 将 StatementMeta 加入批处理队列。
func (s *BatchSession) ExecStatement(ctx context.Context, statement StatementMeta, args NamedArgs) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("goark-orm: context is nil")
	}
	if s == nil {
		return Result{}, fmt.Errorf("goark-orm: batch session is nil")
	}
	if statement.Command == StatementCommandSelect {
		return Result{}, fmt.Errorf("goark-orm: statement %s is select; use Query or QueryOne", statement.FullName)
	}
	if s.statementSession == nil && statement.FullName == "" {
		return Result{}, fmt.Errorf("goark-orm: batch delegate does not support anonymous statement execution")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, batchItem{
		statement: statement.FullName,
		meta:      statement,
		args:      copyNamedArgs(args),
	})
	return Result{}, nil
}

// Flush 按入队顺序执行所有待处理写语句。
func (s *BatchSession) Flush(ctx context.Context) ([]BatchResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm: context is nil")
	}
	if s == nil || s.session == nil {
		return nil, fmt.Errorf("goark-orm: batch session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		return nil, nil
	}
	results := make([]BatchResult, 0, len(s.items))
	for index, item := range s.items {
		result, err := s.execItem(ctx, item)
		if err != nil {
			s.items = append([]batchItem(nil), s.items[index:]...)
			return results, &BatchError{
				Index:         index,
				Statement:     item.statement,
				StatementMeta: item.meta,
				Err:           err,
			}
		}
		results = append(results, BatchResult{
			Statement:     item.statement,
			StatementMeta: item.meta,
			Args:          copyNamedArgs(item.args),
			Result:        result,
		})
	}
	s.items = s.items[:0]
	return results, nil
}

func (s *BatchSession) execItem(ctx context.Context, item batchItem) (Result, error) {
	if item.meta.FullName != "" || item.meta.SQL != "" || len(item.meta.DynamicSQL) > 0 {
		if s.statementSession != nil {
			return s.statementSession.ExecStatement(ctx, item.meta, item.args)
		}
		if item.meta.FullName == "" {
			return Result{}, fmt.Errorf("goark-orm: batch delegate does not support anonymous statement execution")
		}
		return s.session.Exec(ctx, item.meta.FullName, item.args)
	}
	return s.session.Exec(ctx, item.statement, item.args)
}

// Clear 丢弃所有尚未 flush 的批处理语句。
func (s *BatchSession) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = s.items[:0]
}

// Len 返回当前待执行语句数量。
func (s *BatchSession) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// Commit 使用后台上下文刷新批处理并提交底层 Session。
func (s *BatchSession) Commit() error {
	return s.CommitContext(context.Background())
}

// CommitContext 刷新批处理并提交底层 Session。
func (s *BatchSession) CommitContext(ctx context.Context) error {
	if _, err := s.Flush(ctx); err != nil {
		return err
	}
	if s == nil || s.managed == nil {
		return nil
	}
	return s.managed.Commit()
}

// Rollback 丢弃待执行语句并回滚底层 Session。
func (s *BatchSession) Rollback() error {
	if s == nil {
		return fmt.Errorf("goark-orm: batch session is nil")
	}
	s.Clear()
	if s.managed == nil {
		return nil
	}
	return s.managed.Rollback()
}

// Close 丢弃待执行语句并关闭底层 Session。
func (s *BatchSession) Close() error {
	if s == nil {
		return fmt.Errorf("goark-orm: batch session is nil")
	}
	s.Clear()
	if s.managed == nil {
		return nil
	}
	return s.managed.Close()
}

// Dialect 返回底层 Session 的数据库方言。
func (s *BatchSession) Dialect() Dialect {
	if s == nil || s.dialect == nil {
		return NewQuestionDialect()
	}
	return s.dialect
}
