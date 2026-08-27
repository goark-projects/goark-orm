package orm

import (
	"fmt"
	"strconv"
	"strings"
)

type updateAssignment[T any] struct {
	Field Field[T]
	Value any
	Op    updateAssignmentOperator
	SQL   string
	Args  NamedArgs
}

type updateAssignmentOperator string

const (
	updateAssignmentSet    updateAssignmentOperator = ""
	updateAssignmentRaw    updateAssignmentOperator = "raw"
	updateAssignmentIncrBy updateAssignmentOperator = "incr"
	updateAssignmentDecrBy updateAssignmentOperator = "decr"
)

// UpdateWrapper 是实体更新构造器。
type UpdateWrapper[T any] struct {
	assignments []updateAssignment[T]
	conditions  []queryCondition[T]
	lastSQL     string
}

type updateWrapperSQL struct {
	SetSQL   string
	WhereSQL string
	LastSQL  string
	Args     NamedArgs
	Next     int
}

// NewUpdateWrapper 创建空更新构造器。
func NewUpdateWrapper[T any]() *UpdateWrapper[T] {
	return &UpdateWrapper[T]{}
}

// Empty 返回是否没有任何过滤条件。
func (w *UpdateWrapper[T]) Empty() bool {
	return w == nil || len(w.conditions) == 0
}

// SetEmpty 返回是否没有任何更新字段。
func (w *UpdateWrapper[T]) SetEmpty() bool {
	return w == nil || len(w.assignments) == 0
}

// When 在 condition 为 true 时应用构造逻辑。
func (w *UpdateWrapper[T]) When(condition bool, apply func(*UpdateWrapper[T])) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	if condition && apply != nil {
		apply(w)
	}
	return w
}

// Set 添加字段赋值。
func (w *UpdateWrapper[T]) Set(field Field[T], value any) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	w.assignments = append(w.assignments, updateAssignment[T]{Field: field, Value: value, Op: updateAssignmentSet})
	return w
}

// SetIf 在 condition 为 true 时添加字段赋值。
func (w *UpdateWrapper[T]) SetIf(condition bool, field Field[T], value any) *UpdateWrapper[T] {
	if !condition {
		return w
	}
	return w.Set(field, value)
}

// SetTyped 添加类型化字段引用的字段赋值。
func (w *UpdateWrapper[T]) SetTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Set(field.Field(), value)
}

// SetTypedIf 在 condition 为 true 时添加类型化字段引用的字段赋值。
func (w *UpdateWrapper[T]) SetTypedIf(condition bool, field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	if !condition {
		return w
	}
	return w.SetTyped(field, value)
}

// SetTypedValue 用泛型函数保留字段值类型约束。
func SetTypedValue[T any, V any](w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	return w.Set(field.Field(), value)
}

// SetTypedValueIf 在 condition 为 true 时添加类型安全的字段赋值。
func SetTypedValueIf[T any, V any](condition bool, w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	if !condition {
		return w
	}
	return SetTypedValue(w, field, value)
}

// SetSQL 添加安全的原生 SET 片段，SQL 仅允许 #{name} 参数占位符。
func (w *UpdateWrapper[T]) SetSQL(sqlText string, args NamedArgs) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return w
	}
	w.assignments = append(w.assignments, updateAssignment[T]{
		Op:   updateAssignmentRaw,
		SQL:  sqlText,
		Args: copyNamedArgs(args),
	})
	return w
}

// SetIncrBy 添加字段自增 SET 片段。
func (w *UpdateWrapper[T]) SetIncrBy(field Field[T], value any) *UpdateWrapper[T] {
	return w.addSetArithmetic(field, value, updateAssignmentIncrBy)
}

// SetDecrBy 添加字段自减 SET 片段。
func (w *UpdateWrapper[T]) SetDecrBy(field Field[T], value any) *UpdateWrapper[T] {
	return w.addSetArithmetic(field, value, updateAssignmentDecrBy)
}

// SetIncrByTyped 添加类型化字段引用的字段自增 SET 片段。
func (w *UpdateWrapper[T]) SetIncrByTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.SetIncrBy(field.Field(), value)
}

// SetDecrByTyped 添加类型化字段引用的字段自减 SET 片段。
func (w *UpdateWrapper[T]) SetDecrByTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.SetDecrBy(field.Field(), value)
}

