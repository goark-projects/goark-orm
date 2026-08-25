package orm

import (
	"context"
	"fmt"
	"strings"
)

// StatementRuntime 描述一次执行中的可变 SQL 模板。
type StatementRuntime struct {
	Meta    StatementMeta
	SQL     string
	Args    NamedArgs
	Dialect Dialect
}

func (r *StatementRuntime) ensureArgs() {
	if r.Args == nil {
		r.Args = NamedArgs{}
	}
}

// StatementInterceptor 拦截并改写一次 Statement 执行。
type StatementInterceptor interface {
	Intercept(ctx context.Context, invocation *StatementInvocation) error
}

// StatementInterceptorFunc 将函数适配为 StatementInterceptor。
type StatementInterceptorFunc func(ctx context.Context, invocation *StatementInvocation) error

// Intercept 执行函数式拦截器。
func (f StatementInterceptorFunc) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	if f == nil {
		return invocation.Proceed(ctx)
	}
	return f(ctx, invocation)
}

// StatementInvocation 维护拦截器链的执行游标。
type StatementInvocation struct {
	statement    *StatementRuntime
	interceptors []StatementInterceptor
	index        int
}

// Statement 返回当前可变 SQL 模板。
func (i *StatementInvocation) Statement() *StatementRuntime {
	if i == nil {
		return nil
	}
	return i.statement
}

// Proceed 执行链上的下一个拦截器。
func (i *StatementInvocation) Proceed(ctx context.Context) error {
	if i == nil {
		return fmt.Errorf("goark-orm: statement invocation is nil")
	}
	for i.index < len(i.interceptors) {
		next := i.interceptors[i.index]
		i.index++
		if next == nil {
			continue
		}
		return next.Intercept(ctx, i)
	}
	return nil
}

// WithInterceptors 为 SQLSession 注册 Statement 拦截器。
func WithInterceptors(interceptors ...StatementInterceptor) SQLSessionOption {
	return func(session *SQLSession) error {
		for _, interceptor := range interceptors {
			if interceptor == nil {
				return fmt.Errorf("goark-orm: statement interceptor is nil")
			}
			session.interceptors = append(session.interceptors, interceptor)
		}
		return nil
	}
}

// SQLObservation 描述一次 SQL 模板观察事件。
type SQLObservation struct {
	Statement StatementMeta
	SQL       string
	Args      NamedArgs
	Dialect   Dialect
}

type sqlObserverInterceptor struct {
	observe func(context.Context, SQLObservation) error
}

// NewSQLObserverInterceptor 创建 SQL 模板观察拦截器。
func NewSQLObserverInterceptor(observe func(context.Context, SQLObservation) error) StatementInterceptor {
	return &sqlObserverInterceptor{observe: observe}
}

func (i *sqlObserverInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	if err := invocation.Proceed(ctx); err != nil {
		return err
	}
	if i == nil || i.observe == nil {
		return nil
	}
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	return i.observe(ctx, SQLObservation{
		Statement: statement.Meta,
		SQL:       statement.SQL,
		Args:      copyNamedArgs(statement.Args),
		Dialect:   statement.Dialect,
	})
}

type blockAttackInterceptor struct{}

// NewBlockAttackInterceptor 创建全表更新/删除拦截器。
func NewBlockAttackInterceptor() StatementInterceptor {
	return blockAttackInterceptor{}
}

func (blockAttackInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	if err := invocation.Proceed(ctx); err != nil {
		return err
	}
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	switch statement.Meta.Command {
	case StatementCommandUpdate:
		if !containsSQLKeyword(statement.SQL, "where") {
			return fmt.Errorf("goark-orm: blocked full-table update for statement %s", statement.Meta.FullName)
		}
	case StatementCommandDelete:
		if !containsSQLKeyword(statement.SQL, "where") {
			return fmt.Errorf("goark-orm: blocked full-table delete for statement %s", statement.Meta.FullName)
		}
	}
	return nil
}

