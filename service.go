package orm

import (
	"context"
	"database/sql"
	"fmt"
)

// Service 提供基于 BaseMapper 的实体服务层。
type Service[T any, ID any] struct {
	mapper *BaseMapper[T, ID]
}

// NewService 基于 BaseMapper 创建服务层实例。
func NewService[T any, ID any](mapper *BaseMapper[T, ID]) (*Service[T, ID], error) {
	if mapper == nil {
		return nil, fmt.Errorf("goark-orm: service mapper is nil")
	}
	return &Service[T, ID]{mapper: mapper}, nil
}

// BaseMapper 返回底层通用 Mapper。
func (s *Service[T, ID]) BaseMapper() *BaseMapper[T, ID] {
	if s == nil {
		return nil
	}
	return s.mapper
}

// Save 插入实体。
func (s *Service[T, ID]) Save(ctx context.Context, entity *T) (bool, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return false, err
	}
	result, err := mapper.Insert(ctx, entity)
	if err != nil {
		return false, err
	}
	return result.RowsAffected > 0, nil
}

// SaveBatch 按顺序批量插入实体。
func (s *Service[T, ID]) SaveBatch(ctx context.Context, entities []T) (int64, error) {
	return s.SaveBatchSize(ctx, entities, DefaultBatchSize)
}

// SaveBatchSize 按指定批量大小插入实体。
func (s *Service[T, ID]) SaveBatchSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.InsertBatchSize(ctx, entities, batchSize)
}

// SaveOrUpdate 根据主键零值选择插入或更新。
func (s *Service[T, ID]) SaveOrUpdate(ctx context.Context, entity *T) (bool, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return false, err
	}
	result, err := mapper.SaveOrUpdate(ctx, entity)
	if err != nil {
		return false, err
	}
	return result.RowsAffected > 0, nil
}

// SaveOrUpdateBatch 按顺序批量保存或更新实体。
func (s *Service[T, ID]) SaveOrUpdateBatch(ctx context.Context, entities []T) (int64, error) {
	return s.SaveOrUpdateBatchSize(ctx, entities, DefaultBatchSize)
}

// SaveOrUpdateBatchSize 按指定批量大小保存或更新实体。
func (s *Service[T, ID]) SaveOrUpdateBatchSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.SaveOrUpdateBatchSize(ctx, entities, batchSize)
}

// RemoveByID 按主键删除实体。
func (s *Service[T, ID]) RemoveByID(ctx context.Context, id ID) (bool, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return false, err
	}
	rows, err := mapper.DeleteByID(ctx, id)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// RemoveByIDs 按主键集合删除实体。
func (s *Service[T, ID]) RemoveByIDs(ctx context.Context, ids []ID) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.DeleteBatchIDs(ctx, ids)
}

// RemoveBatchByIDs 是 RemoveByIDs 的兼容命名别名。
func (s *Service[T, ID]) RemoveBatchByIDs(ctx context.Context, ids []ID) (int64, error) {
	return s.RemoveByIDs(ctx, ids)
}

// Remove 按条件删除实体。
func (s *Service[T, ID]) Remove(ctx context.Context, wrapper *QueryWrapper[T]) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.Delete(ctx, wrapper)
}

// RemoveByEntity 按实体非零字段删除实体。
func (s *Service[T, ID]) RemoveByEntity(ctx context.Context, entity *T) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.DeleteByEntity(ctx, entity)
}

// RemoveByMap 按列名 map 等值删除实体。
func (s *Service[T, ID]) RemoveByMap(ctx context.Context, columnMap map[string]any) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.DeleteByMap(ctx, columnMap)
}

// UpdateByID 按主键更新实体。
func (s *Service[T, ID]) UpdateByID(ctx context.Context, entity *T) (bool, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return false, err
	}
	rows, err := mapper.UpdateByID(ctx, entity)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// UpdateBatchByID 按默认批量大小根据主键更新实体。
func (s *Service[T, ID]) UpdateBatchByID(ctx context.Context, entities []T) (int64, error) {
	return s.UpdateBatchByIDSize(ctx, entities, DefaultBatchSize)
}

// UpdateBatchByIDSize 按指定批量大小根据主键更新实体。
func (s *Service[T, ID]) UpdateBatchByIDSize(ctx context.Context, entities []T, batchSize int) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.UpdateBatchByIDSize(ctx, entities, batchSize)
}

// Update 按实体和条件更新记录。
func (s *Service[T, ID]) Update(ctx context.Context, entity *T, wrapper *QueryWrapper[T]) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.Update(ctx, entity, wrapper)
}

// UpdateWithWrapper 按 UpdateWrapper 更新记录。
func (s *Service[T, ID]) UpdateWithWrapper(ctx context.Context, wrapper *UpdateWrapper[T]) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.UpdateWithWrapper(ctx, wrapper)
}

