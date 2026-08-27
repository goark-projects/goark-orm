package orm

import (
	"fmt"
	"strings"
)

const sqlBuilderArgPrefix = "__goark_orm_sqlb_"

// SelectSQLBuilder 构造 Provider 可返回的 SELECT SQLSource。
type SelectSQLBuilder struct {
	selects    []string
	from       string
	joins      []sqlBuilderJoin
	conditions []sqlBuilderCondition
	groups     []string
	havings    []rawSQLClause
	orders     []sqlBuilderOrder
	limit      any
	hasLimit   bool
	offset     any
	hasOffset  bool
	rowLock    *sqlBuilderRowLock
	lastSQL    string
	cacheKey   string
}

// InsertSQLBuilder 构造 Provider 可返回的 INSERT SQLSource。
type InsertSQLBuilder struct {
	table      string
	values     []sqlBuilderAssignment
	returnings []string
	cacheKey   string
}

// UpdateSQLBuilder 构造 Provider 可返回的 UPDATE SQLSource。
type UpdateSQLBuilder struct {
	table        string
	sets         []sqlBuilderAssignment
	conditions   []sqlBuilderCondition
	returnings   []string
	requireWhere bool
	cacheKey     string
}

// DeleteSQLBuilder 构造 Provider 可返回的 DELETE SQLSource。
type DeleteSQLBuilder struct {
	table        string
	conditions   []sqlBuilderCondition
	returnings   []string
	requireWhere bool
	cacheKey     string
}

type sqlBuilderAssignment struct {
	column string
	value  any
}

type sqlBuilderOrder struct {
	column string
	desc   bool
}

type sqlBuilderState struct {
	args NamedArgs
	seq  int
}

// NewSelectSQLBuilder 创建 SELECT SQL 构造器。
func NewSelectSQLBuilder() *SelectSQLBuilder {
	return &SelectSQLBuilder{}
}

// NewInsertSQLBuilder 创建 INSERT SQL 构造器。
func NewInsertSQLBuilder() *InsertSQLBuilder {
	return &InsertSQLBuilder{}
}

// NewUpdateSQLBuilder 创建 UPDATE SQL 构造器。
func NewUpdateSQLBuilder() *UpdateSQLBuilder {
	return &UpdateSQLBuilder{}
}

// NewDeleteSQLBuilder 创建 DELETE SQL 构造器。
func NewDeleteSQLBuilder() *DeleteSQLBuilder {
	return &DeleteSQLBuilder{}
}

// Select 指定 SELECT 投影列。未指定时渲染为 `*`。
func (b *SelectSQLBuilder) Select(columns ...string) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.selects = append(b.selects, columns...)
	return b
}

// From 指定 SELECT 来源表。
func (b *SelectSQLBuilder) From(table string) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.from = table
	return b
}

// Where 添加自定义 WHERE 片段，片段只允许 `#{name}` 参数占位符。
func (b *SelectSQLBuilder) Where(sqlText string, args NamedArgs) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionRaw, sql: sqlText, args: copyNamedArgs(args)})
	return b
}

// WhereEq 添加等值 WHERE 条件。
func (b *SelectSQLBuilder) WhereEq(column string, value any) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{column: column, op: "=", value: value})
	return b
}

// GroupBy 添加 GROUP BY 字段。
func (b *SelectSQLBuilder) GroupBy(columns ...string) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.groups = append(b.groups, columns...)
	return b
}

// Having 添加 HAVING 片段，片段只允许 `#{name}` 参数占位符。
func (b *SelectSQLBuilder) Having(sqlText string, args NamedArgs) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.havings = append(b.havings, rawSQLClause{SQL: sqlText, Args: copyNamedArgs(args)})
	return b
}

// OrderByAsc 添加升序排序字段。
func (b *SelectSQLBuilder) OrderByAsc(column string) *SelectSQLBuilder {
	return b.addOrder(column, false)
}

// OrderByDesc 添加降序排序字段。
func (b *SelectSQLBuilder) OrderByDesc(column string) *SelectSQLBuilder {
	return b.addOrder(column, true)
}

