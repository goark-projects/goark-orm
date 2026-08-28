package orm

// When 在 condition 为 true 时应用链式构造逻辑。
func (c *UpdateChain[T, ID]) When(condition bool, apply func(*UpdateChain[T, ID])) *UpdateChain[T, ID] {
	if condition && apply != nil {
		apply(c)
	}
	return c
}

// Set 添加字段赋值。
func (c *UpdateChain[T, ID]) Set(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Set(field, value)
	}
	return c
}

// SetIf 在 condition 为 true 时添加字段赋值。
func (c *UpdateChain[T, ID]) SetIf(condition bool, field Field[T], value any) *UpdateChain[T, ID] {
	if condition {
		c.Set(field, value)
	}
	return c
}

// SetTyped 添加类型化字段引用的字段赋值。
func (c *UpdateChain[T, ID]) SetTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Set(field.Field(), value)
}

// SetTypedIf 在 condition 为 true 时添加类型化字段引用的字段赋值。
func (c *UpdateChain[T, ID]) SetTypedIf(condition bool, field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	if condition {
		c.SetTyped(field, value)
	}
	return c
}

// SetSQL 添加安全的原生 SET 片段。
func (c *UpdateChain[T, ID]) SetSQL(sqlText string, args NamedArgs) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.SetSQL(sqlText, args)
	}
	return c
}

// SetIncrBy 添加字段自增赋值。
func (c *UpdateChain[T, ID]) SetIncrBy(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.SetIncrBy(field, value)
	}
	return c
}

// SetDecrBy 添加字段自减赋值。
func (c *UpdateChain[T, ID]) SetDecrBy(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.SetDecrBy(field, value)
	}
	return c
}

// SetIncrByTyped 添加类型化字段引用的字段自增赋值。
func (c *UpdateChain[T, ID]) SetIncrByTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.SetIncrBy(field.Field(), value)
}

// SetDecrByTyped 添加类型化字段引用的字段自减赋值。
func (c *UpdateChain[T, ID]) SetDecrByTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.SetDecrBy(field.Field(), value)
}
