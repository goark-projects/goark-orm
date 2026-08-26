package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

// ErrTransactionCompleted 表示事务已经提交或回滚。
var ErrTransactionCompleted = errors.New("goark-orm: transaction already completed")

// TransactionSource 是可以开启 database/sql 事务的最小边界。
type TransactionSource interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Transaction 描述 ORM 自身独立事务抽象。
type Transaction interface {
	Executor() SQLExecutor
	Commit() error
	Rollback() error
}

// TransactionFactory 创建事务实例。
type TransactionFactory interface {
	Begin(ctx context.Context, source TransactionSource, opts *sql.TxOptions) (Transaction, error)
}

// SQLTransactionFactory 基于 database/sql 创建事务。
type SQLTransactionFactory struct{}

// NewSQLTransactionFactory 创建默认 database/sql 事务工厂。
func NewSQLTransactionFactory() SQLTransactionFactory {
	return SQLTransactionFactory{}
}

// Begin 开启 database/sql 事务。
func (SQLTransactionFactory) Begin(ctx context.Context, source TransactionSource, opts *sql.TxOptions) (Transaction, error) {
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm: context is nil")
	}
	if source == nil {
		return nil, fmt.Errorf("goark-orm: transaction source is nil")
	}
	tx, err := source.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlTransaction{tx: tx}, nil
}

type sqlTransaction struct {
	tx *sql.Tx
}

func (t *sqlTransaction) Executor() SQLExecutor {
	if t == nil {
		return nil
	}
	return t.tx
}

func (t *sqlTransaction) Commit() error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("goark-orm: transaction is nil")
	}
	return t.tx.Commit()
}

func (t *sqlTransaction) Rollback() error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("goark-orm: transaction is nil")
	}
	return t.tx.Rollback()
}

// SQLSessionFactory 创建普通 Session 和事务 Session。
type SQLSessionFactory struct {
	registry           *Registry
	source             TransactionSource
	executor           SQLExecutor
	dialect            Dialect
	transactionFactory TransactionFactory
	sessionOptions     []SQLSessionOption
}

// NewSQLSessionFactory 创建基于 database/sql 的 SessionFactory。
func NewSQLSessionFactory(registry *Registry, db *sql.DB, dialect Dialect, options ...SQLSessionOption) (*SQLSessionFactory, error) {
	if registry == nil {
		return nil, fmt.Errorf("goark-orm: registry is nil")
	}
	if db == nil {
		return nil, fmt.Errorf("goark-orm: database is nil")
	}
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	return &SQLSessionFactory{
		registry:           registry,
		source:             db,
		executor:           db,
		dialect:            dialect,
		transactionFactory: NewSQLTransactionFactory(),
		sessionOptions:     append([]SQLSessionOption(nil), options...),
	}, nil
}

// SetTransactionFactory 替换事务工厂，供外部集成层适配自定义事务来源。
func (f *SQLSessionFactory) SetTransactionFactory(factory TransactionFactory) error {
	if f == nil {
		return fmt.Errorf("goark-orm: SQL session factory is nil")
	}
	if factory == nil {
		return fmt.Errorf("goark-orm: transaction factory is nil")
	}
	f.transactionFactory = factory
	return nil
}

// OpenSession 打开自动提交 Session。
func (f *SQLSessionFactory) OpenSession() (*SQLSession, error) {
	if f == nil {
		return nil, fmt.Errorf("goark-orm: SQL session factory is nil")
	}
	return NewSQLSession(f.registry, f.executor, f.dialect, f.sessionOptions...)
}

// OpenConfiguredSession 按运行期 Configuration 打开 Session。
//
// 当 DefaultExecutorType 为 BATCH 时返回 BatchSession；其他执行器类型返回普通 SQLSession。
// 该方法保留 OpenSession 的既有返回类型，避免破坏 V1 公共契约。
func (f *SQLSessionFactory) OpenConfiguredSession() (Session, error) {
	session, err := f.OpenSession()
	if err != nil {
		return nil, err
	}
	if session.configuration.DefaultExecutorType != ExecutorTypeBatch {
		return session, nil
	}
	batch, err := NewBatchSession(session)
	if err != nil {
		return nil, errors.Join(err, session.Close())
	}
	return batch, nil
}

// OpenBatchSession 打开自动提交批处理 Session。
func (f *SQLSessionFactory) OpenBatchSession() (*BatchSession, error) {
	session, err := f.OpenSession()
	if err != nil {
		return nil, err
	}
	return NewBatchSession(session)
}

// BeginTx 打开手动事务 Session。
func (f *SQLSessionFactory) BeginTx(ctx context.Context, opts *sql.TxOptions) (*TxSession, error) {
	if f == nil {
		return nil, fmt.Errorf("goark-orm: SQL session factory is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm: context is nil")
	}
	transactionFactory := f.transactionFactory
	if transactionFactory == nil {
		transactionFactory = NewSQLTransactionFactory()
	}
	tx, err := transactionFactory.Begin(ctx, f.source, opts)
	if err != nil {
		return nil, err
	}
	executor := tx.Executor()
	if executor == nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("goark-orm: transaction executor is nil")
	}
	options := append([]SQLSessionOption(nil), f.sessionOptions...)
	options = append(options, withTransactionalSecondLevelCache())
	session, err := NewSQLSession(f.registry, executor, f.dialect, options...)
	if err != nil {
		return nil, errors.Join(err, tx.Rollback())
	}
	return &TxSession{session: session, transaction: tx}, nil
}