// Limit 指定 LIMIT 参数值。
func (b *SelectSQLBuilder) Limit(value any) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.limit = value
	b.hasLimit = true
	return b
}

// Offset 指定 OFFSET 参数值。
func (b *SelectSQLBuilder) Offset(value any) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.offset = value
	b.hasOffset = true
	return b
}

// Last 添加 SQL 尾部片段，尾部片段不允许参数。
func (b *SelectSQLBuilder) Last(sqlText string) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.lastSQL = sqlText
	return b
}

// CacheKey 指定 Provider 额外缓存维度。
func (b *SelectSQLBuilder) CacheKey(cacheKey string) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.cacheKey = cacheKey
	return b
}

func (b *SelectSQLBuilder) addOrder(column string, desc bool) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.orders = append(b.orders, sqlBuilderOrder{column: column, desc: desc})
	return b
}

// Build 构造 SELECT SQLSource。
func (b *SelectSQLBuilder) Build() (SQLSource, error) {
	if b == nil {
		return SQLSource{}, bindingErrorf("select SQL builder is nil")
	}
	state := newSQLBuilderState()
	projections, err := state.identifierList(b.selects, "*")
	if err != nil {
		return SQLSource{}, err
	}
	table, err := state.identifier(b.from)
	if err != nil {
		return SQLSource{}, err
	}
	var parts []string
	parts = append(parts, "SELECT "+strings.Join(projections, ", "), "FROM "+table)
	joinSQL, err := state.joinClause(b.joins)
	if err != nil {
		return SQLSource{}, err
	}
	if joinSQL != "" {
		parts = append(parts, joinSQL)
	}
	whereSQL, err := state.conditions(b.conditions)
	if err != nil {
		return SQLSource{}, err
	}
	if whereSQL != "" {
		parts = append(parts, "WHERE "+whereSQL)
	}
	groupSQL, err := state.identifierClause("GROUP BY", b.groups)
	if err != nil {
		return SQLSource{}, err
	}
	if groupSQL != "" {
		parts = append(parts, groupSQL)
	}
	havingSQL, err := state.rawClauses("HAVING", b.havings)
	if err != nil {
		return SQLSource{}, err
	}
	if havingSQL != "" {
		parts = append(parts, havingSQL)
	}
	orderSQL, err := state.orderClause(b.orders)
	if err != nil {
		return SQLSource{}, err
	}
	if orderSQL != "" {
		parts = append(parts, orderSQL)
	}
	if b.hasLimit {
		parts = append(parts, "LIMIT "+state.value(b.limit))
	}
	if b.hasOffset {
		parts = append(parts, "OFFSET "+state.value(b.offset))
	}
	if b.rowLock != nil {
		lockClause, err := RowLockClause(b.rowLock.dialect, b.rowLock.options)
		if err != nil {
			return SQLSource{}, err
		}
		parts = append(parts, lockClause)
	}
	lastSQL, err := sanitizeLastSQL(b.lastSQL)
	if err != nil {
		return SQLSource{}, err
	}
	if lastSQL != "" {
		parts = append(parts, lastSQL)
	}
	return state.source(strings.Join(parts, " "), b.cacheKey), nil
}

// Into 指定 INSERT 目标表。
func (b *InsertSQLBuilder) Into(table string) *InsertSQLBuilder {
	if b == nil {
		b = NewInsertSQLBuilder()
	}
	b.table = table
	return b
}

// Value 添加 INSERT 列和值。
func (b *InsertSQLBuilder) Value(column string, value any) *InsertSQLBuilder {
	if b == nil {
		b = NewInsertSQLBuilder()
	}
	b.values = append(b.values, sqlBuilderAssignment{column: column, value: value})
	return b
}

// CacheKey 指定 Provider 额外缓存维度。
func (b *InsertSQLBuilder) CacheKey(cacheKey string) *InsertSQLBuilder {
	if b == nil {
		b = NewInsertSQLBuilder()
	}
	b.cacheKey = cacheKey
	return b
}