// SQLCondition 描述可注入到 WHERE 子句的 SQL 条件。
type SQLCondition struct {
	SQL  string
	Args NamedArgs
}

// DataPermissionProvider 根据当前语句生成数据权限条件。
type DataPermissionProvider func(ctx context.Context, statement StatementMeta) (SQLCondition, error)

type dataPermissionInterceptor struct {
	provider DataPermissionProvider
}

// NewDataPermissionInterceptor 创建数据权限条件注入拦截器。
func NewDataPermissionInterceptor(provider DataPermissionProvider) StatementInterceptor {
	return &dataPermissionInterceptor{provider: provider}
}

func (i *dataPermissionInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	if i != nil && i.provider != nil && statementSupportsCondition(statement.Meta.Command) {
		condition, err := i.provider(ctx, statement.Meta)
		if err != nil {
			return err
		}
		if strings.TrimSpace(condition.SQL) != "" {
			statement.ensureArgs()
			if err := mergeSQLCondition(statement.Args, condition); err != nil {
				return err
			}
			statement.SQL = appendSQLCondition(statement.SQL, condition.SQL)
		}
	}
	return invocation.Proceed(ctx)
}

type tenantInterceptor struct {
	column string
	value  any
}

// NewTenantInterceptor 创建多租户条件注入拦截器。
func NewTenantInterceptor(column string, value any) StatementInterceptor {
	return &tenantInterceptor{column: column, value: value}
}

func (i *tenantInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	if i != nil && statementSupportsCondition(statement.Meta.Command) {
		column, err := quoteIdentifierPath(statement.Dialect, i.column)
		if err != nil {
			return err
		}
		statement.ensureArgs()
		name := nextSQLArgName(statement.Args, "__goark_orm_tenant")
		condition := SQLCondition{
			SQL:  column + " = #{" + name + "}",
			Args: NamedArgs{name: i.value},
		}
		if err := mergeSQLCondition(statement.Args, condition); err != nil {
			return err
		}
		statement.SQL = appendSQLCondition(statement.SQL, condition.SQL)
	}
	return invocation.Proceed(ctx)
}

type dynamicTableInterceptor struct {
	tables map[string]string
}

// NewDynamicTableInterceptor 创建动态表名拦截器。
func NewDynamicTableInterceptor(tables map[string]string) StatementInterceptor {
	copied := make(map[string]string, len(tables))
	for logical, physical := range tables {
		logical = strings.TrimSpace(logical)
		physical = strings.TrimSpace(physical)
		if logical == "" || physical == "" {
			continue
		}
		copied[logical] = physical
	}
	return &dynamicTableInterceptor{tables: copied}
}

func (i *dynamicTableInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	if i != nil && len(i.tables) > 0 {
		rewritten, err := rewriteDynamicTables(statement.SQL, statement.Dialect, i.tables)
		if err != nil {
			return err
		}
		statement.SQL = rewritten
	}
	return invocation.Proceed(ctx)
}

type paginationInterceptor struct{}

type pageRequestContextKey struct{}
type paginationDisabledContextKey struct{}

// WithPageRequest 将分页请求挂到 context，供分页拦截器读取。
func WithPageRequest(ctx context.Context, page PageRequest) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, pageRequestContextKey{}, page)
}

// PageRequestFromContext 从 context 读取分页请求。
func PageRequestFromContext(ctx context.Context) (PageRequest, bool) {
	if ctx == nil {
		return PageRequest{}, false
	}
	page, ok := ctx.Value(pageRequestContextKey{}).(PageRequest)
	return page, ok
}

func withPaginationDisabled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, paginationDisabledContextKey{}, true)
}

func paginationDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	disabled, _ := ctx.Value(paginationDisabledContextKey{}).(bool)
	return disabled
}

// NewPaginationInterceptor 创建分页拦截器。
func NewPaginationInterceptor() StatementInterceptor {
	return paginationInterceptor{}
}

