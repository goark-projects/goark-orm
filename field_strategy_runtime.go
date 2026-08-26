package orm

import "reflect"

func effectiveFieldStrategy(column FieldStrategy, global FieldStrategy) FieldStrategy {
	if column != FieldStrategyDefault {
		return column
	}
	if global != FieldStrategyDefault {
		return global
	}
	return FieldStrategyAlways
}

func fieldIncludedByStrategy(value any, strategy FieldStrategy) bool {
	switch strategy {
	case FieldStrategyNever:
		return false
	case FieldStrategyNotNull:
		return !isNilValue(value)
	case FieldStrategyNotEmpty:
		return !isEmptyStrategyValue(value)
	default:
		return true
	}
}

func isEmptyStrategyValue(value any) bool {
	if isNilValue(value) {
		return true
	}
	current := reflect.ValueOf(value)
	for current.Kind() == reflect.Interface || current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return true
		}
		current = current.Elem()
	}
	switch current.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return current.Len() == 0
	default:
		return false
	}
}
