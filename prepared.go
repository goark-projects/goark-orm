package orm

import (
	"context"
	"database/sql"
	"errors"
	"sync"
)

// SQLPreparer 是支持预编译语句的可选执行器能力。
type SQLPreparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// SQLFetchSizeApplier 是支持查询级 fetch size 的可选执行器能力。
type SQLFetchSizeApplier interface {
	ApplyFetchSize(ctx context.Context, query string, fetchSize int) error
}

// SQLStatementOptionsApplier 是支持完整语句级执行选项的可选执行器能力。
type SQLStatementOptionsApplier interface {
	ApplyStatementOptions(ctx context.Context, query string, options StatementOptions) error
}

func (s *SQLSession) querySQL(ctx context.Context, meta StatementMeta, compiled CompiledSQL) (Rows, error) {
	options := s.statementOptions(meta)
	if err := s.applyStatementOptions(ctx, compiled.SQL, options, true); err != nil {
		return nil, err
	}
	queryCtx, cancel := s.statementContext(ctx, meta)
	if !s.usePreparedStatements() {
		rows, err := s.executor.QueryContext(queryCtx, compiled.SQL, compiled.Args...)
		if err != nil {
			cancel()
			return nil, err
		}
		return &cancelRows{Rows: rows, cancel: cancel}, nil
	}
	statement, err := s.preparedStatement(queryCtx, compiled.SQL)
	if err != nil {
		cancel()
		return nil, err
	}
	rows, err := statement.QueryContext(queryCtx, compiled.Args...)
	if err != nil {
		cancel()
		return nil, err
	}
	return &cancelRows{Rows: rows, cancel: cancel}, nil
}

func (s *SQLSession) execSQL(ctx context.Context, meta StatementMeta, compiled CompiledSQL) (sql.Result, error) {
	options := s.statementOptions(meta)
	if err := s.applyStatementOptions(ctx, compiled.SQL, options, false); err != nil {
		return nil, err
	}
	execCtx, cancel := s.statementContext(ctx, meta)
	defer cancel()
	if !s.usePreparedStatements() {
		return s.executor.ExecContext(execCtx, compiled.SQL, compiled.Args...)
	}
	statement, err := s.preparedStatement(execCtx, compiled.SQL)
	if err != nil {
		return nil, err
	}
	return statement.ExecContext(execCtx, compiled.Args...)
}

func (s *SQLSession) usePreparedStatements() bool {
	return s != nil && s.configuration.DefaultExecutorType == ExecutorTypeReuse
}

func (s *SQLSession) statementContext(ctx context.Context, meta StatementMeta) (context.Context, context.CancelFunc) {
	timeout := s.statementOptions(meta).Timeout
	if s == nil || timeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *SQLSession) statementOptions(meta StatementMeta) StatementOptions {
	if s == nil {
		return meta.Options
	}
	return meta.Options.withDefaults(s.configuration.DefaultStatementTimeout, s.configuration.DefaultFetchSize)
}

func (s *SQLSession) applyStatementOptions(ctx context.Context, query string, options StatementOptions, queryOnly bool) error {
	if s == nil || options.isZero() {
		return nil
	}
	if applier, ok := s.executor.(SQLStatementOptionsApplier); ok {
		return applier.ApplyStatementOptions(ctx, query, options)
	}
	if !queryOnly || options.FetchSize <= 0 {
		return nil
	}
	applier, ok := s.executor.(SQLFetchSizeApplier)
	if !ok {
		return nil
	}
	return applier.ApplyFetchSize(ctx, query, options.FetchSize)
}

type cancelRows struct {
	Rows
	cancelOnce sync.Once
	cancel     context.CancelFunc
}

func (r *cancelRows) Close() error {
	err := r.Rows.Close()
	r.cancelOnce.Do(r.cancel)
	return err
}

func (s *SQLSession) preparedStatement(ctx context.Context, query string) (*sql.Stmt, error) {
	if s == nil {
		return nil, configurationErrorf("session is nil")
	}
	preparer, ok := s.executor.(SQLPreparer)
	if !ok {
		return nil, configurationErrorf("executor type REUSE requires PrepareContext support")
	}
	s.preparedMu.Lock()
	if statement := s.preparedStatements[query]; statement != nil {
		s.preparedMu.Unlock()
		return statement, nil
	}
	s.preparedMu.Unlock()

	statement, err := preparer.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	s.preparedMu.Lock()
	defer s.preparedMu.Unlock()
	if s.preparedStatements == nil {
		s.preparedStatements = make(map[string]*sql.Stmt)
	}
	if existing := s.preparedStatements[query]; existing != nil {
		_ = statement.Close()
		return existing, nil
	}
	s.preparedStatements[query] = statement
	return statement, nil
}

func (s *SQLSession) closePreparedStatements() error {
	if s == nil {
		return configurationErrorf("session is nil")
	}
	s.preparedMu.Lock()
	statements := s.preparedStatements
	s.preparedStatements = nil
	s.preparedMu.Unlock()
	var err error
	for _, statement := range statements {
		err = errors.Join(err, statement.Close())
	}
	return err
}
