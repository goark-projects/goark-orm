package orm

import "context"

// canCompileStatementDirectly 判断是否可以跳过 StatementRuntime 分配。
func (s *SQLSession) canCompileStatementDirectly(meta StatementMeta) bool {
	if s == nil || s.statementHandler == nil {
		return false
	}
	if _, ok := s.statementHandler.(*defaultStatementHandler); !ok {
		return false
	}
	return !s.statementRuntimeNeedsMutableArgs(meta)
}

// compileStaticStatement 编译不需要运行期改写的静态 SQL。
func (s *SQLSession) compileStaticStatement(ctx context.Context, meta StatementMeta, args NamedArgs) (CompiledSQL, error) {
	sqlText := meta.SQL
	if s.configuration.ShrinkWhitespacesInSQL {
		sqlText = ShrinkSQLWhitespaces(sqlText)
	}
	boundArgs, err := s.parameterHandler.Bind(ctx, meta, args)
	if err != nil {
		return CompiledSQL{}, bindingFailure(meta.FullName, "bind", err)
	}
	compiled, err := CompileSQLContext(ctx, sqlText, boundArgs, s.Dialect())
	if err != nil {
		return CompiledSQL{}, bindingFailure(meta.FullName, "compile", err)
	}
	return compiled, nil
}
