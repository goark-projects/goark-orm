package orm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Field 描述实体字段对应的数据库列。
type Field[T any] struct {
	Column string
}

// NewField 创建实体字段描述。通常由生成器生成字段常量，业务代码不需要手写列名。
func NewField[T any](column string) Field[T] {
	return Field[T]{Column: column}
}

// TypedField 描述带 Go 值类型的实体字段。
type TypedField[T any, V any] struct {
	Column string
}

// NewTypedField 创建带值类型约束的实体字段描述。
func NewTypedField[T any, V any](column string) TypedField[T, V] {
	return TypedField[T, V]{Column: column}
}

// TypedFieldRef 是可转换为普通字段的类型化字段引用。
type TypedFieldRef[T any] interface {
	Field() Field[T]
}

// Field 返回不携带值类型的字段描述。
func (f TypedField[T, V]) Field() Field[T] {
	return Field[T]{Column: f.Column}
}

type conditionOperator string

const (
	conditionEq         conditionOperator = "="
	conditionNe         conditionOperator = "<>"
	conditionGt         conditionOperator = ">"
	conditionGe         conditionOperator = ">="
	conditionLt         conditionOperator = "<"
	conditionLe         conditionOperator = "<="
	conditionLike       conditionOperator = "LIKE"
	conditionNotLike    conditionOperator = "NOT LIKE"
	conditionIn         conditionOperator = "IN"
	conditionNotIn      conditionOperator = "NOT IN"
	conditionBetween    conditionOperator = "BETWEEN"
	conditionNotBetween conditionOperator = "NOT BETWEEN"
	conditionIsNull     conditionOperator = "IS NULL"
	conditionIsNotNull  conditionOperator = "IS NOT NULL"
)

type conditionConnector string

const (
	conditionConnectorAnd conditionConnector = "AND"
	conditionConnectorOr  conditionConnector = "OR"
)

type queryConditionKind string

const (
	queryConditionPredicate queryConditionKind = ""
	queryConditionNested    queryConditionKind = "nested"
	queryConditionRaw       queryConditionKind = "raw"
	queryConditionExists    queryConditionKind = "exists"
	queryConditionNotExists queryConditionKind = "not_exists"
)

type queryCondition[T any] struct {
	Connector conditionConnector
	Kind      queryConditionKind
	Field     Field[T]
	Op        conditionOperator
	Value     any
	Nested    []queryCondition[T]
	SQL       string
	Args      NamedArgs
}

type queryOrder[T any] struct {
	Field Field[T]
	Desc  bool
}

type rawSQLClause struct {
	SQL  string
	Args NamedArgs
}

// QueryWrapper 是 MyBatis-Plus Wrapper 的 Go 化条件构造器。
type QueryWrapper[T any] struct {
	conditions []queryCondition[T]
	orders     []queryOrder[T]
	groups     []Field[T]
	havings    []rawSQLClause
	selects    []Field[T]
	lastSQL    string
}

type wrapperSQL struct {
	WhereSQL  string
	GroupSQL  string
	HavingSQL string
	OrderSQL  string
	LastSQL   string
	Args      NamedArgs
	Next      int
}

// NewQueryWrapper 创建空查询条件构造器。
func NewQueryWrapper[T any]() *QueryWrapper[T] {
	return &QueryWrapper[T]{}
}

// Empty 返回是否没有任何过滤条件。
func (w *QueryWrapper[T]) Empty() bool {
	return w == nil || len(w.conditions) == 0
}

// When 在 condition 为 true 时应用构造逻辑。
func (w *QueryWrapper[T]) When(condition bool, apply func(*QueryWrapper[T])) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	if condition && apply != nil {
		apply(w)
	}
	return w
}

// Eq 添加等值条件。
func (w *QueryWrapper[T]) Eq(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionEq, value)
}

// EqIf 在 condition 为 true 时添加等值条件。
func (w *QueryWrapper[T]) EqIf(condition bool, field Field[T], value any) *QueryWrapper[T] {
	if !condition {
		return w
	}
	return w.Eq(field, value)
}

// EqTyped 添加类型化字段引用的等值条件。
func (w *QueryWrapper[T]) EqTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.Eq(field.Field(), value)
}

// Ne 添加不等条件。
func (w *QueryWrapper[T]) Ne(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionNe, value)
}

