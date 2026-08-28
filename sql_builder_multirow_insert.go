package orm

import (
	"fmt"
	"strings"
)

// MultiRowInsertSQLBuilder 构造跨方言的多行 INSERT SQLSource。
type MultiRowInsertSQLBuilder struct {
	table    string
	columns  []string
	rows     []NamedArgs
	cacheKey string
}

// NewMultiRowInsertSQLBuilder 创建多行 INSERT SQL 构造器。
func NewMultiRowInsertSQLBuilder() *MultiRowInsertSQLBuilder {
	return &MultiRowInsertSQLBuilder{}
}

// Into 指定 INSERT 目标表。
func (b *MultiRowInsertSQLBuilder) Into(table string) *MultiRowInsertSQLBuilder {
	if b == nil {
		b = NewMultiRowInsertSQLBuilder()
	}
	b.table = table
	return b
}

// Columns 指定多行 INSERT 的列顺序。
func (b *MultiRowInsertSQLBuilder) Columns(columns ...string) *MultiRowInsertSQLBuilder {
	if b == nil {
		b = NewMultiRowInsertSQLBuilder()
	}
	b.columns = append(b.columns, columns...)
	return b
}

// Row 追加一行值，键名必须覆盖 Columns 指定的全部列。
func (b *MultiRowInsertSQLBuilder) Row(values NamedArgs) *MultiRowInsertSQLBuilder {
	if b == nil {
		b = NewMultiRowInsertSQLBuilder()
	}
	b.rows = append(b.rows, copyNamedArgs(values))
	return b
}

// Rows 追加多行值，键名必须覆盖 Columns 指定的全部列。
func (b *MultiRowInsertSQLBuilder) Rows(rows ...NamedArgs) *MultiRowInsertSQLBuilder {
	if b == nil {
		b = NewMultiRowInsertSQLBuilder()
	}
	for _, row := range rows {
		b.rows = append(b.rows, copyNamedArgs(row))
	}
	return b
}

// CacheKey 指定 Provider 额外缓存维度。
func (b *MultiRowInsertSQLBuilder) CacheKey(cacheKey string) *MultiRowInsertSQLBuilder {
	if b == nil {
		b = NewMultiRowInsertSQLBuilder()
	}
	b.cacheKey = cacheKey
	return b
}

// Build 按方言构造多行 INSERT SQLSource。
func (b *MultiRowInsertSQLBuilder) Build(dialect Dialect) (SQLSource, error) {
	if b == nil {
		return SQLSource{}, bindingErrorf("multi-row insert SQL builder is nil")
	}
	if len(b.columns) == 0 {
		return SQLSource{}, bindingErrorf("multi-row insert SQL builder requires at least one column")
	}
	if len(b.rows) == 0 {
		return SQLSource{}, bindingErrorf("multi-row insert SQL builder requires at least one row")
	}
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	state := newSQLBuilderState()
	table, err := state.identifier(b.table)
	if err != nil {
		return SQLSource{}, err
	}
	columns, err := state.identifierList(b.columns, "")
	if err != nil {
		return SQLSource{}, err
	}
	if DialectCapabilitiesOf(dialect).DBType == DbTypeOracle {
		return b.buildOracleInsertAll(state, table, columns)
	}
	values, err := b.rowValueGroups(state)
	if err != nil {
		return SQLSource{}, err
	}
	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, strings.Join(columns, ", "), strings.Join(values, ", "))
	return state.source(sqlText, b.cacheKey), nil
}

func (b *MultiRowInsertSQLBuilder) buildOracleInsertAll(state *sqlBuilderState, table string, columns []string) (SQLSource, error) {
	parts := make([]string, 0, len(b.rows)+2)
	parts = append(parts, "INSERT ALL")
	for index, row := range b.rows {
		values, err := b.rowValues(state, index, row)
		if err != nil {
			return SQLSource{}, err
		}
		parts = append(parts, "INTO "+table+" ("+strings.Join(columns, ", ")+") VALUES ("+strings.Join(values, ", ")+")")
	}
	parts = append(parts, "SELECT 1 FROM dual")
	return state.source(strings.Join(parts, " "), b.cacheKey), nil
}

func (b *MultiRowInsertSQLBuilder) rowValueGroups(state *sqlBuilderState) ([]string, error) {
	values := make([]string, 0, len(b.rows))
	for index, row := range b.rows {
		rowValues, err := b.rowValues(state, index, row)
		if err != nil {
			return nil, err
		}
		values = append(values, "("+strings.Join(rowValues, ", ")+")")
	}
	return values, nil
}

func (b *MultiRowInsertSQLBuilder) rowValues(state *sqlBuilderState, rowIndex int, row NamedArgs) ([]string, error) {
	values := make([]string, 0, len(b.columns))
	for _, column := range b.columns {
		key := strings.TrimSpace(column)
		value, ok := row[key]
		if !ok {
			return nil, &BindingError{
				Operation: "build multi-row insert",
				Parameter: key,
				Message:   fmt.Sprintf("multi-row insert row %d value is missing", rowIndex),
			}
		}
		values = append(values, state.value(value))
	}
	return values, nil
}
