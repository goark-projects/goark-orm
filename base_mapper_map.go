package orm

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// SelectByMap 按列名 map 等值查询记录。
func (m *BaseMapper[T, ID]) SelectByMap(ctx context.Context, columnMap map[string]any) ([]T, error) {
	wrapper, err := queryWrapperFromColumnMap[T](columnMap, false)
	if err != nil {
		return nil, err
	}
	return m.SelectList(ctx, wrapper)
}

// DeleteByMap 按列名 map 等值删除记录，空 map 会被拒绝。
func (m *BaseMapper[T, ID]) DeleteByMap(ctx context.Context, columnMap map[string]any) (int64, error) {
	wrapper, err := queryWrapperFromColumnMap[T](columnMap, true)
	if err != nil {
		return 0, err
	}
	return m.Delete(ctx, wrapper)
}

func queryWrapperFromColumnMap[T any](columnMap map[string]any, requireNonEmpty bool) (*QueryWrapper[T], error) {
	if len(columnMap) == 0 {
		if requireNonEmpty {
			return nil, fmt.Errorf("goark-orm: column map must contain conditions")
		}
		return NewQueryWrapper[T](), nil
	}
	entries, err := sortedColumnMapEntries(columnMap)
	if err != nil {
		return nil, err
	}
	wrapper := NewQueryWrapper[T]()
	for _, entry := range entries {
		field := Field[T]{Column: entry.column}
		if isNilValue(entry.value) {
			wrapper.IsNull(field)
			continue
		}
		wrapper.Eq(field, entry.value)
	}
	return wrapper, nil
}

type columnMapEntry struct {
	column string
	value  any
}

func sortedColumnMapEntries(values map[string]any) ([]columnMapEntry, error) {
	entries := make([]columnMapEntry, 0, len(values))
	for key, value := range values {
		column := strings.TrimSpace(key)
		if column == "" {
			return nil, fmt.Errorf("goark-orm: column map contains empty column")
		}
		entries = append(entries, columnMapEntry{column: column, value: value})
	}
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].column < entries[j].column
	})
	return entries, nil
}
