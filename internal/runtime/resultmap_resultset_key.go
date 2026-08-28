package runtime

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func indexResultSetParents(roots []reflect.Value, resultMap ResultMapMeta, plan resultSetMappingPlan) (map[string][]int, error) {
	indexes := make(map[string][]int, len(roots))
	for index, root := range roots {
		key, ok, err := resultSetRootKey(root, resultMap, plan.parentColumns)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		indexes[key] = append(indexes[key], index)
	}
	return indexes, nil
}

func resultSetRootKey(root reflect.Value, resultMap ResultMapMeta, columns []string) (string, bool, error) {
	values := make([]any, 0, len(columns))
	for _, column := range columns {
		value, ok, err := resultSetRootColumnValue(root, resultMap, column)
		if err != nil || !ok {
			return "", false, err
		}
		if value == nil {
			return "", false, nil
		}
		values = append(values, value)
	}
	return resultSetKey(values), true, nil
}

func resultSetRootColumnValue(root reflect.Value, resultMap ResultMapMeta, column string) (any, bool, error) {
	if property := resultMapPropertyForColumn(resultMap, column); property != "" {
		return resultSetRootFieldValue(root, property)
	}
	return resultSetRootFieldValue(root, column)
}

func resultMapPropertyForColumn(resultMap ResultMapMeta, column string) string {
	key := normalizeColumnKey(column)
	for _, field := range resultMapFieldMetas(resultMap) {
		if normalizeColumnKey(field.Column) == key {
			return strings.TrimSpace(field.Property)
		}
	}
	return ""
}

func resultSetRootFieldValue(root reflect.Value, property string) (any, bool, error) {
	field, ok := exportedFieldByProperty(root, property)
	if !ok {
		return nil, false, nil
	}
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return nil, true, nil
		}
		field = field.Elem()
	}
	if !field.CanInterface() {
		return nil, false, fmt.Errorf("field %s cannot be read", property)
	}
	return field.Interface(), true, nil
}

func resultSetRowKey(columnIndexes map[string]int, values []any, columns []string) (string, bool, error) {
	keyValues := make([]any, 0, len(columns))
	for _, column := range columns {
		index, ok := columnIndexes[normalizeColumnKey(column)]
		if !ok || index < 0 || index >= len(values) {
			return "", false, fmt.Errorf("foreignColumn %s is not present", column)
		}
		if values[index] == nil {
			return "", false, nil
		}
		keyValues = append(keyValues, values[index])
	}
	return resultSetKey(keyValues), true, nil
}

func resultSetKey(values []any) string {
	var builder strings.Builder
	for index, value := range values {
		builder.WriteString(strconv.Itoa(index))
		builder.WriteByte('=')
		writeResultKeyValue(&builder, value)
		builder.WriteByte(';')
	}
	return builder.String()
}
