package audit

import (
	"context"
	"errors"
	"time"

	orm "goark.dev/orm"
)

// Recorder 持久化或转发审计事件。
type Recorder interface {
	Record(ctx context.Context, event Event) error
}

// RecorderFunc 将函数适配为 Recorder。
type RecorderFunc func(context.Context, Event) error

// Record 执行函数式审计记录。
func (f RecorderFunc) Record(ctx context.Context, event Event) error {
	if f == nil {
		return nil
	}
	return f(ctx, event)
}

type middleware struct {
	recorder Recorder
	options  options
}

// NewMiddleware 创建 StatementExecutor 审计中间件。
func NewMiddleware(recorder Recorder, opts ...Option) orm.StatementExecutorMiddleware {
	return middleware{
		recorder: recorder,
		options:  applyOptions(opts),
	}
}

func (m middleware) WrapStatementExecutor(next orm.StatementExecutor) orm.StatementExecutor {
	if next == nil || m.recorder == nil {
		return next
	}
	return executor{
		next:     next,
		recorder: m.recorder,
		options:  m.options,
	}
}

type executor struct {
	next     orm.StatementExecutor
	recorder Recorder
	options  options
}

func (e executor) Query(ctx context.Context, session *orm.SQLSession, meta orm.StatementMeta, args orm.NamedArgs, dest any) error {
	startedAt := time.Now()
	err := e.next.Query(ctx, session, meta, args, dest)
	return e.record(ctx, OperationQuery, meta, orm.Result{}, startedAt, err)
}

func (e executor) QueryOne(ctx context.Context, session *orm.SQLSession, meta orm.StatementMeta, args orm.NamedArgs, dest any) error {
	startedAt := time.Now()
	err := e.next.QueryOne(ctx, session, meta, args, dest)
	return e.record(ctx, OperationQueryOne, meta, orm.Result{}, startedAt, err)
}

func (e executor) Exec(ctx context.Context, session *orm.SQLSession, meta orm.StatementMeta, args orm.NamedArgs) (orm.Result, error) {
	startedAt := time.Now()
	result, err := e.next.Exec(ctx, session, meta, args)
	return result, e.record(ctx, OperationExec, meta, result, startedAt, err)
}

func (e executor) record(ctx context.Context, operation Operation, meta orm.StatementMeta, result orm.Result, startedAt time.Time, operationErr error) error {
	event := newEvent(operation, meta, result, startedAt, operationErr)
	if !e.shouldRecord(event) {
		return operationErr
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := e.recorder.Record(ctx, event); err != nil && !e.options.ignoreRecorderError {
		return errors.Join(operationErr, err)
	}
	return operationErr
}

func (e executor) shouldRecord(event Event) bool {
	if event.Err != nil && !e.options.recordErrors {
		return false
	}
	if e.options.skip != nil && e.options.skip(event) {
		return false
	}
	if event.Operation == OperationExec || event.AffectData {
		return true
	}
	return e.options.recordQueries
}
