package runtime

// When 在 condition 为 true 时应用链式构造逻辑。
func (c *QueryChain[T, ID]) When(condition bool, apply func(*QueryChain[T, ID])) *QueryChain[T, ID] {
	if condition && apply != nil {
		apply(c)
	}
	return c
}

// Select 指定查询投影字段。
func (c *QueryChain[T, ID]) Select(fields ...Field[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.Select(fields...)
	}
	return c
}

// SelectTyped 指定类型化字段查询投影。
func (c *QueryChain[T, ID]) SelectTyped(fields ...TypedFieldRef[T]) *QueryChain[T, ID] {
	if wrapper := c.queryWrapper(); wrapper != nil {
		wrapper.SelectTyped(fields...)
	}
	return c
}
