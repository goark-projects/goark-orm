package orm

import (
	"context"
	"database/sql"
	"fmt"
)

// Service 提供 MyBatis-Plus IService 风格的实体服务层。
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

// RemoveBatchByIDs 是 RemoveByIDs 的 MyBatis-Plus 命名别名。
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

// ChainQuery 创建链式查询。
func (s *Service[T, ID]) ChainQuery() *QueryChain[T, ID] {
	return &QueryChain[T, ID]{service: s, wrapper: NewQueryWrapper[T]()}
}

// ChainUpdate 创建链式更新。
func (s *Service[T, ID]) ChainUpdate() *UpdateChain[T, ID] {
	return &UpdateChain[T, ID]{service: s, wrapper: NewUpdateWrapper[T]()}
}

func (s *Service[T, ID]) requireMapper() (*BaseMapper[T, ID], error) {
	if s == nil || s.mapper == nil {
		return nil, fmt.Errorf("goark-orm: service mapper is nil")
	}
	return s.mapper, nil
}

// QueryChain 提供 MyBatis-Plus QueryChainWrapper 风格的查询链。
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

// UpdateChain 提供 MyBatis-Plus UpdateChainWrapper 风格的更新链。
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