// SetIncrByTypedValue 用泛型函数保留字段值类型约束并添加字段自增 SET 片段。
func SetIncrByTypedValue[T any, V any](w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	return w.SetIncrBy(field.Field(), value)
}

// SetIncrByTypedValueIf 在 condition 为 true 时添加类型安全的字段自增 SET 片段。
func SetIncrByTypedValueIf[T any, V any](condition bool, w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	if !condition {
		return w
	}
	return SetIncrByTypedValue(w, field, value)
}

// SetDecrByTypedValue 用泛型函数保留字段值类型约束并添加字段自减 SET 片段。
func SetDecrByTypedValue[T any, V any](w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	return w.SetDecrBy(field.Field(), value)
}

// SetDecrByTypedValueIf 在 condition 为 true 时添加类型安全的字段自减 SET 片段。
func SetDecrByTypedValueIf[T any, V any](condition bool, w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	if !condition {
		return w
	}
	return SetDecrByTypedValue(w, field, value)
}

// Eq 添加等值条件。
func (w *UpdateWrapper[T]) Eq(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionEq, value)
}

// EqIf 在 condition 为 true 时添加等值条件。
func (w *UpdateWrapper[T]) EqIf(condition bool, field Field[T], value any) *UpdateWrapper[T] {
	if !condition {
		return w
	}
	return w.Eq(field, value)
}

// EqTyped 添加类型化字段引用的等值条件。
func (w *UpdateWrapper[T]) EqTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Eq(field.Field(), value)
}

// Ne 添加不等条件。
func (w *UpdateWrapper[T]) Ne(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionNe, value)
}

// NeTyped 添加类型化字段引用的不等条件。
func (w *UpdateWrapper[T]) NeTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Ne(field.Field(), value)
}

// Gt 添加大于条件。
func (w *UpdateWrapper[T]) Gt(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionGt, value)
}

// GtTyped 添加类型化字段引用的大于条件。
func (w *UpdateWrapper[T]) GtTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Gt(field.Field(), value)
}

// Ge 添加大于等于条件。
func (w *UpdateWrapper[T]) Ge(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionGe, value)
}

// GeTyped 添加类型化字段引用的大于等于条件。
func (w *UpdateWrapper[T]) GeTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Ge(field.Field(), value)
}

// Lt 添加小于条件。
func (w *UpdateWrapper[T]) Lt(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionLt, value)
}

// LtTyped 添加类型化字段引用的小于条件。
func (w *UpdateWrapper[T]) LtTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Lt(field.Field(), value)
}

// Le 添加小于等于条件。
func (w *UpdateWrapper[T]) Le(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionLe, value)
}

// LeTyped 添加类型化字段引用的小于等于条件。
func (w *UpdateWrapper[T]) LeTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Le(field.Field(), value)
}

// Like 添加 LIKE 条件，通配符由调用方显式传入。
func (w *UpdateWrapper[T]) Like(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionLike, value)
}

// NotLike 添加 NOT LIKE 条件。
func (w *UpdateWrapper[T]) NotLike(field Field[T], value any) *UpdateWrapper[T] {
	return w.add(field, conditionNotLike, value)
}

// LikeLeft 添加左侧模糊匹配条件。
func (w *UpdateWrapper[T]) LikeLeft(field Field[T], value any) *UpdateWrapper[T] {
	return w.Like(field, "%"+fmt.Sprint(value))
}

// LikeRight 添加右侧模糊匹配条件。
func (w *UpdateWrapper[T]) LikeRight(field Field[T], value any) *UpdateWrapper[T] {
	return w.Like(field, fmt.Sprint(value)+"%")
}

// LikeTyped 添加类型化字段引用的 LIKE 条件。
func (w *UpdateWrapper[T]) LikeTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.Like(field.Field(), value)
}

// NotLikeTyped 添加类型化字段引用的 NOT LIKE 条件。
func (w *UpdateWrapper[T]) NotLikeTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.NotLike(field.Field(), value)
}

// LikeLeftTyped 添加类型化字段引用的左侧模糊匹配条件。
func (w *UpdateWrapper[T]) LikeLeftTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.LikeLeft(field.Field(), value)
}

// LikeRightTyped 添加类型化字段引用的右侧模糊匹配条件。
func (w *UpdateWrapper[T]) LikeRightTyped(field TypedFieldRef[T], value any) *UpdateWrapper[T] {
	return w.LikeRight(field.Field(), value)
}