// NeTyped 添加类型化字段引用的不等条件。
func (w *QueryWrapper[T]) NeTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.Ne(field.Field(), value)
}

// Gt 添加大于条件。
func (w *QueryWrapper[T]) Gt(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionGt, value)
}

// GtTyped 添加类型化字段引用的大于条件。
func (w *QueryWrapper[T]) GtTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.Gt(field.Field(), value)
}

// Ge 添加大于等于条件。
func (w *QueryWrapper[T]) Ge(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionGe, value)
}

// GeTyped 添加类型化字段引用的大于等于条件。
func (w *QueryWrapper[T]) GeTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.Ge(field.Field(), value)
}

// Lt 添加小于条件。
func (w *QueryWrapper[T]) Lt(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionLt, value)
}

// LtTyped 添加类型化字段引用的小于条件。
func (w *QueryWrapper[T]) LtTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.Lt(field.Field(), value)
}

// Le 添加小于等于条件。
func (w *QueryWrapper[T]) Le(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionLe, value)
}

// LeTyped 添加类型化字段引用的小于等于条件。
func (w *QueryWrapper[T]) LeTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.Le(field.Field(), value)
}

// Like 添加 LIKE 条件，通配符由调用方显式传入。
func (w *QueryWrapper[T]) Like(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionLike, value)
}

// NotLike 添加 NOT LIKE 条件。
func (w *QueryWrapper[T]) NotLike(field Field[T], value any) *QueryWrapper[T] {
	return w.add(field, conditionNotLike, value)
}

// LikeLeft 添加左侧模糊匹配条件。
func (w *QueryWrapper[T]) LikeLeft(field Field[T], value any) *QueryWrapper[T] {
	return w.Like(field, "%"+fmt.Sprint(value))
}

// LikeRight 添加右侧模糊匹配条件。
func (w *QueryWrapper[T]) LikeRight(field Field[T], value any) *QueryWrapper[T] {
	return w.Like(field, fmt.Sprint(value)+"%")
}

// LikeTyped 添加类型化字段引用的 LIKE 条件。
func (w *QueryWrapper[T]) LikeTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.Like(field.Field(), value)
}

// NotLikeTyped 添加类型化字段引用的 NOT LIKE 条件。
func (w *QueryWrapper[T]) NotLikeTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.NotLike(field.Field(), value)
}

// LikeLeftTyped 添加类型化字段引用的左侧模糊匹配条件。
func (w *QueryWrapper[T]) LikeLeftTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.LikeLeft(field.Field(), value)
}

// LikeRightTyped 添加类型化字段引用的右侧模糊匹配条件。
func (w *QueryWrapper[T]) LikeRightTyped(field TypedFieldRef[T], value any) *QueryWrapper[T] {
	return w.LikeRight(field.Field(), value)
}

// In 添加 IN 条件。空集合会渲染为 1 = 0，避免意外全量命中。
func (w *QueryWrapper[T]) In(field Field[T], values any) *QueryWrapper[T] {
	return w.add(field, conditionIn, values)
}

// NotIn 添加 NOT IN 条件。空集合会渲染为 1 = 0，避免写操作意外放大。
func (w *QueryWrapper[T]) NotIn(field Field[T], values any) *QueryWrapper[T] {
	return w.add(field, conditionNotIn, values)
}

// InTyped 添加类型化字段引用的 IN 条件。
func (w *QueryWrapper[T]) InTyped(field TypedFieldRef[T], values any) *QueryWrapper[T] {
	return w.In(field.Field(), values)
}

// NotInTyped 添加类型化字段引用的 NOT IN 条件。
func (w *QueryWrapper[T]) NotInTyped(field TypedFieldRef[T], values any) *QueryWrapper[T] {
	return w.NotIn(field.Field(), values)
}

// Between 添加 BETWEEN 条件。
func (w *QueryWrapper[T]) Between(field Field[T], left any, right any) *QueryWrapper[T] {
	return w.add(field, conditionBetween, betweenValues{left: left, right: right})
}

// NotBetween 添加 NOT BETWEEN 条件。
func (w *QueryWrapper[T]) NotBetween(field Field[T], left any, right any) *QueryWrapper[T] {
	return w.add(field, conditionNotBetween, betweenValues{left: left, right: right})
}

