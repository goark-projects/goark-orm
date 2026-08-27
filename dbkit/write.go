package dbkit

import (
	"context"

	orm "goark.dev/orm"
)

// Save 插入实体。
func (k *Kit[T, ID]) Save(ctx context.Context, entity *T) (bool, error) {
	service, err := k.requireService()
	if err != nil {
		return false, err
	}
	return service.Save(ctx, entity)
}

// SaveBatch 按默认批量大小插入实体。
func (k *Kit[T, ID]) SaveBatch(ctx context.Context, entities []T) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.SaveBatch(ctx, entities)
}

// SaveBatchSize 按指定批量大小插入实体。
func (k *Kit[T, ID]) SaveBatchSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.SaveBatchSize(ctx, entities, batchSize)
}

// SaveOrUpdate 根据主键零值插入或更新实体。
func (k *Kit[T, ID]) SaveOrUpdate(ctx context.Context, entity *T) (bool, error) {
	service, err := k.requireService()
	if err != nil {
		return false, err
	}
	return service.SaveOrUpdate(ctx, entity)
}

// SaveOrUpdateBatch 按默认批量大小保存或更新实体。
func (k *Kit[T, ID]) SaveOrUpdateBatch(ctx context.Context, entities []T) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.SaveOrUpdateBatch(ctx, entities)
}

// SaveOrUpdateBatchSize 按指定批量大小保存或更新实体。
func (k *Kit[T, ID]) SaveOrUpdateBatchSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.SaveOrUpdateBatchSize(ctx, entities, batchSize)
}

// RemoveByID 按主键删除实体。
func (k *Kit[T, ID]) RemoveByID(ctx context.Context, id ID) (bool, error) {
	service, err := k.requireService()
	if err != nil {
		return false, err
	}
	return service.RemoveByID(ctx, id)
}

// RemoveByIDs 按主键集合删除实体。
func (k *Kit[T, ID]) RemoveByIDs(ctx context.Context, ids []ID) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.RemoveByIDs(ctx, ids)
}

// Remove 按条件删除实体。
func (k *Kit[T, ID]) Remove(ctx context.Context, wrapper *orm.QueryWrapper[T]) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.Remove(ctx, wrapper)
}

// RemoveByMap 按列名 map 等值删除实体。
func (k *Kit[T, ID]) RemoveByMap(ctx context.Context, columnMap map[string]any) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.RemoveByMap(ctx, columnMap)
}

// UpdateByID 按主键更新实体。
func (k *Kit[T, ID]) UpdateByID(ctx context.Context, entity *T) (bool, error) {
	service, err := k.requireService()
	if err != nil {
		return false, err
	}
	return service.UpdateByID(ctx, entity)
}

// UpdateBatchByID 按默认批量大小根据主键更新实体。
func (k *Kit[T, ID]) UpdateBatchByID(ctx context.Context, entities []T) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.UpdateBatchByID(ctx, entities)
}

// UpdateBatchByIDSize 按指定批量大小根据主键更新实体。
func (k *Kit[T, ID]) UpdateBatchByIDSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.UpdateBatchByIDSize(ctx, entities, batchSize)
}

// Update 按实体和条件更新记录。
func (k *Kit[T, ID]) Update(ctx context.Context, entity *T, wrapper *orm.QueryWrapper[T]) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.Update(ctx, entity, wrapper)
}

// UpdateWithWrapper 按 UpdateWrapper 更新记录。
func (k *Kit[T, ID]) UpdateWithWrapper(ctx context.Context, wrapper *orm.UpdateWrapper[T]) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.UpdateWithWrapper(ctx, wrapper)
}

// UpdateChain 创建链式更新。
func (k *Kit[T, ID]) UpdateChain() *orm.UpdateChain[T, ID] {
	if k == nil || k.service == nil {
		return nil
	}
	return k.service.ChainUpdate()
}