// In 添加 IN 条件。空集合会渲染为 1 = 0，避免意外全量命中。
func (w *UpdateWrapper[T]) In(field Field[T], values any) *UpdateWrapper[T] {
	return w.add(field, conditionIn, values)
}

// NotIn 添加 NOT IN 条件。空集合会渲染为 1 = 0，避免写操作意外放大。
func (w *UpdateWrapper[T]) NotIn(field Field[T], values any) *UpdateWrapper[T] {
	return w.add(field, conditionNotIn, values)
}

// InTyped 添加类型化字段引用的 IN 条件。
func (w *UpdateWrapper[T]) InTyped(field TypedFieldRef[T], values any) *UpdateWrapper[T] {
	return w.In(field.Field(), values)
}

// NotInTyped 添加类型化字段引用的 NOT IN 条件。
func (w *UpdateWrapper[T]) NotInTyped(field TypedFieldRef[T], values any) *UpdateWrapper[T] {
	return w.NotIn(field.Field(), values)
}

// Between 添加 BETWEEN 条件。
func (w *UpdateWrapper[T]) Between(field Field[T], left any, right any) *UpdateWrapper[T] {
	return w.add(field, conditionBetween, betweenValues{left: left, right: right})
}

// NotBetween 添加 NOT BETWEEN 条件。
func (w *UpdateWrapper[T]) NotBetween(field Field[T], left any, right any) *UpdateWrapper[T] {
	return w.add(field, conditionNotBetween, betweenValues{left: left, right: right})
}

// BetweenTyped 添加类型化字段引用的 BETWEEN 条件。
func (w *UpdateWrapper[T]) BetweenTyped(field TypedFieldRef[T], left any, right any) *UpdateWrapper[T] {
	return w.Between(field.Field(), left, right)
}

// NotBetweenTyped 添加类型化字段引用的 NOT BETWEEN 条件。
func (w *UpdateWrapper[T]) NotBetweenTyped(field TypedFieldRef[T], left any, right any) *UpdateWrapper[T] {
	return w.NotBetween(field.Field(), left, right)
}

// IsNull 添加 IS NULL 条件。
func (w *UpdateWrapper[T]) IsNull(field Field[T]) *UpdateWrapper[T] {
	return w.add(field, conditionIsNull, nil)
}

// IsNullTyped 添加类型化字段引用的 IS NULL 条件。
func (w *UpdateWrapper[T]) IsNullTyped(field TypedFieldRef[T]) *UpdateWrapper[T] {
	return w.IsNull(field.Field())
}

// IsNotNull 添加 IS NOT NULL 条件。
func (w *UpdateWrapper[T]) IsNotNull(field Field[T]) *UpdateWrapper[T] {
	return w.add(field, conditionIsNotNull, nil)
}

// IsNotNullTyped 添加类型化字段引用的 IS NOT NULL 条件。
func (w *UpdateWrapper[T]) IsNotNullTyped(field TypedFieldRef[T]) *UpdateWrapper[T] {
	return w.IsNotNull(field.Field())
}

// Nested 添加默认 AND 连接的嵌套条件组。
func (w *UpdateWrapper[T]) Nested(apply func(*UpdateWrapper[T])) *UpdateWrapper[T] {
	return w.addNested(conditionConnectorAnd, apply)
}

// And 添加 AND 连接的嵌套条件组。
func (w *UpdateWrapper[T]) And(apply func(*UpdateWrapper[T])) *UpdateWrapper[T] {
	return w.addNested(conditionConnectorAnd, apply)
}

// Or 添加 OR 连接的嵌套条件组。
func (w *UpdateWrapper[T]) Or(apply func(*UpdateWrapper[T])) *UpdateWrapper[T] {
	return w.addNested(conditionConnectorOr, apply)
}

// Exists 添加 EXISTS 子查询条件，SQL 仅允许 #{name} 参数占位符。
func (w *UpdateWrapper[T]) Exists(sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addRaw(conditionConnectorAnd, queryConditionExists, sqlText, args)
}

// NotExists 添加 NOT EXISTS 子查询条件，SQL 仅允许 #{name} 参数占位符。
func (w *UpdateWrapper[T]) NotExists(sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addRaw(conditionConnectorAnd, queryConditionNotExists, sqlText, args)
}

// Apply 添加自定义条件片段，SQL 仅允许 #{name} 参数占位符。
func (w *UpdateWrapper[T]) Apply(sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addRaw(conditionConnectorAnd, queryConditionRaw, sqlText, args)
}

