package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// StatementExecutor 承担 Mapper Statement 的最终执行调度。
type StatementExecutor interface {
	Query(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error
	QueryOne(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error
	Exec(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs) (Result, error)
}

// StatementHandler 负责动态 SQL、拦截器和方言占位符编译。
type StatementHandler interface {
	Prepare(ctx context.Context, meta StatementMeta, args NamedArgs) (*StatementRuntime, error)
	Compile(ctx context.Context, runtime *StatementRuntime) (CompiledSQL, error)
	CompileText(ctx context.Context, meta StatementMeta, dialect Dialect, sqlText string, args NamedArgs) (CompiledSQL, error)
}

// ParameterHandler 负责根据参数类型和 TypeHandler 绑定数据库参数。
type ParameterHandler interface {
	Bind(ctx context.Context, statement StatementMeta, args NamedArgs) (NamedArgs, error)
}

// ResultSetHandler 负责把 database/sql 行集扫描到调用方目标对象。
type ResultSetHandler interface {
	ScanRows(ctx context.Context, rows Rows, statement StatementMeta, dest any) error
	ScanOne(ctx context.Context, rows Rows, statement StatementMeta, dest any) error
}

// Rows 是 *sql.Rows 的最小扫描接口，便于测试和自定义执行器复用扫描器。
type Rows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// WithStatementExecutor 替换 Session 级 Statement 执行器。
func WithStatementExecutor(executor StatementExecutor) SQLSessionOption {
	return func(session *SQLSession) error {
		if executor == nil {
			return configurationErrorf("statement executor is nil")
		}
		session.statementExecutor = executor
		return nil
	}
}

// WithStatementHandler 替换 Statement 编译处理器。
func WithStatementHandler(handler StatementHandler) SQLSessionOption {
	return func(session *SQLSession) error {
		if handler == nil {
			return configurationErrorf("statement handler is nil")
		}
		session.statementHandler = handler
		return nil
	}
}

// WithParameterHandler 替换参数绑定处理器。
func WithParameterHandler(handler ParameterHandler) SQLSessionOption {
	return func(session *SQLSession) error {
		if handler == nil {
			return configurationErrorf("parameter handler is nil")
		}
		session.parameterHandler = handler
		return nil
	}
}

// WithResultSetHandler 替换结果集扫描处理器。
func WithResultSetHandler(handler ResultSetHandler) SQLSessionOption {
	return func(session *SQLSession) error {
		if handler == nil {
			return configurationErrorf("result set handler is nil")
		}
		session.resultSetHandler = handler
		return nil
	}
}

// WithMetaObjectHandler 配置 Session 级自动填充处理器。
func WithMetaObjectHandler(handler MetaObjectHandler) SQLSessionOption {
	return func(session *SQLSession) error {
		if handler == nil {
			return configurationErrorf("meta object handler is nil")
		}
		session.metaObjectHandler = handler
		session.configuration.MetaObjectHandler = handler
		return nil
	}
}

type defaultStatementExecutor struct{}

var _ StatementExecutor = defaultStatementExecutor{}

func (defaultStatementExecutor) Query(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error {
	if session == nil {
		return configurationErrorf("session is nil")
	}
	compiled, err := session.compileStatement(ctx, meta, args)
	if err != nil {
		return err
	}
	if meta.Command != StatementCommandSelect {
		return fmt.Errorf("goark-orm: statement %s is %s, not select", meta.FullName, meta.Command)
	}
	if err := session.flushStatementCaches(ctx, meta); err != nil {
		return err
	}
	cacheKey, useCache := session.queryCacheKey(meta, compiled)
	if useCache {
		if hit, err := session.getLocalCache(cacheKey, dest); err != nil || hit {
			return err
		}
		if hit, err := session.getSecondLevelCache(ctx, meta, cacheKey, dest); err != nil || hit {
			return err
		}
		defer session.releaseSecondLevelCacheMiss(ctx, meta, cacheKey)
	}
	rows, err := session.querySQL(ctx, meta, compiled)
	if err != nil {
		return executorFailure(meta, "query", compiled, err)
	}
	scanErr := session.resultSetHandler.ScanRows(ctx, rows, meta, dest)
	closeErr := rows.Close()
	if scanErr != nil {
		return mappingFailure(meta, scanErr)
	}
	if closeErr != nil {
		return executorFailure(meta, "close rows", compiled, closeErr)
	}
	if useCache {
		if err := session.putLocalCache(cacheKey, dest); err != nil {
			return err
		}
		return session.putSecondLevelCache(ctx, meta, cacheKey, dest)
	}
	return nil
}

func (defaultStatementExecutor) QueryOne(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs, dest any) error {
	if session == nil {
		return configurationErrorf("session is nil")
	}
	compiled, err := session.compileStatement(ctx, meta, args)
	if err != nil {
		return err
	}
	if meta.Command != StatementCommandSelect {
		return fmt.Errorf("goark-orm: statement %s is %s, not select", meta.FullName, meta.Command)
	}
	if err := session.flushStatementCaches(ctx, meta); err != nil {
		return err
	}
	cacheKey, useCache := session.queryCacheKey(meta, compiled)
	if useCache {
		if hit, err := session.getLocalCache(cacheKey, dest); err != nil || hit {
			return err
		}
		if hit, err := session.getSecondLevelCache(ctx, meta, cacheKey, dest); err != nil || hit {
			return err
		}
		defer session.releaseSecondLevelCacheMiss(ctx, meta, cacheKey)
	}
	rows, err := session.querySQL(ctx, meta, compiled)
	if err != nil {
		return executorFailure(meta, "query", compiled, err)
	}
	scanErr := session.resultSetHandler.ScanOne(ctx, rows, meta, dest)
	closeErr := rows.Close()
	if scanErr != nil {
		if errors.Is(scanErr, ErrTooManyResults) || errors.Is(scanErr, sql.ErrNoRows) {
			return scanErr
		}
		return mappingFailure(meta, scanErr)
	}
	if closeErr != nil {
		return executorFailure(meta, "close rows", compiled, closeErr)
	}
	if useCache {
		if err := session.putLocalCache(cacheKey, dest); err != nil {
			return err
		}
		return session.putSecondLevelCache(ctx, meta, cacheKey, dest)
	}
	return nil
}

func (defaultStatementExecutor) Exec(ctx context.Context, session *SQLSession, meta StatementMeta, args NamedArgs) (Result, error) {
	if session == nil {
		return Result{}, configurationErrorf("session is nil")
	}
	if meta.Command == StatementCommandSelect {
		return Result{}, fmt.Errorf("goark-orm: statement %s is select; use Query or QueryOne", meta.FullName)
	}
	if meta.Command == StatementCommandCall {
		return Result{}, fmt.Errorf("goark-orm: statement %s is call; use Call", meta.FullName)
	}
	execArgs := copyNamedArgs(args)
	if execArgs == nil {
		execArgs = NamedArgs{}
	}
	var selectKeyValue any
	if meta.SelectKey.Enabled && normalizeSelectKeyOrder(meta.SelectKey.Order) == SelectKeyOrderBefore {
		value, err := session.executeSelectKey(ctx, meta, execArgs)
		if err != nil {
			return Result{}, err
		}
		selectKeyValue = value
		if err := applyKeyProperty(execArgs, selectKeyProperty(meta), value); err != nil {
			return Result{}, err
		}
	}
	compiled, err := session.compileStatement(ctx, meta, execArgs)
	if err != nil {
		return Result{}, err
	}
	session.clearLocalCache()
	sqlResult, err := session.execSQL(ctx, meta, compiled)
	if err != nil {
		return Result{}, executorFailure(meta, "exec", compiled, err)
	}
	if shouldFlushStatementCache(meta) {
		if err := session.flushSecondLevelCache(ctx, meta.Namespace); err != nil {
			return Result{}, err
		}
	}
	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return Result{}, executorFailure(meta, "rows affected", compiled, err)
	}
	result := Result{RowsAffected: rowsAffected}
	if meta.SelectKey.Enabled && normalizeSelectKeyOrder(meta.SelectKey.Order) == SelectKeyOrderAfter {
		value, err := session.executeSelectKey(ctx, meta, execArgs)
		if err != nil {
			return Result{}, err
		}
		selectKeyValue = value
		if err := applyKeyProperty(execArgs, selectKeyProperty(meta), value); err != nil {
			return Result{}, err
		}
	}
	if selectKeyValue != nil {
		if id, ok := keyAsInt64(selectKeyValue); ok {
			result.LastInsertID = id
		}
		return result, nil
	}
	lastInsertID, err := sqlResult.LastInsertId()
	if err != nil {
		if meta.UseGeneratedKeys {
			return Result{}, executorFailure(meta, "last insert id", compiled, err)
		}
		return result, nil
	}
	result.LastInsertID = lastInsertID
	if meta.UseGeneratedKeys {
		if err := applyKeyProperty(execArgs, meta.KeyProperty, lastInsertID); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

type defaultStatementHandler struct {
	session *SQLSession
}

var _ StatementHandler = (*defaultStatementHandler)(nil)

func (h *defaultStatementHandler) Prepare(ctx context.Context, meta StatementMeta, args NamedArgs) (*StatementRuntime, error) {
	return h.session.prepareStatementRuntime(ctx, meta, args)
}

func (h *defaultStatementHandler) Compile(ctx context.Context, runtime *StatementRuntime) (CompiledSQL, error) {
	return h.session.compileRuntime(ctx, runtime)
}

func (h *defaultStatementHandler) CompileText(ctx context.Context, meta StatementMeta, dialect Dialect, sqlText string, args NamedArgs) (CompiledSQL, error) {
	return h.session.compileSQLText(ctx, meta, dialect, sqlText, args)
}

type defaultParameterHandler struct {
	session *SQLSession
}

var _ ParameterHandler = (*defaultParameterHandler)(nil)

func (h *defaultParameterHandler) Bind(ctx context.Context, statement StatementMeta, args NamedArgs) (NamedArgs, error) {
	return h.session.bindArgs(ctx, statement, args)
}

type defaultResultSetHandler struct {
	session *SQLSession
}

var _ ResultSetHandler = (*defaultResultSetHandler)(nil)

func (h *defaultResultSetHandler) ScanRows(ctx context.Context, rows Rows, statement StatementMeta, dest any) error {
	return h.session.scanRows(ctx, rows, statement, dest)
}

func (h *defaultResultSetHandler) ScanOne(ctx context.Context, rows Rows, statement StatementMeta, dest any) error {
	return h.session.scanOne(ctx, rows, statement, dest)
}
