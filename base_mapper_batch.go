package orm

import "context"

const (
	// DefaultBatchSize 是批量写入默认 flush 大小，对齐 MyBatis-Plus 常用默认值。
	DefaultBatchSize = 1000
)

// InsertBatch 按默认批量大小插入实体。
func (m *BaseMapper[T, ID]) InsertBatch(ctx context.Context, entities []T) (int64, error) {
	return m.InsertBatchSize(ctx, entities, DefaultBatchSize)
}

// InsertBatchSize 按指定批量大小插入实体。底层 Session 支持批处理时会分批 flush。
func (m *BaseMapper[T, ID]) InsertBatchSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	if len(entities) == 0 {
		return 0, nil
	}
	batchSize = normalizeBatchSize(batchSize)
	if session, ok := m.session.(Session); ok {
		return m.runBatch(ctx, session, len(entities), batchSize, func(mapper *BaseMapper[T, ID], index int) error {
			_, err := mapper.Insert(ctx, &entities[index])
			return err
		})
	}
	var rows int64
	for index := range entities {
		result, err := m.Insert(ctx, &entities[index])
		if err != nil {
			return rows, err
		}
		rows += result.RowsAffected
	}
	return rows, nil
}

// UpdateBatchByID 按默认批量大小根据主键更新实体。
func (m *BaseMapper[T, ID]) UpdateBatchByID(ctx context.Context, entities []T) (int64, error) {
	return m.UpdateBatchByIDSize(ctx, entities, DefaultBatchSize)
}

// UpdateBatchByIDSize 按指定批量大小根据主键更新实体。底层 Session 支持批处理时会分批 flush。
func (m *BaseMapper[T, ID]) UpdateBatchByIDSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	if len(entities) == 0 {
		return 0, nil
	}
	batchSize = normalizeBatchSize(batchSize)
	if session, ok := m.session.(Session); ok {
		return m.runBatch(ctx, session, len(entities), batchSize, func(mapper *BaseMapper[T, ID], index int) error {
			_, err := mapper.UpdateByID(ctx, &entities[index])
			return err
		})
	}
	var rows int64
	for index := range entities {
		affected, err := m.UpdateByID(ctx, &entities[index])
		if err != nil {
			return rows, err
		}
		rows += affected
	}
	return rows, nil
}

// SaveOrUpdateBatch 按默认批量大小保存或更新实体。
func (m *BaseMapper[T, ID]) SaveOrUpdateBatch(ctx context.Context, entities []T) (int64, error) {
	return m.SaveOrUpdateBatchSize(ctx, entities, DefaultBatchSize)
}

// SaveOrUpdateBatchSize 保存或更新实体。该方法保留逐条语义，确保更新 0 行时再插入。
func (m *BaseMapper[T, ID]) SaveOrUpdateBatchSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	if len(entities) == 0 {
		return 0, nil
	}
	var rows int64
	for index := range entities {
		result, err := m.SaveOrUpdate(ctx, &entities[index])
		if err != nil {
			return rows, err
		}
		rows += result.RowsAffected
	}
	return rows, nil
}

func (m *BaseMapper[T, ID]) runBatch(ctx context.Context, session Session, count int, batchSize int, queue func(*BaseMapper[T, ID], int) error) (int64, error) {
	batch, err := NewBatchSession(session)
	if err != nil {
		return 0, err
	}
	mapper := m.withStatementSession(batch)
	var rows int64
	pending := 0
	flush := func() error {
		results, err := batch.Flush(ctx)
		rows += batchRowsAffected(results)
		pending = 0
		return err
	}
	for index := 0; index < count; index++ {
		if err := queue(mapper, index); err != nil {
			batch.Clear()
			return rows, err
		}
		pending++
		if pending >= batchSize {
			if err := flush(); err != nil {
				return rows, err
			}
		}
	}
	if pending > 0 {
		if err := flush(); err != nil {
			return rows, err
		}
	}
	return rows, nil
}

func (m *BaseMapper[T, ID]) withStatementSession(session StatementSession) *BaseMapper[T, ID] {
	copied := *m
	copied.session = session
	return &copied
}

func batchRowsAffected(results []BatchResult) int64 {
	var rows int64
	for _, result := range results {
		rows += result.Result.RowsAffected
	}
	return rows
}

func normalizeBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return DefaultBatchSize
	}
	return batchSize
}