// Last 添加 SQL 尾部片段，例如 RETURNING id。尾部不支持参数绑定。
func (w *UpdateWrapper[T]) Last(sqlText string) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	w.lastSQL = strings.TrimSpace(sqlText)
	return w
}

func (w *UpdateWrapper[T]) add(field Field[T], op conditionOperator, value any) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	w.conditions = append(w.conditions, queryCondition[T]{Connector: conditionConnectorAnd, Field: field, Op: op, Value: value})
	return w
}

func (w *UpdateWrapper[T]) addNested(connector conditionConnector, apply func(*UpdateWrapper[T])) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	if apply == nil {
		return w
	}
	child := NewUpdateWrapper[T]()
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

func (w *UpdateWrapper[T]) addRaw(connector conditionConnector, kind queryConditionKind, sqlText string, args NamedArgs) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	w.conditions = append(w.conditions, queryCondition[T]{
		Connector: connector,
		Kind:      kind,
		SQL:       sqlText,
		Args:      copyNamedArgs(args),
	})
	return w
}

func (w *UpdateWrapper[T]) addSetArithmetic(field Field[T], value any, op updateAssignmentOperator) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	w.assignments = append(w.assignments, updateAssignment[T]{Field: field, Value: value, Op: op})
	return w
}

func (w *UpdateWrapper[T]) build(dialect Dialect, start int) (updateWrapperSQL, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	if w == nil {
		return updateWrapperSQL{Args: NamedArgs{}, Next: start}, nil
	}
	args := make(NamedArgs)
	seq := start
	sets := make([]string, 0, len(w.assignments))
	for _, assignment := range w.assignments {
		rendered, next, err := renderUpdateAssignment(dialect, assignment, seq, args)
		if err != nil {
			return updateWrapperSQL{}, err
		}
		seq = next
		if rendered != "" {
			sets = append(sets, rendered)
		}
	}
	if len(sets) == 0 {
		return updateWrapperSQL{}, fmt.Errorf("goark-orm: update wrapper must contain rendered set columns")
	}
	conditions, whereArgs, next, err := renderQueryConditions(dialect, w.conditions, seq)
	if err != nil {
		return updateWrapperSQL{}, err
	}
	for key, value := range whereArgs {
		args[key] = value
	}
	lastSQL, err := sanitizeLastSQL(w.lastSQL)
	if err != nil {
		return updateWrapperSQL{}, err
	}
	return updateWrapperSQL{
		SetSQL:   strings.Join(sets, ", "),
		WhereSQL: strings.Join(conditions, " "),
		LastSQL:  lastSQL,
		Args:     args,
		Next:     next,
	}, nil
}

func renderUpdateAssignment[T any](dialect Dialect, assignment updateAssignment[T], seq int, args NamedArgs) (string, int, error) {
	switch assignment.Op {
	case updateAssignmentRaw:
		return renderRawSQLFragment(assignment.SQL, assignment.Args, seq, args)
	case updateAssignmentSet:
		column, err := quoteIdentifierPath(dialect, assignment.Field.Column)
		if err != nil {
			return "", seq, err
		}
		name := updateWrapperArgName(seq)
		seq++
		args[name] = assignment.Value
		return column + " = #{" + name + "}", seq, nil
	case updateAssignmentIncrBy, updateAssignmentDecrBy:
		column, err := quoteIdentifierPath(dialect, assignment.Field.Column)
		if err != nil {
			return "", seq, err
		}
		name := updateWrapperArgName(seq)
		seq++
		args[name] = assignment.Value
		operator := "+"
		if assignment.Op == updateAssignmentDecrBy {
			operator = "-"
		}
		return column + " = " + column + " " + operator + " #{" + name + "}", seq, nil
	default:
		return "", seq, fmt.Errorf("goark-orm: unsupported update assignment %q", assignment.Op)
	}
}

func updateWrapperArgName(seq int) string {
	return "__goark_orm_u_" + strconv.Itoa(seq)
}

func requireUpdateWrapper[T any](wrapper *UpdateWrapper[T]) error {
	if wrapper == nil {
		return fmt.Errorf("goark-orm: update wrapper is nil")
	}
	if wrapper.SetEmpty() {
		return fmt.Errorf("goark-orm: update wrapper must contain set columns")
	}
	if wrapper.Empty() {
		return fmt.Errorf("goark-orm: update wrapper must contain conditions")
	}
	return nil
}
