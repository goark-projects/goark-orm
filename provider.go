package orm

import "context"

// SQLSource 表示 Provider 在运行期返回的 SQL 来源。
type SQLSource struct {
	SQL        string
	DynamicSQL []DynamicSQLNode
}

// SQLProvider 按 Statement 和入参在运行期生成 SQL。
type SQLProvider func(ctx context.Context, statement StatementMeta, args NamedArgs) (SQLSource, error)
