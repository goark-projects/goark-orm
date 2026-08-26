package orm

// StatementExecutorMiddleware 包装最终执行器，用于扩展 Query、QueryOne 和 Exec 层。
type StatementExecutorMiddleware interface {
	WrapStatementExecutor(next StatementExecutor) StatementExecutor
}

// StatementExecutorMiddlewareFunc 将函数适配为 StatementExecutorMiddleware。
type StatementExecutorMiddlewareFunc func(next StatementExecutor) StatementExecutor

// WrapStatementExecutor 执行函数式执行器包装。
func (f StatementExecutorMiddlewareFunc) WrapStatementExecutor(next StatementExecutor) StatementExecutor {
	if f == nil {
		return next
	}
	return f(next)
}

// StatementHandlerMiddleware 包装语句处理器，用于扩展动态 SQL、拦截器和占位符编译层。
type StatementHandlerMiddleware interface {
	WrapStatementHandler(next StatementHandler) StatementHandler
}

// StatementHandlerMiddlewareFunc 将函数适配为 StatementHandlerMiddleware。
type StatementHandlerMiddlewareFunc func(next StatementHandler) StatementHandler

// WrapStatementHandler 执行函数式语句处理器包装。
func (f StatementHandlerMiddlewareFunc) WrapStatementHandler(next StatementHandler) StatementHandler {
	if f == nil {
		return next
	}
	return f(next)
}

// ParameterHandlerMiddleware 包装参数处理器，用于扩展实体参数绑定和 TypeHandler 入库转换层。
type ParameterHandlerMiddleware interface {
	WrapParameterHandler(next ParameterHandler) ParameterHandler
}

// ParameterHandlerMiddlewareFunc 将函数适配为 ParameterHandlerMiddleware。
type ParameterHandlerMiddlewareFunc func(next ParameterHandler) ParameterHandler

// WrapParameterHandler 执行函数式参数处理器包装。
func (f ParameterHandlerMiddlewareFunc) WrapParameterHandler(next ParameterHandler) ParameterHandler {
	if f == nil {
		return next
	}
	return f(next)
}

// ResultSetHandlerMiddleware 包装结果集处理器，用于扩展行扫描和结果映射层。
type ResultSetHandlerMiddleware interface {
	WrapResultSetHandler(next ResultSetHandler) ResultSetHandler
}

// ResultSetHandlerMiddlewareFunc 将函数适配为 ResultSetHandlerMiddleware。
type ResultSetHandlerMiddlewareFunc func(next ResultSetHandler) ResultSetHandler

// WrapResultSetHandler 执行函数式结果集处理器包装。
func (f ResultSetHandlerMiddlewareFunc) WrapResultSetHandler(next ResultSetHandler) ResultSetHandler {
	if f == nil {
		return next
	}
	return f(next)
}

// WithStatementExecutorMiddleware 按声明顺序包装 StatementExecutor。
func WithStatementExecutorMiddleware(middleware ...StatementExecutorMiddleware) SQLSessionOption {
	return func(session *SQLSession) error {
		executor, err := wrapStatementExecutor(session.statementExecutor, middleware)
		if err != nil {
			return err
		}
		session.statementExecutor = executor
		return nil
	}
}

// WithStatementHandlerMiddleware 按声明顺序包装 StatementHandler。
func WithStatementHandlerMiddleware(middleware ...StatementHandlerMiddleware) SQLSessionOption {
	return func(session *SQLSession) error {
		handler, err := wrapStatementHandler(session.statementHandler, middleware)
		if err != nil {
			return err
		}
		session.statementHandler = handler
		return nil
	}
}

// WithParameterHandlerMiddleware 按声明顺序包装 ParameterHandler。
func WithParameterHandlerMiddleware(middleware ...ParameterHandlerMiddleware) SQLSessionOption {
	return func(session *SQLSession) error {
		handler, err := wrapParameterHandler(session.parameterHandler, middleware)
		if err != nil {
			return err
		}
		session.parameterHandler = handler
		return nil
	}
}

// WithResultSetHandlerMiddleware 按声明顺序包装 ResultSetHandler。
func WithResultSetHandlerMiddleware(middleware ...ResultSetHandlerMiddleware) SQLSessionOption {
	return func(session *SQLSession) error {
		handler, err := wrapResultSetHandler(session.resultSetHandler, middleware)
		if err != nil {
			return err
		}
		session.resultSetHandler = handler
		return nil
	}
}

func wrapStatementExecutor(next StatementExecutor, middleware []StatementExecutorMiddleware) (StatementExecutor, error) {
	if next == nil {
		return nil, configurationErrorf("statement executor is nil")
	}
	for index := len(middleware) - 1; index >= 0; index-- {
		item := middleware[index]
		if item == nil {
			continue
		}
		next = item.WrapStatementExecutor(next)
		if next == nil {
			return nil, configurationErrorf("statement executor middleware returned nil")
		}
	}
	return next, nil
}

func wrapStatementHandler(next StatementHandler, middleware []StatementHandlerMiddleware) (StatementHandler, error) {
	if next == nil {
		return nil, configurationErrorf("statement handler is nil")
	}
	for index := len(middleware) - 1; index >= 0; index-- {
		item := middleware[index]
		if item == nil {
			continue
		}
		next = item.WrapStatementHandler(next)
		if next == nil {
			return nil, configurationErrorf("statement handler middleware returned nil")
		}
	}
	return next, nil
}

func wrapParameterHandler(next ParameterHandler, middleware []ParameterHandlerMiddleware) (ParameterHandler, error) {
	if next == nil {
		return nil, configurationErrorf("parameter handler is nil")
	}
	for index := len(middleware) - 1; index >= 0; index-- {
		item := middleware[index]
		if item == nil {
			continue
		}
		next = item.WrapParameterHandler(next)
		if next == nil {
			return nil, configurationErrorf("parameter handler middleware returned nil")
		}
	}
	return next, nil
}

func wrapResultSetHandler(next ResultSetHandler, middleware []ResultSetHandlerMiddleware) (ResultSetHandler, error) {
	if next == nil {
		return nil, configurationErrorf("result set handler is nil")
	}
	for index := len(middleware) - 1; index >= 0; index-- {
		item := middleware[index]
		if item == nil {
			continue
		}
		next = item.WrapResultSetHandler(next)
		if next == nil {
			return nil, configurationErrorf("result set handler middleware returned nil")
		}
	}
	return next, nil
}