// GetByID 按主键查询实体。
func (s *Service[T, ID]) GetByID(ctx context.Context, id ID) (*T, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectByID(ctx, id)
}

// GetOne 按条件查询单条实体。
func (s *Service[T, ID]) GetOne(ctx context.Context, wrapper *QueryWrapper[T]) (*T, error) {
	records, err := s.List(ctx, wrapper)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	if len(records) > 1 {
		return nil, fmt.Errorf("goark-orm: service GetOne returned more than one row")
	}
	return &records[0], nil
}

// GetOneOrFirst 按条件查询单条实体，多行时返回第一行。
func (s *Service[T, ID]) GetOneOrFirst(ctx context.Context, wrapper *QueryWrapper[T]) (*T, error) {
	records, err := s.List(ctx, wrapper)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return &records[0], nil
}

// GetMap 按条件查询单条 map 结果。
func (s *Service[T, ID]) GetMap(ctx context.Context, wrapper *QueryWrapper[T]) (map[string]any, error) {
	records, err := s.ListMaps(ctx, wrapper)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	if len(records) > 1 {
		return nil, fmt.Errorf("goark-orm: service GetMap returned more than one row")
	}
	return records[0], nil
}

// GetMapOrFirst 按条件查询 map 结果，多行时返回第一行。
func (s *Service[T, ID]) GetMapOrFirst(ctx context.Context, wrapper *QueryWrapper[T]) (map[string]any, error) {
	records, err := s.ListMaps(ctx, wrapper)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}

// GetObj 按条件查询首列单个值。
func (s *Service[T, ID]) GetObj(ctx context.Context, wrapper *QueryWrapper[T]) (any, error) {
	records, err := s.ListObjs(ctx, wrapper)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	if len(records) > 1 {
		return nil, fmt.Errorf("goark-orm: service GetObj returned more than one row")
	}
	return records[0], nil
}

// GetObjOrFirst 按条件查询首列值，多行时返回第一行。
func (s *Service[T, ID]) GetObjOrFirst(ctx context.Context, wrapper *QueryWrapper[T]) (any, error) {
	records, err := s.ListObjs(ctx, wrapper)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}

// List 按条件查询实体列表。wrapper 为 nil 时查询全部记录。
func (s *Service[T, ID]) List(ctx context.Context, wrapper *QueryWrapper[T]) ([]T, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectList(ctx, wrapper)
}

// ListByEntity 按实体非零字段查询实体列表。
func (s *Service[T, ID]) ListByEntity(ctx context.Context, entity *T) ([]T, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectListByEntity(ctx, entity)
}

// ListByMap 按列名 map 等值查询实体列表。
func (s *Service[T, ID]) ListByMap(ctx context.Context, columnMap map[string]any) ([]T, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectByMap(ctx, columnMap)
}

// ListByIDs 按主键集合查询实体列表。
func (s *Service[T, ID]) ListByIDs(ctx context.Context, ids []ID) ([]T, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectBatchIDs(ctx, ids)
}

// ListMaps 按条件查询 map 列表。
func (s *Service[T, ID]) ListMaps(ctx context.Context, wrapper *QueryWrapper[T]) ([]map[string]any, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectMaps(ctx, wrapper)
}

// ListObjs 按条件查询首列列表。
func (s *Service[T, ID]) ListObjs(ctx context.Context, wrapper *QueryWrapper[T]) ([]any, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectObjs(ctx, wrapper)
}

// Count 按条件统计记录数。
func (s *Service[T, ID]) Count(ctx context.Context, wrapper *QueryWrapper[T]) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.Count(ctx, wrapper)
}

// CountByEntity 按实体非零字段统计记录数。
func (s *Service[T, ID]) CountByEntity(ctx context.Context, entity *T) (int64, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return 0, err
	}
	return mapper.SelectCountByEntity(ctx, entity)
}

// Page 按条件分页查询实体列表。
func (s *Service[T, ID]) Page(ctx context.Context, page PageRequest, wrapper *QueryWrapper[T]) (Page[T], error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return Page[T]{}, err
	}
	return mapper.SelectPage(ctx, page, wrapper)
}

// PageMaps 按条件分页查询 map 结果。
func (s *Service[T, ID]) PageMaps(ctx context.Context, page PageRequest, wrapper *QueryWrapper[T]) (Page[map[string]any], error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return Page[map[string]any]{}, err
	}
	return mapper.SelectMapsPage(ctx, page, wrapper)
}

func (s *Service[T, ID]) requireMapper() (*BaseMapper[T, ID], error) {
	if s == nil || s.mapper == nil {
		return nil, fmt.Errorf("goark-orm: service mapper is nil")
	}
	return s.mapper, nil
}
