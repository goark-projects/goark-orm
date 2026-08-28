package dbkit

import (
	"context"
	"fmt"

	orm "goark.dev/orm"
)

// ListFieldValues 按类型化字段查询单列值列表。
func ListFieldValues[T any, ID any, V any](ctx context.Context, kit *Kit[T, ID], field orm.TypedField[T, V], wrapper *orm.QueryWrapper[T]) ([]V, error) {
	if kit == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: kit is nil")
	}
	return orm.ListFieldValues(ctx, kit.Service(), field, wrapper)
}

// GetFieldValue 按类型化字段查询单个单列值，多行时返回错误。
func GetFieldValue[T any, ID any, V any](ctx context.Context, kit *Kit[T, ID], field orm.TypedField[T, V], wrapper *orm.QueryWrapper[T]) (V, error) {
	if kit == nil {
		var zero V
		return zero, fmt.Errorf("goark-orm/dbkit: kit is nil")
	}
	return orm.GetFieldValue(ctx, kit.Service(), field, wrapper)
}

// GetFirstFieldValue 按类型化字段查询第一条单列值。
func GetFirstFieldValue[T any, ID any, V any](ctx context.Context, kit *Kit[T, ID], field orm.TypedField[T, V], wrapper *orm.QueryWrapper[T]) (V, error) {
	if kit == nil {
		var zero V
		return zero, fmt.Errorf("goark-orm/dbkit: kit is nil")
	}
	return orm.GetFirstFieldValue(ctx, kit.Service(), field, wrapper)
}

// ListIDs 按主键类型查询 ID 列表。
func (k *Kit[T, ID]) ListIDs(ctx context.Context, wrapper *orm.QueryWrapper[T]) ([]ID, error) {
	service, err := k.requireService()
	if err != nil {
		return nil, err
	}
	return service.ListIDs(ctx, wrapper)
}