func (paginationInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	if paginationDisabled(ctx) {
		return invocation.Proceed(ctx)
	}
	page, ok := PageRequestFromContext(ctx)
	if ok && statement.Meta.Command == StatementCommandSelect {
		page = page.normalized()
		if page.Size >= 0 {
			statement.ensureArgs()
			limitName := nextSQLArgName(statement.Args, "__goark_orm_page_limit")
			statement.Args[limitName] = page.Size
			offsetName := nextSQLArgName(statement.Args, "__goark_orm_page_offset")
			statement.Args[offsetName] = page.offset()
			statement.SQL = limitOffsetSQL(statement.Dialect, statement.SQL, "#{"+limitName+"}", "#{"+offsetName+"}")
		}
	}
	return invocation.Proceed(ctx)
}

func statementSupportsCondition(command StatementCommand) bool {
	switch command {
	case StatementCommandSelect, StatementCommandUpdate, StatementCommandDelete:
		return true
	default:
		return false
	}
}

func mergeSQLCondition(target NamedArgs, condition SQLCondition) error {
	for key, value := range condition.Args {
		if key == "" {
			return fmt.Errorf("goark-orm: SQL condition argument name is empty")
		}
		if _, exists := target[key]; exists {
			return fmt.Errorf("goark-orm: SQL condition argument %q already exists", key)
		}
		target[key] = value
	}
	return nil
}

func appendSQLCondition(query string, condition string) string {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return query
	}
	head, tail := splitSQLTail(query)
	operator := " WHERE "
	if containsSQLKeyword(head, "where") {
		operator = " AND "
	}
	rewritten := strings.TrimRight(head, " \t\r\n") + operator + condition
	if tail != "" {
		rewritten += " " + tail
	}
	return rewritten
}

func splitSQLTail(query string) (string, string) {
	index := -1
	for _, keyword := range []string{"order", "limit", "offset", "fetch", "for"} {
		if found := findSQLKeyword(query, keyword); found >= 0 && (index < 0 || found < index) {
			index = found
		}
	}
	if index < 0 {
		return query, ""
	}
	return strings.TrimRight(query[:index], " \t\r\n"), strings.TrimSpace(query[index:])
}

func nextSQLArgName(args NamedArgs, prefix string) string {
	if args == nil {
		return prefix
	}
	if _, exists := args[prefix]; !exists {
		return prefix
	}
	for index := 1; ; index++ {
		name := fmt.Sprintf("%s_%d", prefix, index)
		if _, exists := args[name]; !exists {
			return name
		}
	}
}

func rewriteDynamicTables(query string, dialect Dialect, tables map[string]string) (string, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	replacements, err := buildTableReplacements(dialect, tables)
	if err != nil {
		return "", err
	}
	if len(replacements) == 0 {
		return query, nil
	}

	var builder strings.Builder
	builder.Grow(len(query))
	expectTable := false
	for index := 0; index < len(query); {
		if next, ok := skipSQLComment(query, index); ok {
			builder.WriteString(query[index:next])
			index = next
			continue
		}
		switch query[index] {
		case '\'':
			next := skipSQLSingleQuoted(query, index)
			builder.WriteString(query[index:next])
			index = next
			continue
		case '"', '`':
			raw, identifier, next, ok := readSQLQuotedIdentifier(query, index)
			if !ok {
				builder.WriteString(query[index:])
				index = len(query)
				continue
			}
			if expectTable {
				if replacement, found := lookupTableReplacement(replacements, identifier); found {
					builder.WriteString(replacement)
				} else {
					builder.WriteString(raw)
				}
				expectTable = false
			} else {
				builder.WriteString(raw)
			}
			index = next
			continue
		}
		if isSQLIdentStart(query[index]) {
			token, next := readSQLIdentifierPath(query, index)
			if expectTable {
				if replacement, found := lookupTableReplacement(replacements, token); found {
					builder.WriteString(replacement)
				} else {
					builder.WriteString(query[index:next])
				}
				expectTable = false
			} else {
				builder.WriteString(query[index:next])
				if tableLeadKeyword(token) {
					expectTable = true
				}
			}
			index = next
			continue
		}
		if expectTable && !isSQLSpace(query[index]) {
			expectTable = false
		}
		builder.WriteByte(query[index])
		index++
	}
	return builder.String(), nil
}

