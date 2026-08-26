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

func (s *SQLSession) querySQL(ctx context.Context, compiled CompiledSQL) (Rows, error) {
	if err := s.applyFetchSize(ctx, compiled.SQL); err != nil {
		return nil, err
	}
	queryCtx, cancel := s.statementContext(ctx)
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

func (s *SQLSession) execSQL(ctx context.Context, compiled CompiledSQL) (sql.Result, error) {
	execCtx, cancel := s.statementContext(ctx)
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

func (s *SQLSession) statementContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if s == nil || s.configuration.DefaultStatementTimeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.configuration.DefaultStatementTimeout)
}

func (s *SQLSession) applyFetchSize(ctx context.Context, query string) error {
	if s == nil || s.configuration.DefaultFetchSize <= 0 {
		return nil
	}
	applier, ok := s.executor.(SQLFetchSizeApplier)
	if !ok {
		return nil
	}
	return applier.ApplyFetchSize(ctx, query, s.configuration.DefaultFetchSize)
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