// BetweenTyped 添加类型化字段引用的 BETWEEN 条件。
func (w *QueryWrapper[T]) BetweenTyped(field TypedFieldRef[T], left any, right any) *QueryWrapper[T] {
	return w.Between(field.Field(), left, right)
}

// NotBetweenTyped 添加类型化字段引用的 NOT BETWEEN 条件。
func (w *QueryWrapper[T]) NotBetweenTyped(field TypedFieldRef[T], left any, right any) *QueryWrapper[T] {
	return w.NotBetween(field.Field(), left, right)
}

// AllEq 按 map 批量添加等值条件。
func (w *QueryWrapper[T]) AllEq(values map[Field[T]]any) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	fields := make([]Field[T], 0, len(values))
	for field := range values {
		fields = append(fields, field)
	}
	sortFields(fields)
	for _, field := range fields {
		value := values[field]
		if isNilValue(value) {
			w.IsNull(field)
			continue
		}
		w.Eq(field, value)
	}
	return w
}

// IsNull 添加 IS NULL 条件。
func (w *QueryWrapper[T]) IsNull(field Field[T]) *QueryWrapper[T] {
	return w.add(field, conditionIsNull, nil)
}

// IsNullTyped 添加类型化字段引用的 IS NULL 条件。
func (w *QueryWrapper[T]) IsNullTyped(field TypedFieldRef[T]) *QueryWrapper[T] {
	return w.IsNull(field.Field())
}

// IsNotNull 添加 IS NOT NULL 条件。
func (w *QueryWrapper[T]) IsNotNull(field Field[T]) *QueryWrapper[T] {
	return w.add(field, conditionIsNotNull, nil)
}

// IsNotNullTyped 添加类型化字段引用的 IS NOT NULL 条件。
func (w *QueryWrapper[T]) IsNotNullTyped(field TypedFieldRef[T]) *QueryWrapper[T] {
	return w.IsNotNull(field.Field())
}

// Nested 添加默认 AND 连接的嵌套条件组。
func (w *QueryWrapper[T]) Nested(apply func(*QueryWrapper[T])) *QueryWrapper[T] {
	return w.addNested(conditionConnectorAnd, apply)
}

// And 添加 AND 连接的嵌套条件组。
func (w *QueryWrapper[T]) And(apply func(*QueryWrapper[T])) *QueryWrapper[T] {
	return w.addNested(conditionConnectorAnd, apply)
}

// Or 添加 OR 连接的嵌套条件组。
func (w *QueryWrapper[T]) Or(apply func(*QueryWrapper[T])) *QueryWrapper[T] {
	return w.addNested(conditionConnectorOr, apply)
}

// Exists 添加 EXISTS 子查询条件，SQL 仅允许 #{name} 参数占位符。
func (w *QueryWrapper[T]) Exists(sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addRaw(conditionConnectorAnd, queryConditionExists, sqlText, args)
}

// NotExists 添加 NOT EXISTS 子查询条件，SQL 仅允许 #{name} 参数占位符。
func (w *QueryWrapper[T]) NotExists(sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addRaw(conditionConnectorAnd, queryConditionNotExists, sqlText, args)
}

// Apply 添加自定义条件片段，SQL 仅允许 #{name} 参数占位符。
func (w *QueryWrapper[T]) Apply(sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addRaw(conditionConnectorAnd, queryConditionRaw, sqlText, args)
}

// OrderByAsc 添加升序排序。
func (w *QueryWrapper[T]) OrderByAsc(field Field[T]) *QueryWrapper[T] {
	return w.addOrder(field, false)
}

// OrderByAscTyped 添加类型化字段引用的升序排序。
func (w *QueryWrapper[T]) OrderByAscTyped(field TypedFieldRef[T]) *QueryWrapper[T] {
	return w.OrderByAsc(field.Field())
}

// OrderByDesc 添加降序排序。
func (w *QueryWrapper[T]) OrderByDesc(field Field[T]) *QueryWrapper[T] {
	return w.addOrder(field, true)
}

// OrderByDescTyped 添加类型化字段引用的降序排序。
func (w *QueryWrapper[T]) OrderByDescTyped(field TypedFieldRef[T]) *QueryWrapper[T] {
	return w.OrderByDesc(field.Field())
}

