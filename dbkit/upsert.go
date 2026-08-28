package dbkit

import (
	"context"

	orm "goark.dev/orm"
)

// Upsert 执行方言原生插入或更新。
func (k *Kit[T, ID]) Upsert(ctx context.Context, entity *T, conflictFields []orm.Field[T], updateFields []orm.Field[T]) (bool, error) {
	service, err := k.requireService()
	if err != nil {
		return false, err
	}
	return service.Upsert(ctx, entity, conflictFields, updateFields)
}

// UpsertBatch 按默认批量大小执行原生 upsert。
func (k *Kit[T, ID]) UpsertBatch(ctx context.Context, entities []T, conflictFields []orm.Field[T], updateFields []orm.Field[T]) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.UpsertBatch(ctx, entities, conflictFields, updateFields)
}

// UpsertBatchSize 按指定批量大小执行原生 upsert。
func (k *Kit[T, ID]) UpsertBatchSize(ctx context.Context, entities []T, conflictFields []orm.Field[T], updateFields []orm.Field[T], batchSize int) (int64, error) {
	service, err := k.requireService()
	if err != nil {
		return 0, err
	}
	return service.UpsertBatchSize(ctx, entities, conflictFields, updateFields, batchSize)
}
