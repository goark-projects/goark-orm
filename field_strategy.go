package orm

import (
	"fmt"
	"strings"
)

// FieldStrategy 描述字段参与 INSERT、UPDATE、WHERE 片段的策略。
type FieldStrategy string

const (
	// FieldStrategyDefault 表示沿用调用方或全局默认策略。
	FieldStrategyDefault FieldStrategy = ""
	// FieldStrategyAlways 表示字段始终参与 SQL。
	FieldStrategyAlways FieldStrategy = "ALWAYS"
	// FieldStrategyNotNull 表示字段值非 nil 时参与 SQL。
	FieldStrategyNotNull FieldStrategy = "NOT_NULL"
	// FieldStrategyNotEmpty 表示字段值非 nil 且非空字符串、切片、数组或 map 时参与 SQL。
	FieldStrategyNotEmpty FieldStrategy = "NOT_EMPTY"
	// FieldStrategyNotZero 表示字段值非 nil 且非 Go 零值时参与 SQL。
	FieldStrategyNotZero FieldStrategy = "NOT_ZERO"
	// FieldStrategyNever 表示字段永不参与对应 SQL。
	FieldStrategyNever FieldStrategy = "NEVER"
)

// ParseFieldStrategy 解析字段策略，兼容下划线和短横线写法。
func ParseFieldStrategy(value string) (FieldStrategy, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch FieldStrategy(normalized) {
	case FieldStrategyDefault:
		return FieldStrategyDefault, nil
	case FieldStrategyAlways:
		return FieldStrategyAlways, nil
	case FieldStrategyNotNull:
		return FieldStrategyNotNull, nil
	case FieldStrategyNotEmpty:
		return FieldStrategyNotEmpty, nil
	case FieldStrategyNotZero:
		return FieldStrategyNotZero, nil
	case FieldStrategyNever:
		return FieldStrategyNever, nil
	default:
		return "", fmt.Errorf("goark-orm: unsupported field strategy %q", value)
	}
}
