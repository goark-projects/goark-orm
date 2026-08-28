package runtime

import (
	"fmt"
	"strings"
)

// EqSQL 添加等值 SQL 子查询条件。
func (w *QueryWrapper[T]) EqSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionEq, sqlText, args)
}

// NeSQL 添加不等 SQL 子查询条件。
func (w *QueryWrapper[T]) NeSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionNe, sqlText, args)
}

// GtSQL 添加大于 SQL 子查询条件。
func (w *QueryWrapper[T]) GtSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionGt, sqlText, args)
}

// GeSQL 添加大于等于 SQL 子查询条件。
func (w *QueryWrapper[T]) GeSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionGe, sqlText, args)
}

// LtSQL 添加小于 SQL 子查询条件。
func (w *QueryWrapper[T]) LtSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionLt, sqlText, args)
}

// LeSQL 添加小于等于 SQL 子查询条件。
func (w *QueryWrapper[T]) LeSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionLe, sqlText, args)
}

// InSQL 添加 IN SQL 片段条件。
func (w *QueryWrapper[T]) InSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionIn, sqlText, args)
}

// NotInSQL 添加 NOT IN SQL 片段条件。
func (w *QueryWrapper[T]) NotInSQL(field Field[T], sqlText string, args NamedArgs) *QueryWrapper[T] {
	return w.addSQLCondition(field, conditionNotIn, sqlText, args)
}

// EqSQL 添加等值 SQL 子查询条件。
func (w *UpdateWrapper[T]) EqSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionEq, sqlText, args)
}

// NeSQL 添加不等 SQL 子查询条件。
func (w *UpdateWrapper[T]) NeSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionNe, sqlText, args)
}

// GtSQL 添加大于 SQL 子查询条件。
func (w *UpdateWrapper[T]) GtSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionGt, sqlText, args)
}

// GeSQL 添加大于等于 SQL 子查询条件。
func (w *UpdateWrapper[T]) GeSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionGe, sqlText, args)
}

// LtSQL 添加小于 SQL 子查询条件。
func (w *UpdateWrapper[T]) LtSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionLt, sqlText, args)
}

// LeSQL 添加小于等于 SQL 子查询条件。
func (w *UpdateWrapper[T]) LeSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionLe, sqlText, args)
}

// InSQL 添加 IN SQL 片段条件。
func (w *UpdateWrapper[T]) InSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionIn, sqlText, args)
}

// NotInSQL 添加 NOT IN SQL 片段条件。
func (w *UpdateWrapper[T]) NotInSQL(field Field[T], sqlText string, args NamedArgs) *UpdateWrapper[T] {
	return w.addSQLCondition(field, conditionNotIn, sqlText, args)
}

func (w *QueryWrapper[T]) addSQLCondition(field Field[T], op conditionOperator, sqlText string, args NamedArgs) *QueryWrapper[T] {
	if w == nil {
		w = NewQueryWrapper[T]()
	}
	w.conditions = append(w.conditions, queryCondition[T]{
		Connector: conditionConnectorAnd,
		Kind:      queryConditionSQL,
		Field:     field,
		Op:        op,
		SQL:       sqlText,
		Args:      copyNamedArgs(args),
	})
	return w
}

func (w *UpdateWrapper[T]) addSQLCondition(field Field[T], op conditionOperator, sqlText string, args NamedArgs) *UpdateWrapper[T] {
	if w == nil {
		w = NewUpdateWrapper[T]()
	}
	w.conditions = append(w.conditions, queryCondition[T]{
		Connector: conditionConnectorAnd,
		Kind:      queryConditionSQL,
		Field:     field,
		Op:        op,
		SQL:       sqlText,
		Args:      copyNamedArgs(args),
	})
	return w
}

func renderSQLCondition[T any](dialect Dialect, condition queryCondition[T], seq int, args NamedArgs) (string, int, error) {
	column, err := quoteIdentifierPath(dialect, condition.Field.Column)
	if err != nil {
		return "", seq, err
	}
	fragment, next, err := renderRawSQLFragment(condition.SQL, condition.Args, seq, args)
	if err != nil {
		return "", next, err
	}
	if strings.TrimSpace(fragment) == "" {
		return "", next, fmt.Errorf("goark-orm: SQL condition for column %s is empty", condition.Field.Column)
	}
	op := condition.Op
	switch op {
	case conditionEq, conditionNe, conditionGt, conditionGe, conditionLt, conditionLe:
		return column + " " + string(op) + " (" + fragment + ")", next, nil
	case conditionIn, conditionNotIn:
		return column + " " + string(op) + " (" + fragment + ")", next, nil
	default:
		return "", next, fmt.Errorf("goark-orm: unsupported SQL condition operator %q", op)
	}
}
