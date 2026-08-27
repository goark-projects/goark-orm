package dbkit

import (
	"context"
	"fmt"

	orm "goark.dev/orm"
)

// List 查询实体后投影为目标值列表。
func List[T any, ID any, V any](ctx context.Context, mapper *orm.BaseMapper[T, ID], wrapper *orm.QueryWrapper[T], pick func(T) V) ([]V, error) {
	records, err := selectList(ctx, mapper, wrapper)
	if err != nil {
		return nil, err
	}
	return ListFrom(records, pick)
}

// ListFrom 将已有实体列表投影为目标值列表。
func ListFrom[T any, V any](records []T, pick func(T) V) ([]V, error) {
	if pick == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: list projector is nil")
	}
	out := make([]V, 0, len(records))
	for _, record := range records {
		out = append(out, pick(record))
	}
	return out, nil
}

// KeyMap 查询实体后按 key 函数构造成 map。
func KeyMap[T any, ID any, K comparable](ctx context.Context, mapper *orm.BaseMapper[T, ID], wrapper *orm.QueryWrapper[T], key func(T) K) (map[K]T, error) {
	records, err := selectList(ctx, mapper, wrapper)
	if err != nil {
		return nil, err
	}
	return KeyMapFrom(records, key)
}

// KeyMapFrom 将已有实体列表按 key 函数构造成 map。
func KeyMapFrom[T any, K comparable](records []T, key func(T) K) (map[K]T, error) {
	return MapFrom(records, key, func(record T) T { return record })
}

// Map 查询实体后构造成目标值 map。
func Map[T any, ID any, K comparable, V any](ctx context.Context, mapper *orm.BaseMapper[T, ID], wrapper *orm.QueryWrapper[T], key func(T) K, value func(T) V) (map[K]V, error) {
	records, err := selectList(ctx, mapper, wrapper)
	if err != nil {
		return nil, err
	}
	return MapFrom(records, key, value)
}

// MapFrom 将已有实体列表构造成目标值 map，重复 key 以后写入值为准。
func MapFrom[T any, K comparable, V any](records []T, key func(T) K, value func(T) V) (map[K]V, error) {
	if key == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: map key function is nil")
	}
	if value == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: map value function is nil")
	}
	out := make(map[K]V, len(records))
	for _, record := range records {
		out[key(record)] = value(record)
	}
	return out, nil
}

// Group 查询实体后按 key 函数分组。
func Group[T any, ID any, K comparable](ctx context.Context, mapper *orm.BaseMapper[T, ID], wrapper *orm.QueryWrapper[T], key func(T) K) (map[K][]T, error) {
	records, err := selectList(ctx, mapper, wrapper)
	if err != nil {
		return nil, err
	}
	return GroupFrom(records, key)
}

// GroupFrom 将已有实体列表按 key 函数分组。
func GroupFrom[T any, K comparable](records []T, key func(T) K) (map[K][]T, error) {
	return GroupValuesFrom(records, key, func(record T) T { return record })
}

// GroupValues 查询实体后按 key 函数分组并投影组内值。
func GroupValues[T any, ID any, K comparable, V any](ctx context.Context, mapper *orm.BaseMapper[T, ID], wrapper *orm.QueryWrapper[T], key func(T) K, value func(T) V) (map[K][]V, error) {
	records, err := selectList(ctx, mapper, wrapper)
	if err != nil {
		return nil, err
	}
	return GroupValuesFrom(records, key, value)
}

// GroupValuesFrom 将已有实体列表按 key 函数分组并投影组内值。
func GroupValuesFrom[T any, K comparable, V any](records []T, key func(T) K, value func(T) V) (map[K][]V, error) {
	if key == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: group key function is nil")
	}
	if value == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: group value function is nil")
	}
	out := make(map[K][]V)
	for _, record := range records {
		groupKey := key(record)
		out[groupKey] = append(out[groupKey], value(record))
	}
	return out, nil
}

func selectList[T any, ID any](ctx context.Context, mapper *orm.BaseMapper[T, ID], wrapper *orm.QueryWrapper[T]) ([]T, error) {
	if mapper == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: mapper is nil")
	}
	return mapper.SelectList(ctx, wrapper)
}
