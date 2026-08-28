package runtime

import "sort"

func defaultOrderColumns(columns []ColumnMeta) []ColumnMeta {
	orders := make([]ColumnMeta, 0, len(columns))
	for _, column := range columns {
		if column.OrderBy {
			orders = append(orders, column)
		}
	}
	sort.SliceStable(orders, func(i int, j int) bool {
		return orders[i].OrderPriority < orders[j].OrderPriority
	})
	return orders
}