// OrderBy 按条件和方向添加排序。
func (w *QueryWrapper[T]) OrderBy(condition bool, asc bool, fields ...Field[T]) *QueryWrapper[T] {
	if !condition {
		return w
	}
	for _, field := range fields {
		w = w.addOrder(field, !asc)
	}
	return w
}

// OrderByTyped 按条件和方向添加类型化字段排序。
func (w *QueryWrapper[T]) OrderByTyped(condition bool, asc bool, fields ...TypedFieldRef[T]) *QueryWrapper[T] {
	if !condition {
		return w
	}
	for _, field := range fields {
		w = w.addOrder(field.Field(), !asc)
	}
	return w
}

// Select 指定查询投影字段。
func (w *QueryWrapper[T]) Select(fields ...Field[T]) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.selects = append(w.selects[:0], fields...)
	return w
}

// SelectTyped 指定类型化字段查询投影。
func (w *QueryWrapper[T]) SelectTyped(fields ...TypedFieldRef[T]) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.selects = w.selects[:0]
	for _, field := range fields {
		w.selects = append(w.selects, field.Field())
	}
	return w
}

// GroupBy 添加 GROUP BY 字段。
func (w *QueryWrapper[T]) GroupBy(fields ...Field[T]) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.groups = append(w.groups, fields...)
	return w
}

// GroupByTyped 添加类型化字段引用的 GROUP BY 字段。
func (w *QueryWrapper[T]) GroupByTyped(fields ...TypedFieldRef[T]) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	for _, field := range fields {
		w.groups = append(w.groups, field.Field())
	}
	return w
}

// Having 添加 HAVING 条件片段，SQL 仅允许 #{name} 参数占位符。
func (w *QueryWrapper[T]) Having(sqlText string, args NamedArgs) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.havings = append(w.havings, rawSQLClause{SQL: sqlText, Args: copyNamedArgs(args)})
	return w
}

// Last 添加 SQL 尾部片段，例如 FOR UPDATE。尾部不支持参数绑定。
func (w *QueryWrapper[T]) Last(sqlText string) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.lastSQL = strings.TrimSpace(sqlText)
	return w
}

func (w *QueryWrapper[T]) add(field Field[T], op conditionOperator, value any) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.conditions = append(w.conditions, queryCondition[T]{Connector: conditionConnectorAnd, Field: field, Op: op, Value: value})
	return w
}

func (w *QueryWrapper[T]) addNested(connector conditionConnector, apply func(*QueryWrapper[T])) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	if apply == nil {
		return w
	}
	child := NewQueryWrapper[T]()
	apply(child)
	if len(child.conditions) == 0 {
		return w
	}
	w.conditions = append(w.conditions, queryCondition[T]{
		Connector: connector,
		Kind:      queryConditionNested,
		Nested:    append([]queryCondition[T](nil), child.conditions...),
	})
	return w
}

func (w *QueryWrapper[T]) addRaw(connector conditionConnector, kind queryConditionKind, sqlText string, args NamedArgs) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.conditions = append(w.conditions, queryCondition[T]{
		Connector: connector,
		Kind:      kind,
		SQL:       sqlText,
		Args:      copyNamedArgs(args),
	})
	return w
}

func (w *QueryWrapper[T]) addOrder(field Field[T], desc bool) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.orders = append(w.orders, queryOrder[T]{Field: field, Desc: desc})
	return w
}

func (w *QueryWrapper[T]) build(dialect Dialect, start int) (wrapperSQL, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	if w == nil {
		return wrapperSQL{Args: NamedArgs{}, Next: start}, nil
	}
	conditions, args, seq, err := renderQueryConditions(dialect, w.conditions, start)
	if err != nil {
		return wrapperSQL{}, err
	}
	groups := make([]string, 0, len(w.groups))
	for _, group := range w.groups {
		column, err := quoteIdentifierPath(dialect, group.Column)
		if err != nil {
			return wrapperSQL{}, err
		}
		groups = append(groups, column)
	}
	havings := make([]string, 0, len(w.havings))
	for _, having := range w.havings {
		rendered, next, err := renderRawSQLFragment(having.SQL, having.Args, seq, args)
		if err != nil {
			return wrapperSQL{}, err
		}
		seq = next
		if rendered != "" {
			havings = append(havings, rendered)
		}
	}
	orders := make([]string, 0, len(w.orders))
	for _, order := range w.orders {
		column, err := quoteIdentifierPath(dialect, order.Field.Column)
		if err != nil {
			return wrapperSQL{}, err
		}
		direction := "ASC"
		if order.Desc {
			direction = "DESC"
		}
		orders = append(orders, column+" "+direction)
	}
	lastSQL, err := sanitizeLastSQL(w.lastSQL)
	if err != nil {
		return wrapperSQL{}, err
	}
	result := wrapperSQL{
		WhereSQL: strings.Join(conditions, " "),
		Args:     args,
		Next:     seq,
	}
	if len(groups) > 0 {
		result.GroupSQL = "GROUP BY " + strings.Join(groups, ", ")
	}
	if len(havings) > 0 {
		result.HavingSQL = "HAVING " + strings.Join(havings, " AND ")
	}
	if len(orders) > 0 {
		result.OrderSQL = "ORDER BY " + strings.Join(orders, ", ")
	}
	result.LastSQL = lastSQL
	return result, nil
}

