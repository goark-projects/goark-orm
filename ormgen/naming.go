package ormgen

import (
	"fmt"
	"strings"
	"unicode"
)

// NamingStrategy 描述生成期表名和列名推导策略。
type NamingStrategy string

const (
	// NamingStrategyExplicit 表示必须显式声明名称。
	NamingStrategyExplicit NamingStrategy = ""
	// NamingStrategySame 表示直接使用 Go 类型或字段名。
	NamingStrategySame NamingStrategy = "same"
	// NamingStrategySnakeCase 表示使用 snake_case 名称。
	NamingStrategySnakeCase NamingStrategy = "snake_case"
)

// NamingConfig 描述生成期命名策略。
type NamingConfig struct {
	Table       NamingStrategy `json:"table,omitempty"`
	Column      NamingStrategy `json:"column,omitempty"`
	TablePrefix string         `json:"tablePrefix,omitempty"`
}

func normalizeNamingConfig(config NamingConfig) (NamingConfig, error) {
	table, err := normalizeNamingStrategy(config.Table)
	if err != nil {
		return NamingConfig{}, fmt.Errorf("table naming %w", err)
	}
	column, err := normalizeNamingStrategy(config.Column)
	if err != nil {
		return NamingConfig{}, fmt.Errorf("column naming %w", err)
	}
	config.Table = table
	config.Column = column
	config.TablePrefix = strings.TrimSpace(config.TablePrefix)
	return config, nil
}

func normalizeNamingStrategy(value NamingStrategy) (NamingStrategy, error) {
	raw := strings.TrimSpace(string(value))
	normalized := strings.ToLower(strings.ReplaceAll(raw, "-", "_"))
	switch normalized {
	case "", "explicit":
		return NamingStrategyExplicit, nil
	case "same":
		return NamingStrategySame, nil
	case "snake", "snake_case", "underline":
		return NamingStrategySnakeCase, nil
	default:
		return "", fmt.Errorf("strategy %q is invalid", value)
	}
}

func deriveTableName(typeName string, config NamingConfig) string {
	table := deriveName(typeName, config.Table)
	if table == "" {
		return ""
	}
	if config.TablePrefix != "" && !strings.HasPrefix(table, config.TablePrefix) {
		table = config.TablePrefix + table
	}
	return table
}

func deriveColumnName(fieldName string, config NamingConfig) string {
	return deriveName(fieldName, config.Column)
}

func deriveName(value string, strategy NamingStrategy) string {
	value = strings.TrimSpace(value)
	switch strategy {
	case NamingStrategySame:
		return value
	case NamingStrategySnakeCase:
		return toSnakeCase(value)
	default:
		return ""
	}
}

func toSnakeCase(value string) string {
	runes := []rune(strings.TrimSpace(value))
	var builder strings.Builder
	builder.Grow(len(value) + 4)
	prevClass := runeClassOther
	lastUnderscore := false
	for index, r := range runes {
		class := classifyRune(r)
		if class == runeClassOther {
			if builder.Len() > 0 && !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
			prevClass = runeClassOther
			continue
		}
		nextClass := runeClassOther
		if index+1 < len(runes) {
			nextClass = classifyRune(runes[index+1])
		}
		if builder.Len() > 0 && needsSnakeSeparator(prevClass, class, nextClass) && !lastUnderscore {
			builder.WriteByte('_')
		}
		builder.WriteRune(unicode.ToLower(r))
		lastUnderscore = false
		prevClass = class
	}
	return strings.Trim(builder.String(), "_")
}

type runeClass uint8

const (
	runeClassOther runeClass = iota
	runeClassLower
	runeClassUpper
	runeClassDigit
)

func classifyRune(r rune) runeClass {
	switch {
	case unicode.IsLower(r):
		return runeClassLower
	case unicode.IsUpper(r):
		return runeClassUpper
	case unicode.IsDigit(r):
		return runeClassDigit
	default:
		return runeClassOther
	}
}

func needsSnakeSeparator(prev runeClass, current runeClass, next runeClass) bool {
	if prev == runeClassOther || current == runeClassOther {
		return false
	}
	if current == runeClassUpper && (prev == runeClassLower || prev == runeClassDigit) {
		return true
	}
	if current == runeClassUpper && prev == runeClassUpper && next == runeClassLower {
		return true
	}
	if current == runeClassDigit && prev != runeClassDigit {
		return true
	}
	return (current == runeClassLower || current == runeClassUpper) && prev == runeClassDigit
}