// Build 构造 INSERT SQLSource。
func (b *InsertSQLBuilder) Build() (SQLSource, error) {
	if b == nil {
		return SQLSource{}, bindingErrorf("insert SQL builder is nil")
	}
	if len(b.values) == 0 {
		return SQLSource{}, bindingErrorf("insert SQL builder requires at least one value")
	}
	state := newSQLBuilderState()
	table, err := state.identifier(b.table)
	if err != nil {
		return SQLSource{}, err
	}
	columns := make([]string, 0, len(b.values))
	values := make([]string, 0, len(b.values))
	for _, assignment := range b.values {
		column, err := state.identifier(assignment.column)
		if err != nil {
			return SQLSource{}, err
		}
		columns = append(columns, column)
		values = append(values, state.value(assignment.value))
	}
	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(values, ", "))
	returningSQL, err := state.identifierClause("RETURNING", b.returnings)
	if err != nil {
		return SQLSource{}, err
	}
	if returningSQL != "" {
		sqlText += " " + returningSQL
	}
	return state.source(sqlText, b.cacheKey), nil
}

// Table 指定 UPDATE 目标表。
func (b *UpdateSQLBuilder) Table(table string) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.table = table
	return b
}

// Set 添加 UPDATE SET 列和值。
func (b *UpdateSQLBuilder) Set(column string, value any) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.sets = append(b.sets, sqlBuilderAssignment{column: column, value: value})
	return b
}

// Where 添加自定义 WHERE 片段，片段只允许 `#{name}` 参数占位符。
func (b *UpdateSQLBuilder) Where(sqlText string, args NamedArgs) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionRaw, sql: sqlText, args: copyNamedArgs(args)})
	return b
}

// WhereEq 添加等值 WHERE 条件。
func (b *UpdateSQLBuilder) WhereEq(column string, value any) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{column: column, op: "=", value: value})
	return b
}

// CacheKey 指定 Provider 额外缓存维度。
func (b *UpdateSQLBuilder) CacheKey(cacheKey string) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.cacheKey = cacheKey
	return b
}

// Build 构造 UPDATE SQLSource。
func (b *UpdateSQLBuilder) Build() (SQLSource, error) {
	if b == nil {
		return SQLSource{}, bindingErrorf("update SQL builder is nil")
	}
	if len(b.sets) == 0 {
		return SQLSource{}, bindingErrorf("update SQL builder requires at least one set")
	}
	state := newSQLBuilderState()
	table, err := state.identifier(b.table)
	if err != nil {
		return SQLSource{}, err
	}
	sets := make([]string, 0, len(b.sets))
	for _, assignment := range b.sets {
		column, err := state.identifier(assignment.column)
		if err != nil {
			return SQLSource{}, err
		}
		sets = append(sets, column+" = "+state.value(assignment.value))
	}
	sqlText := "UPDATE " + table + " SET " + strings.Join(sets, ", ")
	whereSQL, err := state.conditions(b.conditions)
	if err != nil {
		return SQLSource{}, err
	}
	if whereSQL != "" {
		sqlText += " WHERE " + whereSQL
	} else if b.requireWhere {
		return SQLSource{}, bindingErrorf("update SQL builder requires WHERE condition")
	}
	returningSQL, err := state.identifierClause("RETURNING", b.returnings)
	if err != nil {
		return SQLSource{}, err
	}
	if returningSQL != "" {
		sqlText += " " + returningSQL
	}
	return state.source(sqlText, b.cacheKey), nil
}

// From 指定 DELETE 目标表。
func (b *DeleteSQLBuilder) From(table string) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.table = table
	return b
}

// Where 添加自定义 WHERE 片段，片段只允许 `#{name}` 参数占位符。
func (b *DeleteSQLBuilder) Where(sqlText string, args NamedArgs) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionRaw, sql: sqlText, args: copyNamedArgs(args)})
	return b
}

