package dbkit

import (
	"context"

	orm "goark.dev/orm"
)

// GetByID 按主键查询实体。
func (k *Kit[T, ID]) GetByID(ctx context.Context, id ID) (*T, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.GetByID(ctx, id)
}

// GetOne 按条件查询单条实体，多行时返回错误。
func (k *Kit[T, ID]) GetOne(ctx context.Context, wrapper *orm.QueryWrapper[T]) (*T, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.GetOne(ctx, wrapper)
}

// First 按条件查询第一条实体。
func (k *Kit[T, ID]) First(ctx context.Context, wrapper *orm.QueryWrapper[T]) (*T, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.GetOneOrFirst(ctx, wrapper)
}

// List 按条件查询实体列表。
func (k *Kit[T, ID]) List(ctx context.Context, wrapper *orm.QueryWrapper[T]) ([]T, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.List(ctx, wrapper)
}

// ListByMap 按列名 map 等值查询实体列表。
func (k *Kit[T, ID]) ListByMap(ctx context.Context, columnMap map[string]any) ([]T, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.ListByMap(ctx, columnMap)
}

// ListByIDs 按主键集合查询实体列表。
func (k *Kit[T, ID]) ListByIDs(ctx context.Context, ids []ID) ([]T, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.ListByIDs(ctx, ids)
}

// ListMaps 按条件查询 map 列表。
func (k *Kit[T, ID]) ListMaps(ctx context.Context, wrapper *orm.QueryWrapper[T]) ([]map[string]any, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.ListMaps(ctx, wrapper)
}

// ListObjs 按条件查询首列列表。
func (k *Kit[T, ID]) ListObjs(ctx context.Context, wrapper *orm.QueryWrapper[T]) ([]any, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.ListObjs(ctx, wrapper)
}

// Count 按条件统计记录数。
func (k *Kit[T, ID]) Count(ctx context.Context, wrapper *orm.QueryWrapper[T]) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.Count(ctx, wrapper)
}

// Page 按条件分页查询实体列表。
func (k *Kit[T, ID]) Page(ctx context.Context, page orm.PageRequest, wrapper *orm.QueryWrapper[T]) (orm.Page[T], error) {
	service, err := k.requireService()
	if err != nil {
		return orm.Page[T]{}, err
	}
	return service.Page(ctx, page, wrapper)
}

// PageMaps 按条件分页查询 map 列表。
func (k *Kit[T, ID]) PageMaps(ctx context.Context, page orm.PageRequest, wrapper *orm.QueryWrapper[T]) (orm.Page[map[string]any], error) {
	service, err := k.requireService()
	if err != nil {
		return orm.Page[map[string]any]{}, err
	}
	return service.PageMaps(ctx, page, wrapper)
}

// Query 创建链式查询。
func (k *Kit[T, ID]) Query() *orm.QueryChain[T, ID] {
	if k == nil || k.service == nil {
		return nil
	}
	return k.service.ChainQuery()
}
