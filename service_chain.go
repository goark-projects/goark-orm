package orm

import (
	"context"
	"fmt"
)

// ChainQuery 创建链式查询。
func (s *Service[T, ID]) ChainQuery() *QueryChain[T, ID] {
	return &QueryChain[T, ID]{service: s, wrapper: NewQueryWrapper[T]()}
}

// ChainUpdate 创建链式更新。
func (s *Service[T, ID]) ChainUpdate() *UpdateChain[T, ID] {
	return &UpdateChain[T, ID]{service: s, wrapper: NewUpdateWrapper[T]()}
}

// QueryChain 提供实体查询链。
type QueryChain[T any, ID any] struct {
	service *Service[T, ID]
	wrapper *QueryWrapper[T]
}

// Wrapper 返回底层查询 Wrapper。
func (c *QueryChain[T, ID]) Wrapper() *QueryWrapper[T] {
	if c == nil {
		return nil
	}
	return c.wrapper
}

// Where 使用底层 QueryWrapper 添加复杂条件。
func (c *QueryChain[T, ID]) Where(apply func(*QueryWrapper[T])) *QueryChain[T, ID] {
	if c == nil {
		return c
	}
	if c.wrapper == nil {
		c.wrapper = NewQueryWrapper[T]()
	}
	if apply != nil {
		apply(c.wrapper)
	}
	return c
}

// Eq 添加等值条件。
func (c *QueryChain[T, ID]) Eq(field Field[T], value any) *QueryChain[T, ID] {
	return c.Where(func(wrapper *QueryWrapper[T]) { wrapper.Eq(field, value) })
}

// Like 添加 LIKE 条件。
func (c *QueryChain[T, ID]) Like(field Field[T], value any) *QueryChain[T, ID] {
	return c.Where(func(wrapper *QueryWrapper[T]) { wrapper.Like(field, value) })
}

// OrderByAsc 添加升序排序。
func (c *QueryChain[T, ID]) OrderByAsc(field Field[T]) *QueryChain[T, ID] {
	return c.Where(func(wrapper *QueryWrapper[T]) { wrapper.OrderByAsc(field) })
}

// OrderByDesc 添加降序排序。
func (c *QueryChain[T, ID]) OrderByDesc(field Field[T]) *QueryChain[T, ID] {
	return c.Where(func(wrapper *QueryWrapper[T]) { wrapper.OrderByDesc(field) })
}

// List 执行链式列表查询。
func (c *QueryChain[T, ID]) List(ctx context.Context) ([]T, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.List(ctx, c.wrapper)
}

// One 执行链式单条查询。
func (c *QueryChain[T, ID]) One(ctx context.Context) (*T, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.GetOne(ctx, c.wrapper)
}

// First 执行链式单条查询，多行时返回第一行。
func (c *QueryChain[T, ID]) First(ctx context.Context) (*T, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.GetOneOrFirst(ctx, c.wrapper)
}

// Maps 执行链式 map 列表查询。
func (c *QueryChain[T, ID]) Maps(ctx context.Context) ([]map[string]any, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.ListMaps(ctx, c.wrapper)
}

// Map 执行链式单条 map 查询。
func (c *QueryChain[T, ID]) Map(ctx context.Context) (map[string]any, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.GetMap(ctx, c.wrapper)
}

// Objs 执行链式首列列表查询。
func (c *QueryChain[T, ID]) Objs(ctx context.Context) ([]any, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.ListObjs(ctx, c.wrapper)
}

// Obj 执行链式首列单值查询。
func (c *QueryChain[T, ID]) Obj(ctx context.Context) (any, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.GetObj(ctx, c.wrapper)
}

// Count 执行链式计数查询。
func (c *QueryChain[T, ID]) Count(ctx context.Context) (int64, error) {
	if c == nil || c.service == nil {
		return 0, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.Count(ctx, c.wrapper)
}

// Page 执行链式分页查询。
func (c *QueryChain[T, ID]) Page(ctx context.Context, page PageRequest) (Page[T], error) {
	if c == nil || c.service == nil {
		return Page[T]{}, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.Page(ctx, page, c.wrapper)
}

// UpdateChain 提供实体更新链。
type UpdateChain[T any, ID any] struct {
	service *Service[T, ID]
	wrapper *UpdateWrapper[T]
}

// Wrapper 返回底层更新 Wrapper。
func (c *UpdateChain[T, ID]) Wrapper() *UpdateWrapper[T] {
	if c == nil {
		return nil
	}
	return c.wrapper
}

// Where 使用底层 UpdateWrapper 添加复杂条件。
func (c *UpdateChain[T, ID]) Where(apply func(*UpdateWrapper[T])) *UpdateChain[T, ID] {
	if c == nil {
		return c
	}
	if c.wrapper == nil {
		c.wrapper = NewUpdateWrapper[T]()
	}
	if apply != nil {
		apply(c.wrapper)
	}
	return c
}

// Set 添加字段赋值。
func (c *UpdateChain[T, ID]) Set(field Field[T], value any) *UpdateChain[T, ID] {
	return c.Where(func(wrapper *UpdateWrapper[T]) { wrapper.Set(field, value) })
}

// SetIncrBy 添加字段自增赋值。
func (c *UpdateChain[T, ID]) SetIncrBy(field Field[T], value any) *UpdateChain[T, ID] {
	return c.Where(func(wrapper *UpdateWrapper[T]) { wrapper.SetIncrBy(field, value) })
}

// SetDecrBy 添加字段自减赋值。
func (c *UpdateChain[T, ID]) SetDecrBy(field Field[T], value any) *UpdateChain[T, ID] {
	return c.Where(func(wrapper *UpdateWrapper[T]) { wrapper.SetDecrBy(field, value) })
}

// Eq 添加等值条件。
func (c *UpdateChain[T, ID]) Eq(field Field[T], value any) *UpdateChain[T, ID] {
	return c.Where(func(wrapper *UpdateWrapper[T]) { wrapper.Eq(field, value) })
}

// Update 执行链式更新。
func (c *UpdateChain[T, ID]) Update(ctx context.Context) (int64, error) {
	if c == nil || c.service == nil {
		return 0, fmt.Errorf("goark-orm: update chain service is nil")
	}
	return c.service.UpdateWithWrapper(ctx, c.wrapper)
}
