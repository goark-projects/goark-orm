package runtime

import (
	"reflect"
	"strings"
)

type sqlBuilderConditionKind int

const (
	sqlBuilderConditionBinary sqlBuilderConditionKind = iota
	sqlBuilderConditionRaw
	sqlBuilderConditionIn
	sqlBuilderConditionBetween
	sqlBuilderConditionUnary
)

type sqlBuilderCondition struct {
	kind   sqlBuilderConditionKind
	column string
	op     string
	value  any
	second any
	values []any
	sql    string
	args   NamedArgs
}

// WhereIn 添加 IN 条件，values 可以是可变参数或单个 slice/array。
func (b *SelectSQLBuilder) WhereIn(column string, values ...any) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionIn, column: column, op: "IN", values: sqlBuilderValues(values)})
	return b
}

// WhereBetween 添加 BETWEEN 条件。
func (b *SelectSQLBuilder) WhereBetween(column string, begin any, end any) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionBetween, column: column, value: begin, second: end})
	return b
}

// WhereIsNull 添加 IS NULL 条件。
func (b *SelectSQLBuilder) WhereIsNull(column string) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionUnary, column: column, op: "IS NULL"})
	return b
}

// WhereIsNotNull 添加 IS NOT NULL 条件。
func (b *SelectSQLBuilder) WhereIsNotNull(column string) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionUnary, column: column, op: "IS NOT NULL"})
	return b
}

// WhereIn 添加 IN 条件，values 可以是可变参数或单个 slice/array。
func (b *UpdateSQLBuilder) WhereIn(column string, values ...any) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionIn, column: column, op: "IN", values: sqlBuilderValues(values)})
	return b
}

// WhereBetween 添加 BETWEEN 条件。
func (b *UpdateSQLBuilder) WhereBetween(column string, begin any, end any) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionBetween, column: column, value: begin, second: end})
	return b
}

// WhereIsNull 添加 IS NULL 条件。
func (b *UpdateSQLBuilder) WhereIsNull(column string) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionUnary, column: column, op: "IS NULL"})
	return b
}

// WhereIsNotNull 添加 IS NOT NULL 条件。
func (b *UpdateSQLBuilder) WhereIsNotNull(column string) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionUnary, column: column, op: "IS NOT NULL"})
	return b
}

// WhereIn 添加 IN 条件，values 可以是可变参数或单个 slice/array。
func (b *DeleteSQLBuilder) WhereIn(column string, values ...any) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionIn, column: column, op: "IN", values: sqlBuilderValues(values)})
	return b
}

// WhereBetween 添加 BETWEEN 条件。
func (b *DeleteSQLBuilder) WhereBetween(column string, begin any, end any) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionBetween, column: column, value: begin, second: end})
	return b
}

// WhereIsNull 添加 IS NULL 条件。
func (b *DeleteSQLBuilder) WhereIsNull(column string) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionUnary, column: column, op: "IS NULL"})
	return b
}

// WhereIsNotNull 添加 IS NOT NULL 条件。
func (b *DeleteSQLBuilder) WhereIsNotNull(column string) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.conditions = append(b.conditions, sqlBuilderCondition{kind: sqlBuilderConditionUnary, column: column, op: "IS NOT NULL"})
	return b
}

func (s *sqlBuilderState) conditions(conditions []sqlBuilderCondition) (string, error) {
	if len(conditions) == 0 {
		return "", nil
	}
	rendered := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		sqlText, err := s.condition(condition)
		if err != nil {
			return "", err
		}
		if sqlText != "" {
			rendered = append(rendered, sqlText)
		}
	}
	return strings.Join(rendered, " AND "), nil
}

func (s *sqlBuilderState) condition(condition sqlBuilderCondition) (string, error) {
	if condition.kind == sqlBuilderConditionRaw || strings.TrimSpace(condition.sql) != "" {
		rendered, next, err := renderRawSQLFragment(condition.sql, condition.args, s.seq, s.args)
		if err != nil {
			return "", err
		}
		s.seq = next
		return rendered, nil
	}
	column, err := s.identifier(condition.column)
	if err != nil {
		return "", err
	}
	switch condition.kind {
	case sqlBuilderConditionIn:
		if len(condition.values) == 0 {
			return "", bindingErrorf("SQL builder IN condition for column %s requires at least one value", condition.column)
		}
		values := make([]string, 0, len(condition.values))
		for _, value := range condition.values {
			values = append(values, s.value(value))
		}
		return column + " " + firstNonEmpty(condition.op, "IN") + " (" + strings.Join(values, ", ") + ")", nil
	case sqlBuilderConditionBetween:
		return column + " BETWEEN " + s.value(condition.value) + " AND " + s.value(condition.second), nil
	case sqlBuilderConditionUnary:
		op := strings.TrimSpace(condition.op)
		if op == "" {
			return "", bindingErrorf("SQL builder unary condition for column %s requires operator", condition.column)
		}
		return column + " " + op, nil
	}
	op := strings.TrimSpace(condition.op)
	if op == "" {
		op = "="
	}
	return column + " " + op + " " + s.value(condition.value), nil
}

func sqlBuilderValues(values []any) []any {
	if len(values) != 1 {
		return append([]any(nil), values...)
	}
	value := values[0]
	if value == nil {
		return []any{nil}
	}
	reflectValue := reflect.ValueOf(value)
	if !reflectValue.IsValid() {
		return []any{value}
	}
	if reflectValue.Kind() != reflect.Slice && reflectValue.Kind() != reflect.Array {
		return []any{value}
	}
	if reflectValue.Kind() == reflect.Slice && reflectValue.Type().Elem().Kind() == reflect.Uint8 {
		return []any{value}
	}
	out := make([]any, 0, reflectValue.Len())
	for index := 0; index < reflectValue.Len(); index++ {
		out = append(out, reflectValue.Index(index).Interface())
	}
	return out
}