func renderQueryConditions[T any](dialect Dialect, items []queryCondition[T], start int) ([]string, NamedArgs, int, error) {
	args := make(NamedArgs)
	conditions, seq, err := renderQueryConditionsInto(dialect, items, start, args)
	if err != nil {
		return nil, nil, seq, err
	}
	return conditions, args, seq, nil
}

func renderQueryConditionsInto[T any](dialect Dialect, items []queryCondition[T], start int, args NamedArgs) ([]string, int, error) {
	seq := start
	conditions := make([]string, 0, len(items))
	for _, condition := range items {
		rendered, next, err := renderQueryCondition(dialect, condition, seq, args)
		if err != nil {
			return nil, seq, err
		}
		seq = next
		rendered = strings.TrimSpace(rendered)
		if rendered == "" {
			continue
		}
		if len(conditions) == 0 {
			conditions = append(conditions, rendered)
			continue
		}
		connector := condition.Connector
		if connector == "" {
			connector = conditionConnectorAnd
		}
		conditions = append(conditions, string(connector)+" "+rendered)
	}
	return conditions, seq, nil
}

func renderQueryCondition[T any](dialect Dialect, condition queryCondition[T], seq int, args NamedArgs) (string, int, error) {
	switch condition.Kind {
	case queryConditionNested:
		nested, next, err := renderQueryConditionsInto(dialect, condition.Nested, seq, args)
		if err != nil || len(nested) == 0 {
			return "", next, err
		}
		return "(" + strings.Join(nested, " ") + ")", next, nil
	case queryConditionRaw:
		return renderRawSQLFragment(condition.SQL, condition.Args, seq, args)
	case queryConditionExists, queryConditionNotExists:
		rendered, next, err := renderRawSQLFragment(condition.SQL, condition.Args, seq, args)
		if err != nil || rendered == "" {
			return "", next, err
		}
		prefix := "EXISTS"
		if condition.Kind == queryConditionNotExists {
			prefix = "NOT EXISTS"
		}
		return prefix + " (" + rendered + ")", next, nil
	default:
		return renderPredicateCondition(dialect, condition, seq, args)
	}
}

func renderPredicateCondition[T any](dialect Dialect, condition queryCondition[T], seq int, args NamedArgs) (string, int, error) {
	column, err := quoteIdentifierPath(dialect, condition.Field.Column)
	if err != nil {
		return "", seq, err
	}
	switch condition.Op {
	case conditionIsNull, conditionIsNotNull:
		return column + " " + string(condition.Op), seq, nil
	case conditionIn:
		return renderInCondition(column, condition.Value, false, seq, args)
	case conditionNotIn:
		return renderInCondition(column, condition.Value, true, seq, args)
	case conditionBetween, conditionNotBetween:
		return renderBetweenCondition(column, condition.Op, condition.Value, seq, args)
	default:
		name := wrapperArgName(seq)
		seq++
		args[name] = condition.Value
		return column + " " + string(condition.Op) + " #{" + name + "}", seq, nil
	}
}

type betweenValues struct {
	left  any
	right any
}

