package orm

import "strings"

// AutoMappingBehavior 描述自动映射策略。
type AutoMappingBehavior string

const (
	// AutoMappingBehaviorNone 表示禁用自动映射，仅使用显式 resultMap。
	AutoMappingBehaviorNone AutoMappingBehavior = "NONE"
	// AutoMappingBehaviorPartial 表示仅对没有嵌套结果对象的 resultMap 自动映射。
	AutoMappingBehaviorPartial AutoMappingBehavior = "PARTIAL"
	// AutoMappingBehaviorFull 表示对所有可匹配列执行自动映射。
	AutoMappingBehaviorFull AutoMappingBehavior = "FULL"
)

// AutoMappingUnknownColumnBehavior 描述自动映射遇到未知列时的处理方式。
type AutoMappingUnknownColumnBehavior string

const (
	// AutoMappingUnknownColumnBehaviorNone 表示忽略未知列。
	AutoMappingUnknownColumnBehaviorNone AutoMappingUnknownColumnBehavior = "NONE"
	// AutoMappingUnknownColumnBehaviorWarning 表示保留兼容行为，当前核心不直接写日志。
	AutoMappingUnknownColumnBehaviorWarning AutoMappingUnknownColumnBehavior = "WARNING"
	// AutoMappingUnknownColumnBehaviorFailing 表示遇到未知列立即返回映射错误。
	AutoMappingUnknownColumnBehaviorFailing AutoMappingUnknownColumnBehavior = "FAILING"
)

// ParseAutoMappingBehavior 解析 autoMappingBehavior 配置值。
func ParseAutoMappingBehavior(value string) (AutoMappingBehavior, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case string(AutoMappingBehaviorNone):
		return AutoMappingBehaviorNone, nil
	case string(AutoMappingBehaviorPartial):
		return AutoMappingBehaviorPartial, nil
	case string(AutoMappingBehaviorFull):
		return AutoMappingBehaviorFull, nil
	default:
		return "", configurationErrorf("autoMappingBehavior %q is invalid", value)
	}
}

// ParseAutoMappingUnknownColumnBehavior 解析 autoMappingUnknownColumnBehavior 配置值。
func ParseAutoMappingUnknownColumnBehavior(value string) (AutoMappingUnknownColumnBehavior, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case string(AutoMappingUnknownColumnBehaviorNone):
		return AutoMappingUnknownColumnBehaviorNone, nil
	case string(AutoMappingUnknownColumnBehaviorWarning):
		return AutoMappingUnknownColumnBehaviorWarning, nil
	case string(AutoMappingUnknownColumnBehaviorFailing):
		return AutoMappingUnknownColumnBehaviorFailing, nil
	default:
		return "", configurationErrorf("autoMappingUnknownColumnBehavior %q is invalid", value)
	}
}

func normalizeJDBCTypeName(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	for _, item := range value {
		if item >= 'A' && item <= 'Z' || item >= '0' && item <= '9' || item == '_' {
			continue
		}
		return "", configurationErrorf("jdbcTypeForNull %q is invalid", value)
	}
	return value, nil
}
