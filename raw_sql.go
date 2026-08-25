package orm

import (
	"strings"
)

// RawSQLToken 表示允许进入 ${} 的受控 SQL 片段。
//
// 接口包含非导出方法，外部包只能通过本包构造器创建安全 token。
type RawSQLToken interface {
	renderRawSQL(dialect Dialect) (string, error)
	rawSQLToken()
}

// RawIdentifier 表示表名、列名或 schema.table 形式的标识符。
type RawIdentifier struct {
	name string
}

// NewRawIdentifier 创建受控 SQL 标识符 token。
func NewRawIdentifier(name string) (RawIdentifier, error) {
	name = strings.TrimSpace(name)
	if _, err := quoteIdentifierPath(NewQuestionDialect(), name); err != nil {
		return RawIdentifier{}, err
	}
	return RawIdentifier{name: name}, nil
}

func (i RawIdentifier) renderRawSQL(dialect Dialect) (string, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	return quoteIdentifierPath(dialect, i.name)
}

func (RawIdentifier) rawSQLToken() {}

// RawOrderItem 表示 ORDER BY 中的一个受控字段。
type RawOrderItem struct {
	Column RawIdentifier
	Desc   bool
}

// NewRawOrderItem 创建受控排序字段。
func NewRawOrderItem(column string, desc bool) (RawOrderItem, error) {
	identifier, err := NewRawIdentifier(column)
	if err != nil {
		return RawOrderItem{}, err
	}
	return RawOrderItem{Column: identifier, Desc: desc}, nil
}

// RawOrderBy 表示 ORDER BY 子句后的受控字段列表。
type RawOrderBy struct {
	items []RawOrderItem
}

// NewRawOrderBy 创建受控 ORDER BY token。
func NewRawOrderBy(items ...RawOrderItem) RawOrderBy {
	copied := append([]RawOrderItem(nil), items...)
	return RawOrderBy{items: copied}
}

func (o RawOrderBy) renderRawSQL(dialect Dialect) (string, error) {
	if len(o.items) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(o.items))
	for _, item := range o.items {
		column, err := item.Column.renderRawSQL(dialect)
		if err != nil {
			return "", err
		}
		if item.Desc {
			column += " DESC"
		} else {
			column += " ASC"
		}
		parts = append(parts, column)
	}
	return strings.Join(parts, ", "), nil
}

func (RawOrderBy) rawSQLToken() {}
