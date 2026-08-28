package runtime

import "strings"

// statementRuntimeArgs 在只读静态 SQL 路径复用调用方参数，避免每次执行分配 map。
func (s *SQLSession) statementRuntimeArgs(meta StatementMeta, args NamedArgs) NamedArgs {
	if s.statementRuntimeNeedsMutableArgs(meta) {
		return copyNamedArgs(args)
	}
	return args
}

// statementRuntimeNeedsMutableArgs 标记 Provider、动态 SQL 和扩展点等可能改写参数的路径。
func (s *SQLSession) statementRuntimeNeedsMutableArgs(meta StatementMeta) bool {
	if s == nil {
		return true
	}
	return strings.TrimSpace(meta.Provider) != "" ||
		len(meta.DynamicSQL) > 0 ||
		len(s.interceptors) > 0 ||
		s.hasStatementHandlerMiddleware ||
		s.metaObjectHandler != nil && meta.Source != StatementSourceBase
}