func renderBetweenCondition(column string, op conditionOperator, value any, seq int, args NamedArgs) (string, int, error) {
	values, ok := value.(betweenValues)
	if !ok {
		return "", seq, fmt.Errorf("goark-orm: BETWEEN value for %s is invalid", column)
	}
	leftName := wrapperArgName(seq)
	seq++
	rightName := wrapperArgName(seq)
	seq++
	args[leftName] = values.left
	args[rightName] = values.right
	return column + " " + string(op) + " #{" + leftName + "} AND #{" + rightName + "}", seq, nil
}

func renderInCondition(column string, values any, not bool, seq int, args NamedArgs) (string, int, error) {
	if isNilValue(values) {
		return "1 = 0", seq, nil
	}
	value := reflect.ValueOf(values)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "1 = 0", seq, nil
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return "", seq, fmt.Errorf("goark-orm: IN value for %s must be slice or array", column)
	}
	if value.Len() == 0 {
		return "1 = 0", seq, nil
	}
	placeholders := make([]string, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		name := wrapperArgName(seq)
		seq++
		args[name] = value.Index(i).Interface()
		placeholders = append(placeholders, "#{"+name+"}")
	}
	operator := "IN"
	if not {
		operator = "NOT IN"
	}
	return column + " " + operator + " (" + strings.Join(placeholders, ", ") + ")", seq, nil
}

func renderRawSQLFragment(sqlText string, rawArgs NamedArgs, seq int, args NamedArgs) (string, int, error) {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return "", seq, nil
	}
	if err := validateRawSQLFragment(sqlText); err != nil {
		return "", seq, err
	}
	matches := statementParamPattern.FindAllStringSubmatchIndex(sqlText, -1)
	if len(matches) == 0 {
		if strings.Contains(sqlText, "#{") {
			return "", seq, fmt.Errorf("goark-orm: raw SQL contains invalid parameter placeholder")
		}
		return sqlText, seq, nil
	}
	var builder strings.Builder
	builder.Grow(len(sqlText))
	offset := 0
	for _, match := range matches {
		builder.WriteString(sqlText[offset:match[0]])
		sourceName := strings.TrimSpace(sqlText[match[2]:match[3]])
		value, ok, err := resolveNamedArg(rawArgs, sourceName)
		if err != nil {
			return "", seq, err
		}
		if !ok {
			return "", seq, fmt.Errorf("goark-orm: raw SQL parameter %q is missing", sourceName)
		}
		targetName := wrapperArgName(seq)
		seq++
		args[targetName] = value
		builder.WriteString("#{" + targetName + "}")
		offset = match[1]
	}
	builder.WriteString(sqlText[offset:])
	rendered := builder.String()
	if strings.Contains(rendered, "#{") && len(statementParamPattern.FindAllStringSubmatch(rendered, -1)) != strings.Count(rendered, "#{") {
		return "", seq, fmt.Errorf("goark-orm: raw SQL contains invalid parameter placeholder")
	}
	return rendered, seq, nil
}

func validateRawSQLFragment(sqlText string) error {
	if strings.Contains(sqlText, "${") {
		return fmt.Errorf("goark-orm: raw SQL uses forbidden ${}")
	}
	if strings.Contains(sqlText, ";") {
		return fmt.Errorf("goark-orm: raw SQL must not contain semicolon")
	}
	return nil
}

func sanitizeLastSQL(sqlText string) (string, error) {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return "", nil
	}
	if err := validateRawSQLFragment(sqlText); err != nil {
		return "", err
	}
	if strings.Contains(sqlText, "#{") {
		return "", fmt.Errorf("goark-orm: last SQL does not support parameters")
	}
	return sqlText, nil
}

func wrapperArgName(seq int) string {
	return "__goark_orm_w_" + strconv.Itoa(seq)
}

func sortFields[T any](fields []Field[T]) {
	for i := 1; i < len(fields); i++ {
		current := fields[i]
		j := i - 1
		for j >= 0 && fields[j].Column > current.Column {
			fields[j+1] = fields[j]
			j--
		}
		fields[j+1] = current
	}
}

func quoteIdentifierPath(dialect Dialect, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", fmt.Errorf("goark-orm: SQL identifier is empty")
	}
	parts := strings.Split(identifier, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !validIdentifierPart(part) {
			return "", fmt.Errorf("goark-orm: invalid SQL identifier %q", identifier)
		}
		quoted = append(quoted, dialect.QuoteIdent(part))
	}
	return strings.Join(quoted, "."), nil
}

func validIdentifierPart(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		if index > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}
