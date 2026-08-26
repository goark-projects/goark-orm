package orm

import (
	"context"
	"fmt"
)

// SQLGuardRule 表示一条可组合的 SQL 治理规则。
type SQLGuardRule interface {
	CheckSQL(ctx context.Context, statement StatementMeta, sql string) error
}

// SQLGuardRuleFunc 将函数适配为 SQLGuardRule。
type SQLGuardRuleFunc func(ctx context.Context, statement StatementMeta, sql string) error

// CheckSQL 执行函数式 SQL 治理规则。
func (f SQLGuardRuleFunc) CheckSQL(ctx context.Context, statement StatementMeta, sql string) error {
	if f == nil {
		return nil
	}
	return f(ctx, statement, sql)
}

type sqlGuardInterceptor struct {
	ignoreName string
	rules      []SQLGuardRule
}

// NewSQLGuardInterceptor 创建通用 SQL 治理拦截器。
func NewSQLGuardInterceptor(rules ...SQLGuardRule) StatementInterceptor {
	return newSQLGuardInterceptor(InterceptorNameSQLGuard, rules...)
}

func newSQLGuardInterceptor(ignoreName string, rules ...SQLGuardRule) StatementInterceptor {
	copied := make([]SQLGuardRule, 0, len(rules))
	for _, rule := range rules {
		if rule != nil {
			copied = append(copied, rule)
		}
	}
	return &sqlGuardInterceptor{ignoreName: ignoreName, rules: copied}
}

func (i *sqlGuardInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	if i != nil && !StatementInterceptorIgnored(statement.Meta, i.ignoreName) {
		for _, rule := range i.rules {
			if err := rule.CheckSQL(ctx, statement.Meta, statement.SQL); err != nil {
				return err
			}
		}
	}
	return invocation.Proceed(ctx)
}

type illegalSQLConfig struct {
	DenySelectWildcard     bool
	DenyMultipleStatements bool
	DenyWriteWithoutWhere  bool
}

// IllegalSQLOption 配置非法 SQL 治理拦截器。
type IllegalSQLOption func(*illegalSQLConfig)

// WithIllegalSQLDenySelectWildcard 配置是否拒绝顶层 SELECT *。
func WithIllegalSQLDenySelectWildcard(enabled bool) IllegalSQLOption {
	return func(config *illegalSQLConfig) {
		config.DenySelectWildcard = enabled
	}
}

// WithIllegalSQLDenyMultipleStatements 配置是否拒绝多语句 SQL 模板。
func WithIllegalSQLDenyMultipleStatements(enabled bool) IllegalSQLOption {
	return func(config *illegalSQLConfig) {
		config.DenyMultipleStatements = enabled
	}
}

// WithIllegalSQLDenyWriteWithoutWhere 配置是否拒绝无 WHERE 的 UPDATE/DELETE。
func WithIllegalSQLDenyWriteWithoutWhere(enabled bool) IllegalSQLOption {
	return func(config *illegalSQLConfig) {
		config.DenyWriteWithoutWhere = enabled
	}
}

// NewIllegalSQLInterceptor 创建非法 SQL 治理拦截器。
func NewIllegalSQLInterceptor(options ...IllegalSQLOption) StatementInterceptor {
	config := illegalSQLConfig{
		DenySelectWildcard:     true,
		DenyMultipleStatements: true,
		DenyWriteWithoutWhere:  true,
	}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	rules := make([]SQLGuardRule, 0, 3)
	if config.DenyMultipleStatements {
		rules = append(rules, SQLGuardRuleFunc(rejectMultipleStatements))
	}
	if config.DenySelectWildcard {
		rules = append(rules, SQLGuardRuleFunc(rejectSelectWildcard))
	}
	if config.DenyWriteWithoutWhere {
		rules = append(rules, SQLGuardRuleFunc(rejectWriteWithoutWhere))
	}
	return newSQLGuardInterceptor(InterceptorNameIllegalSQL, rules...)
}

type readOnlyInterceptor struct{}

// NewReadOnlyInterceptor 创建只读治理拦截器。
func NewReadOnlyInterceptor() StatementInterceptor {
	return readOnlyInterceptor{}
}

func (readOnlyInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	if !StatementInterceptorIgnored(statement.Meta, InterceptorNameReadOnly) && statementIsWrite(statement.Meta.Command) {
		return fmt.Errorf("goark-orm: read-only session rejected %s statement %s", statement.Meta.Command, statement.Meta.FullName)
	}
	return invocation.Proceed(ctx)
}

func rejectMultipleStatements(ctx context.Context, statement StatementMeta, sql string) error {
	if hasSQLAfterStatementSeparator(sql) {
		return fmt.Errorf("goark-orm: illegal SQL rejected multiple statements for %s", statement.FullName)
	}
	return nil
}

func rejectSelectWildcard(ctx context.Context, statement StatementMeta, sql string) error {
	if statement.Command != StatementCommandSelect {
		return nil
	}
	if selectProjectionHasWildcard(sql) {
		return fmt.Errorf("goark-orm: illegal SQL rejected SELECT * for %s", statement.FullName)
	}
	return nil
}

func rejectWriteWithoutWhere(ctx context.Context, statement StatementMeta, sql string) error {
	switch statement.Command {
	case StatementCommandUpdate:
		if !containsSQLKeyword(sql, "where") {
			return fmt.Errorf("goark-orm: illegal SQL rejected update without WHERE for %s", statement.FullName)
		}
	case StatementCommandDelete:
		if !containsSQLKeyword(sql, "where") {
			return fmt.Errorf("goark-orm: illegal SQL rejected delete without WHERE for %s", statement.FullName)
		}
	}
	return nil
}

func statementIsWrite(command StatementCommand) bool {
	switch command {
	case StatementCommandInsert, StatementCommandUpdate, StatementCommandDelete:
		return true
	default:
		return false
	}
}

func hasSQLAfterStatementSeparator(sql string) bool {
	for index := 0; index < len(sql); {
		if next, ok := skipSQLComment(sql, index); ok {
			index = next
			continue
		}
		switch sql[index] {
		case '\'':
			index = skipSQLSingleQuoted(sql, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(sql, index); ok {
				index = next
				continue
			}
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(sql, index)
			if !ok {
				return false
			}
			index = next
			continue
		case '[':
			index = skipSQLBracketQuotedIdentifier(sql, index)
			continue
		case ';':
			return skipSQLSpacesAndComments(sql, index+1) < len(sql)
		}
		index++
	}
	return false
}

func selectProjectionHasWildcard(sql string) bool {
	selectIndex := findSQLKeyword(sql, "select")
	if selectIndex < 0 {
		return false
	}
	projectionStart := selectIndex + len("select")
	fromRelative := findSQLKeyword(sql[projectionStart:], "from")
	if fromRelative < 0 {
		return false
	}
	return projectionHasTopLevelWildcard(sql[projectionStart : projectionStart+fromRelative])
}

func projectionHasTopLevelWildcard(projection string) bool {
	depth := 0
	for index := 0; index < len(projection); {
		if next, ok := skipSQLComment(projection, index); ok {
			index = next
			continue
		}
		switch projection[index] {
		case '\'':
			index = skipSQLSingleQuoted(projection, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(projection, index); ok {
				index = next
				continue
			}
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(projection, index)
			if !ok {
				return false
			}
			index = next
			continue
		case '[':
			index = skipSQLBracketQuotedIdentifier(projection, index)
			continue
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '*':
			if depth == 0 {
				return true
			}
		}
		index++
	}
	return false
}