func buildTableReplacements(dialect Dialect, tables map[string]string) (map[string]string, error) {
	replacements := make(map[string]string, len(tables)*2)
	for logical, physical := range tables {
		logical = strings.TrimSpace(logical)
		physical = strings.TrimSpace(physical)
		if logical == "" || physical == "" {
			continue
		}
		if _, err := quoteIdentifierPath(dialect, logical); err != nil {
			return nil, err
		}
		quotedPhysical, err := quoteIdentifierPath(dialect, physical)
		if err != nil {
			return nil, err
		}
		replacements[logical] = quotedPhysical
		replacements[strings.ToLower(logical)] = quotedPhysical
	}
	return replacements, nil
}

func lookupTableReplacement(replacements map[string]string, identifier string) (string, bool) {
	if replacement, ok := replacements[identifier]; ok {
		return replacement, true
	}
	replacement, ok := replacements[strings.ToLower(identifier)]
	return replacement, ok
}

func tableLeadKeyword(token string) bool {
	switch strings.ToLower(token) {
	case "from", "join", "update", "into":
		return true
	default:
		return false
	}
}

func containsSQLKeyword(query string, keyword string) bool {
	return findSQLKeyword(query, keyword) >= 0
}

func findSQLKeyword(query string, keyword string) int {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return -1
	}
	for index := 0; index < len(query); {
		if next, ok := skipSQLComment(query, index); ok {
			index = next
			continue
		}
		switch query[index] {
		case '\'':
			index = skipSQLSingleQuoted(query, index)
			continue
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(query, index)
			if !ok {
				return -1
			}
			index = next
			continue
		}
		if isSQLIdentStart(query[index]) {
			start := index
			for index < len(query) && isSQLIdentPart(query[index]) {
				index++
			}
			if strings.ToLower(query[start:index]) == keyword {
				return start
			}
			continue
		}
		index++
	}
	return -1
}

func skipSQLComment(query string, index int) (int, bool) {
	if index+1 >= len(query) {
		return index, false
	}
	if query[index] == '-' && query[index+1] == '-' {
		index += 2
		for index < len(query) && query[index] != '\n' {
			index++
		}
		return index, true
	}
	if query[index] == '/' && query[index+1] == '*' {
		index += 2
		for index+1 < len(query) {
			if query[index] == '*' && query[index+1] == '/' {
				return index + 2, true
			}
			index++
		}
		return len(query), true
	}
	return index, false
}

func skipSQLSingleQuoted(query string, index int) int {
	index++
	for index < len(query) {
		if query[index] != '\'' {
			index++
			continue
		}
		if index+1 < len(query) && query[index+1] == '\'' {
			index += 2
			continue
		}
		return index + 1
	}
	return len(query)
}

func readSQLQuotedIdentifier(query string, index int) (string, string, int, bool) {
	quote := query[index]
	var identifier strings.Builder
	next := index + 1
	for next < len(query) {
		if query[next] != quote {
			identifier.WriteByte(query[next])
			next++
			continue
		}
		if next+1 < len(query) && query[next+1] == quote {
			identifier.WriteByte(quote)
			next += 2
			continue
		}
		next++
		return query[index:next], identifier.String(), next, true
	}
	return query[index:], "", len(query), false
}

func readSQLIdentifierPath(query string, index int) (string, int) {
	start := index
	for index < len(query) {
		if !isSQLIdentStart(query[index]) {
			break
		}
		index++
		for index < len(query) && isSQLIdentPart(query[index]) {
			index++
		}
		if index >= len(query) || query[index] != '.' || index+1 >= len(query) || !isSQLIdentStart(query[index+1]) {
			break
		}
		index++
	}
	return query[start:index], index
}

func isSQLIdentStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isSQLIdentPart(ch byte) bool {
	return isSQLIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func isSQLSpace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}
