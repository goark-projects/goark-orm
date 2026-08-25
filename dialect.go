package orm

import (
	"strconv"
	"strings"
)

// Dialect 封装数据库方言差异。
type Dialect interface {
	Name() string
	Placeholder(index int) string
	QuoteIdent(identifier string) string
}

type questionDialect struct{}

// NewQuestionDialect 创建使用问号占位符的通用方言。
func NewQuestionDialect() Dialect {
	return questionDialect{}
}

func (questionDialect) Name() string {
	return "question"
}

func (questionDialect) Placeholder(index int) string {
	return "?"
}

func (questionDialect) QuoteIdent(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func (questionDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	return query + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
}

type postgresDialect struct{}

// NewPostgresDialect 创建 PostgreSQL 方言。
func NewPostgresDialect() Dialect {
	return postgresDialect{}
}

func (postgresDialect) Name() string {
	return "postgres"
}

func (postgresDialect) Placeholder(index int) string {
	return "$" + strconv.Itoa(index)
}

func (postgresDialect) QuoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func (postgresDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	return query + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
}
