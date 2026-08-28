package runtime

import (
	"context"
	"fmt"
	"strings"
)

// StatementRuntime 描述一次执行中的可变 SQL 模板。
type StatementRuntime struct {
	Meta          StatementMeta
	SQL           string
	Args          NamedArgs
	CacheKey      string
	Dialect       Dialect
	Configuration Configuration
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
	if StatementInterceptorIgnored(statement.Meta, InterceptorNameSQLObserver) {
		return nil
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
	if StatementInterceptorIgnored(statement.Meta, InterceptorNameBlockAttack) {
		return nil
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
	if i != nil && i.provider != nil && !StatementInterceptorIgnored(statement.Meta, InterceptorNameDataPermission) && statementSupportsCondition(statement.Meta.Command) {
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
	if i != nil && !StatementInterceptorIgnored(statement.Meta, InterceptorNameTenant) {
		switch {
		case statement.Meta.Command == StatementCommandInsert:
			if err := i.interceptInsert(statement); err != nil {
				return err
			}
		case statementSupportsCondition(statement.Meta.Command):
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
	}
	return invocation.Proceed(ctx)
}

func (i *tenantInterceptor) interceptInsert(statement *StatementRuntime) error {
	column, err := quoteIdentifierPath(statement.Dialect, i.column)
	if err != nil {
		return err
	}
	statement.ensureArgs()
	name := nextSQLArgName(statement.Args, "__goark_orm_tenant")
	rewritten, injected, err := appendTenantInsertColumn(statement.SQL, i.column, column, name)
	if err != nil {
		return err
	}
	if !injected {
		return nil
	}
	statement.SQL = rewritten
	statement.Args[name] = i.value
	return nil
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
	if i != nil && len(i.tables) > 0 && !StatementInterceptorIgnored(statement.Meta, InterceptorNameDynamicTable) {
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
	if paginationDisabled(ctx) || StatementInterceptorIgnored(statement.Meta, InterceptorNamePagination) {
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
	head, tail := splitSQLConditionTail(query)
	operator := " WHERE "
	if containsSQLKeyword(head, "where") {
		operator = " AND "
	}
	trimmedHead := strings.TrimRight(head, " \t\r\n")
	var builder strings.Builder
	builder.Grow(len(trimmedHead) + len(operator) + len(condition) + len(tail) + 1)
	builder.WriteString(trimmedHead)
	builder.WriteString(operator)
	builder.WriteString(condition)
	if tail != "" {
		builder.WriteByte(' ')
		builder.WriteString(tail)
	}
	return builder.String()
}

func appendTenantInsertColumn(query string, rawColumn string, quotedColumn string, argName string) (string, bool, error) {
	intoIndex := findSQLKeyword(query, "into")
	if intoIndex < 0 {
		return "", false, fmt.Errorf("goark-orm: tenant insert requires INSERT INTO statement")
	}
	index := skipSQLSpacesAndComments(query, intoIndex+len("into"))
	next, ok := readSQLTableReference(query, index)
	if !ok {
		return "", false, fmt.Errorf("goark-orm: tenant insert table is missing")
	}
	index = skipSQLSpacesAndComments(query, next)
	if index >= len(query) || query[index] != '(' {
		return "", false, fmt.Errorf("goark-orm: tenant insert requires explicit column list")
	}
	columnsEnd, ok := findClosingSQLParen(query, index)
	if !ok {
		return "", false, fmt.Errorf("goark-orm: tenant insert column list is not closed")
	}
	columnsText := query[index+1 : columnsEnd]
	if insertColumnListContains(columnsText, rawColumn) {
		return query, false, nil
	}
	valuesIndex := findSQLKeyword(query[columnsEnd+1:], "values")
	if valuesIndex < 0 {
		return "", false, fmt.Errorf("goark-orm: tenant insert supports only VALUES insert")
	}
	valuesIndex += columnsEnd + 1
	valuesSQL, err := appendTenantInsertValues(query, valuesIndex+len("values"), "#{"+argName+"}")
	if err != nil {
		return "", false, err
	}
	var builder strings.Builder
	builder.Grow(len(query) + len(quotedColumn) + strings.Count(valuesSQL, "#{"+argName+"}")*len(argName))
	builder.WriteString(query[:columnsEnd])
	builder.WriteString(", ")
	builder.WriteString(quotedColumn)
	builder.WriteByte(')')
	builder.WriteString(query[columnsEnd+1 : valuesIndex+len("values")])
	builder.WriteString(valuesSQL)
	return builder.String(), true, nil
}

func appendTenantInsertValues(query string, valuesStart int, placeholder string) (string, error) {
	var builder strings.Builder
	cursor := valuesStart
	expectRow := true
	rows := 0
	for cursor <= len(query) {
		index := skipSQLSpacesAndComments(query, cursor)
		if expectRow {
			if index >= len(query) || query[index] != '(' {
				return "", fmt.Errorf("goark-orm: tenant insert VALUES row is missing")
			}
			closeIndex, ok := findClosingSQLParen(query, index)
			if !ok {
				return "", fmt.Errorf("goark-orm: tenant insert VALUES row is not closed")
			}
			builder.WriteString(query[cursor:closeIndex])
			builder.WriteString(", ")
			builder.WriteString(placeholder)
			builder.WriteByte(')')
			cursor = closeIndex + 1
			expectRow = false
			rows++
			continue
		}
		if index < len(query) && query[index] == ',' {
			builder.WriteString(query[cursor : index+1])
			cursor = index + 1
			expectRow = true
			continue
		}
		builder.WriteString(query[cursor:])
		if rows == 0 {
			return "", fmt.Errorf("goark-orm: tenant insert VALUES row is missing")
		}
		return builder.String(), nil
	}
	return "", fmt.Errorf("goark-orm: tenant insert VALUES row is missing")
}

func insertColumnListContains(columns string, column string) bool {
	target := normalizeColumnKey(lastSQLIdentifierPart(column))
	if target == "" {
		return false
	}
	for _, item := range splitSQLTopLevelComma(columns) {
		if normalizeColumnKey(lastSQLIdentifierPart(unquoteSQLIdentifier(strings.TrimSpace(item)))) == target {
			return true
		}
	}
	return false
}

func splitSQLTopLevelComma(value string) []string {
	items := make([]string, 0, 4)
	start := 0
	depth := 0
	for index := 0; index < len(value); {
		if next, ok := skipSQLComment(value, index); ok {
			index = next
			continue
		}
		switch value[index] {
		case '\'':
			index = skipSQLSingleQuoted(value, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(value, index); ok {
				index = next
				continue
			}
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(value, index)
			if !ok {
				index = len(value)
			} else {
				index = next
			}
			continue
		case '[':
			index = skipSQLBracketQuotedIdentifier(value, index)
			continue
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				items = append(items, value[start:index])
				start = index + 1
			}
		}
		index++
	}
	items = append(items, value[start:])
	return items
}

func lastSQLIdentifierPart(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return ""
	}
	if index := strings.LastIndex(identifier, "."); index >= 0 {
		identifier = identifier[index+1:]
	}
	return strings.TrimSpace(identifier)
}

func unquoteSQLIdentifier(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if len(identifier) < 2 {
		return identifier
	}
	first := identifier[0]
	last := identifier[len(identifier)-1]
	switch {
	case first == '"' && last == '"':
		return strings.ReplaceAll(identifier[1:len(identifier)-1], `""`, `"`)
	case first == '`' && last == '`':
		return strings.ReplaceAll(identifier[1:len(identifier)-1], "``", "`")
	case first == '[' && last == ']':
		return identifier[1 : len(identifier)-1]
	default:
		return identifier
	}
}

func skipSQLSpacesAndComments(query string, index int) int {
	for index < len(query) {
		if isSQLSpace(query[index]) {
			index++
			continue
		}
		if next, ok := skipSQLComment(query, index); ok {
			index = next
			continue
		}
		return index
	}
	return index
}

func readSQLTableReference(query string, index int) (int, bool) {
	next, ok := readSQLIdentifierSegment(query, index)
	if !ok {
		return index, false
	}
	index = next
	for index < len(query) && query[index] == '.' {
		next, ok = readSQLIdentifierSegment(query, index+1)
		if !ok {
			break
		}
		index = next
	}
	return index, true
}

func readSQLIdentifierSegment(query string, index int) (int, bool) {
	if index >= len(query) {
		return index, false
	}
	switch query[index] {
	case '"', '`':
		_, _, next, ok := readSQLQuotedIdentifier(query, index)
		return next, ok
	case '[':
		_, next, ok := readSQLBracketQuotedIdentifier(query, index)
		return next, ok
	default:
		if !isSQLIdentStart(query[index]) {
			return index, false
		}
		index++
		for index < len(query) && isSQLIdentPart(query[index]) {
			index++
		}
		return index, true
	}
}

func findClosingSQLParen(query string, openIndex int) (int, bool) {
	if openIndex >= len(query) || query[openIndex] != '(' {
		return openIndex, false
	}
	depth := 0
	for index := openIndex; index < len(query); {
		if next, ok := skipSQLComment(query, index); ok {
			index = next
			continue
		}
		switch query[index] {
		case '\'':
			index = skipSQLSingleQuoted(query, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(query, index); ok {
				index = next
				continue
			}
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(query, index)
			if !ok {
				return len(query), false
			}
			index = next
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return index, true
			}
		}
		index++
	}
	return len(query), false
}

func splitSQLTail(query string) (string, string) {
	index := findSQLTailStart(query, false)
	if index < 0 {
		return query, ""
	}
	return strings.TrimRight(query[:index], " \t\r\n"), strings.TrimSpace(query[index:])
}

func splitSQLConditionTail(query string) (string, string) {
	index := findSQLTailStart(query, true)
	if index < 0 {
		return query, ""
	}
	return strings.TrimRight(query[:index], " \t\r\n"), strings.TrimSpace(query[index:])
}

func findSQLTailStart(query string, includeGrouping bool) int {
	depth := 0
	for index := sqlTailScanStart(query); index < len(query); {
		if next, ok := skipSQLComment(query, index); ok {
			index = next
			continue
		}
		switch query[index] {
		case '\'':
			index = skipSQLSingleQuoted(query, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(query, index); ok {
				index = next
				continue
			}
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(query, index)
			if !ok {
				return -1
			}
			index = next
			continue
		case '[':
			index = skipSQLBracketQuotedIdentifier(query, index)
			continue
		case '(':
			depth++
			index++
			continue
		case ')':
			if depth > 0 {
				depth--
			}
			index++
			continue
		}
		if depth == 0 && isSQLIdentStart(query[index]) {
			start := index
			for index < len(query) && isSQLIdentPart(query[index]) {
				index++
			}
			if isSQLTailClause(query, start, index, includeGrouping) {
				return start
			}
			continue
		}
		index++
	}
	return -1
}

func sqlTailScanStart(query string) int {
	index := skipSQLSpacesAndComments(query, 0)
	if hasSQLKeywordAt(query, index, "select") {
		fromIndex := findSQLKeyword(query, "from")
		if fromIndex < 0 {
			return len(query)
		}
		return fromIndex + len("from")
	}
	if !hasSQLKeywordAt(query, index, "with") || findSQLKeyword(query, "select") < 0 {
		return 0
	}
	fromIndex := findSQLKeyword(query, "from")
	if fromIndex < 0 {
		return len(query)
	}
	return fromIndex + len("from")
}

func isSQLTailClause(query string, tokenStart int, tokenEnd int, includeGrouping bool) bool {
	next := skipSQLSpacesAndComments(query, tokenEnd)
	switch {
	case sqlTokenEquals(query, tokenStart, tokenEnd, "order"):
		return hasSQLKeywordAt(query, next, "by")
	case sqlTokenEquals(query, tokenStart, tokenEnd, "group"):
		return includeGrouping && hasSQLKeywordAt(query, next, "by")
	case sqlTokenEquals(query, tokenStart, tokenEnd, "having"):
		return includeGrouping && !sqlTokenFollowedByPredicateOperator(query, next)
	case sqlTokenEquals(query, tokenStart, tokenEnd, "fetch"):
		return hasSQLKeywordAt(query, next, "first") || hasSQLKeywordAt(query, next, "next")
	case sqlTokenEquals(query, tokenStart, tokenEnd, "for"):
		return hasSQLKeywordAt(query, next, "update") ||
			hasSQLKeywordAt(query, next, "share") ||
			hasSQLKeywordAt(query, next, "no") ||
			hasSQLKeywordAt(query, next, "key")
	case sqlTokenEquals(query, tokenStart, tokenEnd, "limit") ||
		sqlTokenEquals(query, tokenStart, tokenEnd, "offset"):
		return sqlTailArgumentStartsAt(query, next)
	default:
		return false
	}
}

func sqlTailArgumentStartsAt(query string, index int) bool {
	if index >= len(query) {
		return false
	}
	ch := query[index]
	return (ch >= '0' && ch <= '9') ||
		ch == '?' ||
		ch == ':' ||
		ch == '(' ||
		ch == '#' ||
		ch == '$' ||
		hasSQLKeywordAt(query, index, "all")
}

func sqlTokenFollowedByPredicateOperator(query string, index int) bool {
	if index >= len(query) {
		return false
	}
	switch query[index] {
	case '=', '<', '>', '!':
		return true
	}
	return hasSQLKeywordAt(query, index, "is") ||
		hasSQLKeywordAt(query, index, "in") ||
		hasSQLKeywordAt(query, index, "like") ||
		hasSQLKeywordAt(query, index, "ilike") ||
		hasSQLKeywordAt(query, index, "between") ||
		hasSQLKeywordAt(query, index, "not")
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
	return strings.EqualFold(token, "from") ||
		strings.EqualFold(token, "join") ||
		strings.EqualFold(token, "update") ||
		strings.EqualFold(token, "into")
}

func containsSQLKeyword(query string, keyword string) bool {
	return findSQLKeyword(query, keyword) >= 0
}

func findSQLKeyword(query string, keyword string) int {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return -1
	}
	depth := 0
	for index := 0; index < len(query); {
		if next, ok := skipSQLComment(query, index); ok {
			index = next
			continue
		}
		switch query[index] {
		case '\'':
			index = skipSQLSingleQuoted(query, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(query, index); ok {
				index = next
				continue
			}
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(query, index)
			if !ok {
				return -1
			}
			index = next
			continue
		case '[':
			index = skipSQLBracketQuotedIdentifier(query, index)
			continue
		case '(':
			depth++
			index++
			continue
		case ')':
			if depth > 0 {
				depth--
			}
			index++
			continue
		}
		if depth == 0 && isSQLIdentStart(query[index]) {
			start := index
			for index < len(query) && isSQLIdentPart(query[index]) {
				index++
			}
			if sqlTokenEquals(query, start, index, keyword) {
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

func skipSQLPlaceholder(query string, index int) (int, bool) {
	if index+1 >= len(query) || (query[index] != '#' && query[index] != '$') || query[index+1] != '{' {
		return index, false
	}
	index += 2
	for index < len(query) {
		if query[index] == '}' {
			return index + 1, true
		}
		index++
	}
	return len(query), true
}

func skipSQLBracketQuotedIdentifier(query string, index int) int {
	index++
	for index < len(query) {
		if query[index] != ']' {
			index++
			continue
		}
		if index+1 < len(query) && query[index+1] == ']' {
			index += 2
			continue
		}
		return index + 1
	}
	return len(query)
}

func readSQLBracketQuotedIdentifier(query string, index int) (string, int, bool) {
	index++
	var identifier strings.Builder
	for index < len(query) {
		if query[index] != ']' {
			identifier.WriteByte(query[index])
			index++
			continue
		}
		if index+1 < len(query) && query[index+1] == ']' {
			identifier.WriteByte(']')
			index += 2
			continue
		}
		return identifier.String(), index + 1, true
	}
	return "", len(query), false
}

func hasSQLKeywordAt(query string, index int, keyword string) bool {
	if index < 0 || index >= len(query) || !isSQLIdentStart(query[index]) {
		return false
	}
	start := index
	for index < len(query) && isSQLIdentPart(query[index]) {
		index++
	}
	return sqlTokenEquals(query, start, index, keyword)
}

func sqlTokenEquals(query string, start int, end int, keyword string) bool {
	return start >= 0 && end <= len(query) && end-start == len(keyword) && strings.EqualFold(query[start:end], keyword)
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

func readSQLIdentifierNamePath(query string, index int) (string, int, bool) {
	name, next, ok := readSQLIdentifierNameSegment(query, index)
	if !ok {
		return "", index, false
	}
	last := name
	for next < len(query) && query[next] == '.' {
		name, after, ok := readSQLIdentifierNameSegment(query, next+1)
		if !ok {
			break
		}
		last = name
		next = after
	}
	return last, next, true
}

func readSQLIdentifierNameSegment(query string, index int) (string, int, bool) {
	if index >= len(query) {
		return "", index, false
	}
	switch query[index] {
	case '"', '`':
		_, identifier, next, ok := readSQLQuotedIdentifier(query, index)
		return identifier, next, ok
	case '[':
		return readSQLBracketQuotedIdentifier(query, index)
	default:
		if !isSQLIdentStart(query[index]) {
			return "", index, false
		}
		start := index
		index++
		for index < len(query) && isSQLIdentPart(query[index]) {
			index++
		}
		return query[start:index], index, true
	}
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
