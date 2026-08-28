package runtime

import "context"

// Upsert 执行方言原生插入或更新。
func (s *Service[T, ID]) Upsert(ctx context.Context, entity *T, conflictFields []Field[T], updateFields []Field[T]) (bool, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return false, err
	}
	result, err := mapper.Upsert(ctx, entity, conflictFields, updateFields)
	if err != nil {
		return false, err
	}
	return result.RowsAffected > 0, nil
}

// UpsertBatch 按默认批量大小执行原生 upsert。
func (s *Service[T, ID]) UpsertBatch(ctx context.Context, entities []T, conflictFields []Field[T], updateFields []Field[T]) (int64, error) {
	return s.UpsertBatchSize(ctx, entities, conflictFields, updateFields, DefaultBatchSize)
}

// UpsertBatchSize 按指定批量大小执行原生 upsert。
func (s *Service[T, ID]) UpsertBatchSize(ctx context.Context, entities []T, conflictFields []Field[T], updateFields []Field[T], batchSize int) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.UpsertBatchSize(ctx, entities, conflictFields, updateFields, batchSize)
}