// BeginBatchTx 打开手动事务批处理 Session。
func (f *SQLSessionFactory) BeginBatchTx(ctx context.Context, opts *sql.TxOptions) (*BatchSession, error) {
	session, err := f.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	batch, err := NewBatchSession(session)
	if err != nil {
		return nil, errors.Join(err, session.Close())
	}
	return batch, nil
}

// InTx 在事务中执行回调，回调返回错误时自动回滚。
func (f *SQLSessionFactory) InTx(ctx context.Context, opts *sql.TxOptions, fn func(context.Context, Session) error) error {
	if fn == nil {
		return fmt.Errorf("goark-orm: transaction callback is nil")
	}
	session, err := f.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = session.Rollback()
			panic(recovered)
		}
	}()
	if err := fn(ctx, session); err != nil {
		return errors.Join(err, session.Rollback())
	}
	return session.Commit()
}

// Commit 对自动提交 Session 不执行任何操作。
func (s *SQLSession) Commit() error {
	if s == nil {
		return fmt.Errorf("goark-orm: session is nil")
	}
	s.clearLocalCache()
	return nil
}

// Rollback 对自动提交 Session 不执行任何操作。
func (s *SQLSession) Rollback() error {
	if s == nil {
		return fmt.Errorf("goark-orm: session is nil")
	}
	s.clearLocalCache()
	return nil
}

// Close 关闭自动提交 Session。底层连接由 database/sql 管理。
func (s *SQLSession) Close() error {
	if s == nil {
		return fmt.Errorf("goark-orm: session is nil")
	}
	s.clearLocalCache()
	return s.closePreparedStatements()
}

// TxSession 是绑定单个事务的 Session。
type TxSession struct {
	session     *SQLSession
	transaction Transaction
	mu          sync.Mutex
	completed   bool
}

var _ Session = (*TxSession)(nil)
var _ ManagedSession = (*SQLSession)(nil)
var _ ManagedSession = (*TxSession)(nil)
var _ StatementSession = (*TxSession)(nil)

// Query 执行事务内查询并扫描多行结果。
func (s *TxSession) Query(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if err := s.ensureActive(); err != nil {
		return err
	}
	return s.session.Query(ctx, statement, args, dest)
}

// QueryOne 执行事务内查询并要求最多返回一行。
func (s *TxSession) QueryOne(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if err := s.ensureActive(); err != nil {
		return err
	}
	return s.session.QueryOne(ctx, statement, args, dest)
}

// Exec 执行事务内写入语句。
func (s *TxSession) Exec(ctx context.Context, statement string, args NamedArgs) (Result, error) {
	if err := s.ensureActive(); err != nil {
		return Result{}, err
	}
	return s.session.Exec(ctx, statement, args)
}

// QueryStatement 执行事务内查询语句元数据并扫描多行结果。
func (s *TxSession) QueryStatement(ctx context.Context, statement StatementMeta, args NamedArgs, dest any) error {
	if err := s.ensureActive(); err != nil {
		return err
	}
	return s.session.QueryStatement(ctx, statement, args, dest)
}

// QueryOneStatement 执行事务内查询语句元数据并要求最多返回一行。
func (s *TxSession) QueryOneStatement(ctx context.Context, statement StatementMeta, args NamedArgs, dest any) error {
	if err := s.ensureActive(); err != nil {
		return err
	}
	return s.session.QueryOneStatement(ctx, statement, args, dest)
}

// ExecStatement 执行事务内写入语句元数据。
func (s *TxSession) ExecStatement(ctx context.Context, statement StatementMeta, args NamedArgs) (Result, error) {
	if err := s.ensureActive(); err != nil {
		return Result{}, err
	}
	return s.session.ExecStatement(ctx, statement, args)
}

// Dialect 返回当前事务 Session 使用的数据库方言。
func (s *TxSession) Dialect() Dialect {
	if s == nil || s.session == nil {
		return NewQuestionDialect()
	}
	return s.session.Dialect()
}

// Commit 提交事务。
func (s *TxSession) Commit() error {
	if err := s.markCompleted(); err != nil {
		return err
	}
	s.session.clearLocalCache()
	commitErr := s.transaction.Commit()
	closeErr := s.session.closePreparedStatements()
	if commitErr != nil {
		s.session.discardSecondLevelCacheChanges()
		return errors.Join(commitErr, closeErr)
	}
	return errors.Join(closeErr, s.session.commitSecondLevelCache(context.Background()))
}

// Rollback 回滚事务。
func (s *TxSession) Rollback() error {
	if err := s.markCompleted(); err != nil {
		return err
	}
	s.session.clearLocalCache()
	s.session.discardSecondLevelCacheChanges()
	rollbackErr := s.transaction.Rollback()
	closeErr := s.session.closePreparedStatements()
	return errors.Join(rollbackErr, closeErr)
}

// Close 在事务未完成时回滚，避免连接被长时间占用。
func (s *TxSession) Close() error {
	s.mu.Lock()
	if s.completed {
		s.mu.Unlock()
		return nil
	}
	s.completed = true
	s.mu.Unlock()
	s.session.clearLocalCache()
	s.session.discardSecondLevelCacheChanges()
	rollbackErr := s.transaction.Rollback()
	closeErr := s.session.closePreparedStatements()
	return errors.Join(rollbackErr, closeErr)
}

func (s *TxSession) ensureActive() error {
	if s == nil || s.session == nil || s.transaction == nil {
		return fmt.Errorf("goark-orm: transaction session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.completed {
		return ErrTransactionCompleted
	}
	return nil
}

func (s *TxSession) markCompleted() error {
	if s == nil || s.session == nil || s.transaction == nil {
		return fmt.Errorf("goark-orm: transaction session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.completed {
		return ErrTransactionCompleted
	}
	s.completed = true
	return nil
}
