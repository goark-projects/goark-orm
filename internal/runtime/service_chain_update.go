package runtime

// Eq 添加等值条件。
func (c *UpdateChain[T, ID]) Eq(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Eq(field, value)
	}
	return c
}

// EqIf 在 condition 为 true 时添加等值条件。
func (c *UpdateChain[T, ID]) EqIf(condition bool, field Field[T], value any) *UpdateChain[T, ID] {
	if condition {
		c.Eq(field, value)
	}
	return c
}

// EqTyped 添加类型化字段引用的等值条件。
func (c *UpdateChain[T, ID]) EqTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Eq(field.Field(), value)
}

// Ne 添加不等条件。
func (c *UpdateChain[T, ID]) Ne(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Ne(field, value)
	}
	return c
}

// NeTyped 添加类型化字段引用的不等条件。
func (c *UpdateChain[T, ID]) NeTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Ne(field.Field(), value)
}

// Gt 添加大于条件。
func (c *UpdateChain[T, ID]) Gt(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Gt(field, value)
	}
	return c
}

// GtTyped 添加类型化字段引用的大于条件。
func (c *UpdateChain[T, ID]) GtTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Gt(field.Field(), value)
}

// Ge 添加大于等于条件。
func (c *UpdateChain[T, ID]) Ge(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Ge(field, value)
	}
	return c
}

// GeTyped 添加类型化字段引用的大于等于条件。
func (c *UpdateChain[T, ID]) GeTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Ge(field.Field(), value)
}

// Lt 添加小于条件。
func (c *UpdateChain[T, ID]) Lt(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Lt(field, value)
	}
	return c
}

// LtTyped 添加类型化字段引用的小于条件。
func (c *UpdateChain[T, ID]) LtTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Lt(field.Field(), value)
}

// Le 添加小于等于条件。
func (c *UpdateChain[T, ID]) Le(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Le(field, value)
	}
	return c
}

// LeTyped 添加类型化字段引用的小于等于条件。
func (c *UpdateChain[T, ID]) LeTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Le(field.Field(), value)
}

// Like 添加 LIKE 条件。
func (c *UpdateChain[T, ID]) Like(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Like(field, value)
	}
	return c
}

// NotLike 添加 NOT LIKE 条件。
func (c *UpdateChain[T, ID]) NotLike(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.NotLike(field, value)
	}
	return c
}

// LikeLeft 添加左侧模糊匹配条件。
func (c *UpdateChain[T, ID]) LikeLeft(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.LikeLeft(field, value)
	}
	return c
}

// LikeRight 添加右侧模糊匹配条件。
func (c *UpdateChain[T, ID]) LikeRight(field Field[T], value any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.LikeRight(field, value)
	}
	return c
}

// LikeTyped 添加类型化字段引用的 LIKE 条件。
func (c *UpdateChain[T, ID]) LikeTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.Like(field.Field(), value)
}

// NotLikeTyped 添加类型化字段引用的 NOT LIKE 条件。
func (c *UpdateChain[T, ID]) NotLikeTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.NotLike(field.Field(), value)
}

// LikeLeftTyped 添加类型化字段引用的左侧模糊匹配条件。
func (c *UpdateChain[T, ID]) LikeLeftTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.LikeLeft(field.Field(), value)
}

// LikeRightTyped 添加类型化字段引用的右侧模糊匹配条件。
func (c *UpdateChain[T, ID]) LikeRightTyped(field TypedFieldRef[T], value any) *UpdateChain[T, ID] {
	return c.LikeRight(field.Field(), value)
}

// In 添加 IN 条件。
func (c *UpdateChain[T, ID]) In(field Field[T], values any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.In(field, values)
	}
	return c
}

// NotIn 添加 NOT IN 条件。
func (c *UpdateChain[T, ID]) NotIn(field Field[T], values any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.NotIn(field, values)
	}
	return c
}

// InTyped 添加类型化字段引用的 IN 条件。
func (c *UpdateChain[T, ID]) InTyped(field TypedFieldRef[T], values any) *UpdateChain[T, ID] {
	return c.In(field.Field(), values)
}

// NotInTyped 添加类型化字段引用的 NOT IN 条件。
func (c *UpdateChain[T, ID]) NotInTyped(field TypedFieldRef[T], values any) *UpdateChain[T, ID] {
	return c.NotIn(field.Field(), values)
}

// Between 添加 BETWEEN 条件。
func (c *UpdateChain[T, ID]) Between(field Field[T], left any, right any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.Between(field, left, right)
	}
	return c
}

// NotBetween 添加 NOT BETWEEN 条件。
func (c *UpdateChain[T, ID]) NotBetween(field Field[T], left any, right any) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.NotBetween(field, left, right)
	}
	return c
}

// BetweenTyped 添加类型化字段引用的 BETWEEN 条件。
func (c *UpdateChain[T, ID]) BetweenTyped(field TypedFieldRef[T], left any, right any) *UpdateChain[T, ID] {
	return c.Between(field.Field(), left, right)
}

// NotBetweenTyped 添加类型化字段引用的 NOT BETWEEN 条件。
func (c *UpdateChain[T, ID]) NotBetweenTyped(field TypedFieldRef[T], left any, right any) *UpdateChain[T, ID] {
	return c.NotBetween(field.Field(), left, right)
}

// IsNull 添加 IS NULL 条件。
func (c *UpdateChain[T, ID]) IsNull(field Field[T]) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.IsNull(field)
	}
	return c
}

// IsNullTyped 添加类型化字段引用的 IS NULL 条件。
func (c *UpdateChain[T, ID]) IsNullTyped(field TypedFieldRef[T]) *UpdateChain[T, ID] {
	return c.IsNull(field.Field())
}

// IsNotNull 添加 IS NOT NULL 条件。
func (c *UpdateChain[T, ID]) IsNotNull(field Field[T]) *UpdateChain[T, ID] {
	if wrapper := c.updateWrapper(); wrapper != nil {
		wrapper.IsNotNull(field)
	}
	return c
}

// IsNotNullTyped 添加类型化字段引用的 IS NOT NULL 条件。
func (c *UpdateChain[T, ID]) IsNotNullTyped(field TypedFieldRef[T]) *UpdateChain[T, ID] {
	return c.IsNotNull(field.Field())
}