// WhereEq 添加等值 WHERE 条件。
func (b *DeleteSQLBuilder) WhereEq(column string, value any) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{column: column, op: "=", value: value})
	return b
}

// CacheKey 指定 Provider 额外缓存维度。
func (b *DeleteSQLBuilder) CacheKey(cacheKey string) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.cacheKey = cacheKey
	return b
}

// Build 构造 DELETE SQLSource。
func (b *DeleteSQLBuilder) Build() (SQLSource, error) {
	if b == nil {
		return SQLSource{}, bindingErrorf("delete SQL builder is nil")
	}
	state := newSQLBuilderState()
	table, err := state.identifier(b.table)
	if err != nil {
		return SQLSource{}, err
	}
	sqlText := "DELETE FROM " + table
	whereSQL, err := state.conditions(b.conditions)
	if err != nil {
		return SQLSource{}, err
	}
	if whereSQL != "" {
		sqlText += " WHERE " + whereSQL
	} else if b.requireWhere {
		return SQLSource{}, bindingErrorf("delete SQL builder requires WHERE condition")
	}
	returningSQL, err := state.identifierClause("RETURNING", b.returnings)
	if err != nil {
		return SQLSource{}, err
	}
	if returningSQL != "" {
		sqlText += " " + returningSQL
	}
	return state.source(sqlText, b.cacheKey), nil
}

func newSQLBuilderState() *sqlBuilderState {
	return &sqlBuilderState{args: NamedArgs{}}
}

func (s *sqlBuilderState) source(sqlText string, cacheKey string) SQLSource {
	return SQLSource{
		SQL:      strings.TrimSpace(sqlText),
		Args:     copyNamedArgs(s.args),
		CacheKey: strings.TrimSpace(cacheKey),
	}
}

func (s *sqlBuilderState) identifier(identifier string) (string, error) {
	token, err := NewRawIdentifier(identifier)
	if err != nil {
		return "", err
	}
	name := s.next()
	s.args[name] = token
	return "${" + name + "}", nil
}

func (s *sqlBuilderState) identifierList(identifiers []string, fallback string) ([]string, error) {
	if len(identifiers) == 0 {
		return []string{fallback}, nil
	}
	out := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		rendered, err := s.identifier(identifier)
		if err != nil {
			return nil, err
		}
		out = append(out, rendered)
	}
	return out, nil
}

func (s *sqlBuilderState) identifierClause(prefix string, identifiers []string) (string, error) {
	if len(identifiers) == 0 {
		return "", nil
	}
	rendered, err := s.identifierList(identifiers, "")
	if err != nil {
		return "", err
	}
	return prefix + " " + strings.Join(rendered, ", "), nil
}

func (s *sqlBuilderState) value(value any) string {
	name := s.next()
	s.args[name] = value
	return "#{" + name + "}"
}

func (s *sqlBuilderState) rawClauses(prefix string, clauses []rawSQLClause) (string, error) {
	if len(clauses) == 0 {
		return "", nil
	}
	rendered := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		sqlText, next, err := renderRawSQLFragment(clause.SQL, clause.Args, s.seq, s.args)
		if err != nil {
			return "", err
		}
		s.seq = next
		if sqlText != "" {
			rendered = append(rendered, sqlText)
		}
	}
	if len(rendered) == 0 {
		return "", nil
	}
	return prefix + " " + strings.Join(rendered, " AND "), nil
}

func (s *sqlBuilderState) orderClause(orders []sqlBuilderOrder) (string, error) {
	if len(orders) == 0 {
		return "", nil
	}
	rendered := make([]string, 0, len(orders))
	for _, order := range orders {
		column, err := s.identifier(order.column)
		if err != nil {
			return "", err
		}
		direction := "ASC"
		if order.desc {
			direction = "DESC"
		}
		rendered = append(rendered, column+" "+direction)
	}
	return "ORDER BY " + strings.Join(rendered, ", "), nil
}

func (s *sqlBuilderState) next() string {
	name := fmt.Sprintf("%s%d", sqlBuilderArgPrefix, s.seq)
	s.seq++
	return name
}
