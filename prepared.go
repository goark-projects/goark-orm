package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SQLPreparer 是支持预编译语句的可选执行器能力。
type SQLPreparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

func (s *SQLSession) querySQL(ctx context.Context, compiled CompiledSQL) (*sql.Rows, error) {
	if !s.usePreparedStatements() {
		return s.executor.QueryContext(ctx, compiled.SQL, compiled.Args...)
	}
	statement, err := s.preparedStatement(ctx, compiled.SQL)
	if err != nil {
		return nil, err
	}
	return statement.QueryContext(ctx, compiled.Args...)
}

func (s *SQLSession) execSQL(ctx context.Context, compiled CompiledSQL) (sql.Result, error) {
	if !s.usePreparedStatements() {
		return s.executor.ExecContext(ctx, compiled.SQL, compiled.Args...)
	}
	statement, err := s.preparedStatement(ctx, compiled.SQL)
	if err != nil {
		return nil, err
	}
	return statement.ExecContext(ctx, compiled.Args...)
}

func (s *SQLSession) usePreparedStatements() bool {
	return s != nil && s.configuration.DefaultExecutorType == ExecutorTypeReuse
}

func (s *SQLSession) preparedStatement(ctx context.Context, query string) (*sql.Stmt, error) {
	if s == nil {
		return nil, fmt.Errorf("goark-orm: session is nil")
	}
	preparer, ok := s.executor.(SQLPreparer)
	if !ok {
		return nil, fmt.Errorf("goark-orm: executor type REUSE requires PrepareContext support")
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
		return fmt.Errorf("goark-orm: session is nil")
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
