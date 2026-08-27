package orm

import "strings"

type sqlBuilderJoin struct {
	kind  string
	table string
	onSQL string
	args  NamedArgs
}

type sqlBuilderRowLock struct {
	dialect Dialect
	options RowLockOptions
}

// Join 添加 INNER JOIN 片段，ON 片段只允许 `#{name}` 参数占位符。
func (b *SelectSQLBuilder) Join(table string, onSQL string, args NamedArgs) *SelectSQLBuilder {
	return b.addJoin("JOIN", table, onSQL, args)
}

// LeftJoin 添加 LEFT JOIN 片段，ON 片段只允许 `#{name}` 参数占位符。
func (b *SelectSQLBuilder) LeftJoin(table string, onSQL string, args NamedArgs) *SelectSQLBuilder {
	return b.addJoin("LEFT JOIN", table, onSQL, args)
}

// RightJoin 添加 RIGHT JOIN 片段，ON 片段只允许 `#{name}` 参数占位符。
func (b *SelectSQLBuilder) RightJoin(table string, onSQL string, args NamedArgs) *SelectSQLBuilder {
	return b.addJoin("RIGHT JOIN", table, onSQL, args)
}

// ForUpdate 根据方言追加 SELECT 行锁子句。
func (b *SelectSQLBuilder) ForUpdate(dialect Dialect, options RowLockOptions) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.rowLock = &sqlBuilderRowLock{dialect: dialect, options: options}
	return b
}

func (b *SelectSQLBuilder) addJoin(kind string, table string, onSQL string, args NamedArgs) *SelectSQLBuilder {
	if b == nil {
		b = NewSelectSQLBuilder()
	}
	b.joins = append(b.joins, sqlBuilderJoin{kind: kind, table: table, onSQL: onSQL, args: copyNamedArgs(args)})
	return b
}

// Returning 添加 INSERT RETURNING 回读列。
func (b *InsertSQLBuilder) Returning(columns ...string) *InsertSQLBuilder {
	if b == nil {
		b = NewInsertSQLBuilder()
	}
	b.returnings = append(b.returnings, columns...)
	return b
}

// Returning 添加 UPDATE RETURNING 回读列。
func (b *UpdateSQLBuilder) Returning(columns ...string) *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.returnings = append(b.returnings, columns...)
	return b
}

// RequireWhere 要求 UPDATE 必须生成非空 WHERE 条件。
func (b *UpdateSQLBuilder) RequireWhere() *UpdateSQLBuilder {
	if b == nil {
		b = NewUpdateSQLBuilder()
	}
	b.requireWhere = true
	return b
}

// Returning 添加 DELETE RETURNING 回读列。
func (b *DeleteSQLBuilder) Returning(columns ...string) *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.returnings = append(b.returnings, columns...)
	return b
}

// RequireWhere 要求 DELETE 必须生成非空 WHERE 条件。
func (b *DeleteSQLBuilder) RequireWhere() *DeleteSQLBuilder {
	if b == nil {
		b = NewDeleteSQLBuilder()
	}
	b.requireWhere = true
	return b
}

func (s *sqlBuilderState) joinClause(joins []sqlBuilderJoin) (string, error) {
	if len(joins) == 0 {
		return "", nil
	}
	rendered := make([]string, 0, len(joins))
	for _, join := range joins {
		table, err := s.identifier(join.table)
		if err != nil {
			return "", err
		}
		onSQL, next, err := renderRawSQLFragment(join.onSQL, join.args, s.seq, s.args)
		if err != nil {
			return "", err
		}
		s.seq = next
		if onSQL == "" {
			return "", bindingErrorf("%s on table %s requires ON SQL", strings.TrimSpace(join.kind), join.table)
		}
		kind := strings.TrimSpace(join.kind)
		if kind == "" {
			kind = "JOIN"
		}
		rendered = append(rendered, kind+" "+table+" ON "+onSQL)
	}
	return strings.Join(rendered, " "), nil
}
