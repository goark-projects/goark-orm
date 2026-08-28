package runtime

import "fmt"

type TypedConditionTarget[T any, W any] interface {
	addTypedCondition(field Field[T], op conditionOperator, value any) W
}

func (w *QueryWrapper[T]) addTypedCondition(field Field[T], op conditionOperator, value any) *QueryWrapper[T] {
	return w.add(field, op, value)
}

func (w *UpdateWrapper[T]) addTypedCondition(field Field[T], op conditionOperator, value any) *UpdateWrapper[T] {
	return w.add(field, op, value)
}

func addTypedConditionValue[T any, V any, W TypedConditionTarget[T, W]](
	w W,
	field TypedField[T, V],
	op conditionOperator,
	value V,
) W {
	return w.addTypedCondition(field.Field(), op, value)
}

// EqTypedValue 用泛型函数为 QueryWrapper 和 UpdateWrapper 保留字段值类型约束。
func EqTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionEq, value)
}

// EqTypedValueIf 在 condition 为 true 时添加类型安全的等值条件。
func EqTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return EqTypedValue(w, field, value)
}

// NeTypedValue 用泛型函数保留字段值类型约束并添加不等条件。
func NeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionNe, value)
}

// NeTypedValueIf 在 condition 为 true 时添加类型安全的不等条件。
func NeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return NeTypedValue(w, field, value)
}

// GtTypedValue 用泛型函数保留字段值类型约束并添加大于条件。
func GtTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionGt, value)
}

// GtTypedValueIf 在 condition 为 true 时添加类型安全的大于条件。
func GtTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return GtTypedValue(w, field, value)
}

// GeTypedValue 用泛型函数保留字段值类型约束并添加大于等于条件。
func GeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionGe, value)
}

// GeTypedValueIf 在 condition 为 true 时添加类型安全的大于等于条件。
func GeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return GeTypedValue(w, field, value)
}

// LtTypedValue 用泛型函数保留字段值类型约束并添加小于条件。
func LtTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionLt, value)
}

// LtTypedValueIf 在 condition 为 true 时添加类型安全的小于条件。
func LtTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return LtTypedValue(w, field, value)
}

// LeTypedValue 用泛型函数保留字段值类型约束并添加小于等于条件。
func LeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionLe, value)
}

// LeTypedValueIf 在 condition 为 true 时添加类型安全的小于等于条件。
func LeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return LeTypedValue(w, field, value)
}

// LikeTypedValue 用泛型函数保留字段值类型约束并添加 LIKE 条件。
func LikeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionLike, value)
}

// LikeTypedValueIf 在 condition 为 true 时添加类型安全的 LIKE 条件。
func LikeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return LikeTypedValue(w, field, value)
}

// NotLikeTypedValue 用泛型函数保留字段值类型约束并添加 NOT LIKE 条件。
func NotLikeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return addTypedConditionValue(w, field, conditionNotLike, value)
}

// NotLikeTypedValueIf 在 condition 为 true 时添加类型安全的 NOT LIKE 条件。
func NotLikeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return NotLikeTypedValue(w, field, value)
}

// LikeLeftTypedValue 用泛型函数保留字段值类型约束并添加左侧模糊匹配条件。
func LikeLeftTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return w.addTypedCondition(field.Field(), conditionLike, "%"+fmt.Sprint(value))
}

// LikeLeftTypedValueIf 在 condition 为 true 时添加类型安全的左侧模糊匹配条件。
func LikeLeftTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return LikeLeftTypedValue(w, field, value)
}

// LikeRightTypedValue 用泛型函数保留字段值类型约束并添加右侧模糊匹配条件。
func LikeRightTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return w.addTypedCondition(field.Field(), conditionLike, fmt.Sprint(value)+"%")
}

// LikeRightTypedValueIf 在 condition 为 true 时添加类型安全的右侧模糊匹配条件。
func LikeRightTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	if !condition {
		return w
	}
	return LikeRightTypedValue(w, field, value)
}

// InTypedValues 用泛型函数保留字段值类型约束并添加 IN 条件。
func InTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], values ...V) W {
	return w.addTypedCondition(field.Field(), conditionIn, values)
}

// InTypedValuesIf 在 condition 为 true 时添加类型安全的 IN 条件。
func InTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], values ...V) W {
	if !condition {
		return w
	}
	return InTypedValues(w, field, values...)
}

// NotInTypedValues 用泛型函数保留字段值类型约束并添加 NOT IN 条件。
func NotInTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], values ...V) W {
	return w.addTypedCondition(field.Field(), conditionNotIn, values)
}

// NotInTypedValuesIf 在 condition 为 true 时添加类型安全的 NOT IN 条件。
func NotInTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], values ...V) W {
	if !condition {
		return w
	}
	return NotInTypedValues(w, field, values...)
}

// BetweenTypedValues 用泛型函数保留字段值类型约束并添加 BETWEEN 条件。
func BetweenTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], left V, right V) W {
	return w.addTypedCondition(field.Field(), conditionBetween, betweenValues{left: left, right: right})
}

// BetweenTypedValuesIf 在 condition 为 true 时添加类型安全的 BETWEEN 条件。
func BetweenTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], left V, right V) W {
	if !condition {
		return w
	}
	return BetweenTypedValues(w, field, left, right)
}

// NotBetweenTypedValues 用泛型函数保留字段值类型约束并添加 NOT BETWEEN 条件。
func NotBetweenTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], left V, right V) W {
	return w.addTypedCondition(field.Field(), conditionNotBetween, betweenValues{left: left, right: right})
}

// NotBetweenTypedValuesIf 在 condition 为 true 时添加类型安全的 NOT BETWEEN 条件。
func NotBetweenTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], left V, right V) W {
	if !condition {
		return w
	}
	return NotBetweenTypedValues(w, field, left, right)
}
