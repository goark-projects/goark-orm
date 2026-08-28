package orm

// Nested 添加默认 AND 连接的嵌套条件组。
func (c *QueryChain[T, ID]) Nested(apply func(*QueryWrapper[T])) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.Nested(apply)
	}
	return c
}

// And 添加 AND 连接的嵌套条件组。
func (c *QueryChain[T, ID]) And(apply func(*QueryWrapper[T])) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.And(apply)
	}
	return c
}

// Or 添加 OR 连接的嵌套条件组。
func (c *QueryChain[T, ID]) Or(apply func(*QueryWrapper[T])) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.Or(apply)
	}
	return c
}

// Exists 添加 EXISTS 子查询条件。
func (c *QueryChain[T, ID]) Exists(sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.Exists(sqlText, args)
	}
	return c
}

// NotExists 添加 NOT EXISTS 子查询条件。
func (c *QueryChain[T, ID]) NotExists(sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.NotExists(sqlText, args)
	}
	return c
}

// Apply 添加自定义条件片段。
func (c *QueryChain[T, ID]) Apply(sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.Apply(sqlText, args)
	}
	return c
}

// EqSQL 添加等值 SQL 子查询条件。
func (c *QueryChain[T, ID]) EqSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.EqSQL(field, sqlText, args)
	}
	return c
}

// NeSQL 添加不等 SQL 子查询条件。
func (c *QueryChain[T, ID]) NeSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.NeSQL(field, sqlText, args)
	}
	return c
}

// GtSQL 添加大于 SQL 子查询条件。
func (c *QueryChain[T, ID]) GtSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.GtSQL(field, sqlText, args)
	}
	return c
}

// GeSQL 添加大于等于 SQL 子查询条件。
func (c *QueryChain[T, ID]) GeSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.GeSQL(field, sqlText, args)
	}
	return c
}

// LtSQL 添加小于 SQL 子查询条件。
func (c *QueryChain[T, ID]) LtSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.LtSQL(field, sqlText, args)
	}
	return c
}

// LeSQL 添加小于等于 SQL 子查询条件。
func (c *QueryChain[T, ID]) LeSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.LeSQL(field, sqlText, args)
	}
	return c
}

// InSQL 添加 IN SQL 片段条件。
func (c *QueryChain[T, ID]) InSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.InSQL(field, sqlText, args)
	}
	return c
}

// NotInSQL 添加 NOT IN SQL 片段条件。
func (c *QueryChain[T, ID]) NotInSQL(field Field[T], sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.NotInSQL(field, sqlText, args)
	}
	return c
}

// GroupBy 添加 GROUP BY 字段。
func (c *QueryChain[T, ID]) GroupBy(fields ...Field[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.GroupBy(fields...)
	}
	return c
}

// GroupByTyped 添加类型化字段引用的 GROUP BY 字段。
func (c *QueryChain[T, ID]) GroupByTyped(fields ...TypedFieldRef[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.GroupByTyped(fields...)
	}
	return c
}

// Having 添加 HAVING 条件片段。
func (c *QueryChain[T, ID]) Having(sqlText string, args NamedArgs) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.Having(sqlText, args)
	}
	return c
}

// OrderByAsc 添加升序排序。
func (c *QueryChain[T, ID]) OrderByAsc(field Field[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.OrderByAsc(field)
	}
	return c
}

// OrderByAscTyped 添加类型化字段引用的升序排序。
func (c *QueryChain[T, ID]) OrderByAscTyped(field TypedFieldRef[T]) *QueryChain[T, ID] {
	return c.OrderByAsc(field.Field())
}

// OrderByDesc 添加降序排序。
func (c *QueryChain[T, ID]) OrderByDesc(field Field[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.OrderByDesc(field)
	}
	return c
}

// OrderByDescTyped 添加类型化字段引用的降序排序。
func (c *QueryChain[T, ID]) OrderByDescTyped(field TypedFieldRef[T]) *QueryChain[T, ID] {
	return c.OrderByDesc(field.Field())
}

// OrderBy 按条件和方向添加排序。
func (c *QueryChain[T, ID]) OrderBy(condition bool, asc bool, fields ...Field[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.OrderBy(condition, asc, fields...)
	}
	return c
}

// OrderByTyped 按条件和方向添加类型化字段排序。
func (c *QueryChain[T, ID]) OrderByTyped(condition bool, asc bool, fields ...TypedFieldRef[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.OrderByTyped(condition, asc, fields...)
	}
	return c
}

// Last 添加 SQL 尾部片段。
func (c *QueryChain[T, ID]) Last(sqlText string) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.Last(sqlText)
	}
	return c
}
