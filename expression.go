package orm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type valueLookup func(name string) (any, bool)

func evalExpression(expression string, lookup valueLookup) (bool, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return false, fmt.Errorf("goark-orm: dynamic SQL test expression is empty")
	}
	orParts := splitExpression(expression, "or")
	if len(orParts) > 1 {
		for _, part := range orParts {
			ok, err := evalExpression(part, lookup)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
	andParts := splitExpression(expression, "and")
	if len(andParts) > 1 {
		for _, part := range andParts {
			ok, err := evalExpression(part, lookup)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	}
	if left, right, ok := cutOperator(expression, "!="); ok {
		equal, err := compareExpressionValues(left, right, lookup)
		return !equal, err
	}
	if left, right, ok := cutOperator(expression, "=="); ok {
		return compareExpressionValues(left, right, lookup)
	}
	value, ok := lookup(strings.TrimSpace(expression))
	if !ok {
		return false, nil
	}
	return truthy(value), nil
}

func evalValueExpression(expression string, lookup valueLookup) (any, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("goark-orm: dynamic SQL value expression is empty")
	}
	parts := splitValueExpression(expression, '+')
	if len(parts) > 1 {
		var builder strings.Builder
		for _, part := range parts {
			value, ok := expressionValue(part, lookup)
			if !ok {
				return nil, fmt.Errorf("SQL value expression variable %q is missing", part)
			}
			if value != nil {
				builder.WriteString(fmt.Sprint(comparableValue(value)))
			}
		}
		return builder.String(), nil
	}
	value, ok := expressionValue(expression, lookup)
	if !ok {
		return nil, fmt.Errorf("SQL value expression variable %q is missing", expression)
	}
	return value, nil
}

func splitValueExpression(expression string, operator rune) []string {
	var out []string
	inQuote := rune(0)
	start := 0
	for index, r := range expression {
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			continue
		}
		if r == operator {
			out = append(out, strings.TrimSpace(expression[start:index]))
			start = index + 1
		}
	}
	if len(out) == 0 {
		return []string{expression}
	}
	out = append(out, strings.TrimSpace(expression[start:]))
	return out
}

func splitExpression(expression string, operator string) []string {
	var out []string
	inQuote := rune(0)
	token := " " + operator + " "
	start := 0
	for index, r := range expression {
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			continue
		}
		if strings.HasPrefix(expression[index:], token) {
			out = append(out, strings.TrimSpace(expression[start:index]))
			start = index + len(token)
		}
	}
	if len(out) == 0 {
		return []string{expression}
	}
	out = append(out, strings.TrimSpace(expression[start:]))
	return out
}

func cutOperator(expression string, operator string) (string, string, bool) {
	inQuote := rune(0)
	for index, r := range expression {
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			continue
		}
		if strings.HasPrefix(expression[index:], operator) {
			return strings.TrimSpace(expression[:index]), strings.TrimSpace(expression[index+len(operator):]), true
		}
	}
	return "", "", false
}

func compareExpressionValues(left string, right string, lookup valueLookup) (bool, error) {
	leftValue, _ := expressionValue(left, lookup)
	rightValue, _ := expressionValue(right, lookup)
	if isNilValue(leftValue) || isNilValue(rightValue) {
		return isNilValue(leftValue) && isNilValue(rightValue), nil
	}
	leftComparable := comparableValue(leftValue)
	rightComparable := comparableValue(rightValue)
	if reflect.TypeOf(leftComparable) == reflect.TypeOf(rightComparable) {
		return reflect.DeepEqual(leftComparable, rightComparable), nil
	}
	return fmt.Sprint(leftComparable) == fmt.Sprint(rightComparable), nil
}

func expressionValue(raw string, lookup valueLookup) (any, bool) {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "nil":
		return nil, true
	case raw == "true":
		return true, true
	case raw == "false":
		return false, true
	case strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") && len(raw) >= 2:
		return strings.TrimSuffix(strings.TrimPrefix(raw, "'"), "'"), true
	case strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\""):
		value, err := strconv.Unquote(raw)
		if err == nil {
			return value, true
		}
	case raw != "":
		value, ok := lookup(raw)
		return value, ok
	}
	return nil, false
}

func comparableValue(value any) any {
	switch item := value.(type) {
	case []byte:
		return string(item)
	default:
		return item
	}
}

func truthy(value any) bool {
	if isNilValue(value) {
		return false
	}
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return item != ""
	case []byte:
		return len(item) > 0
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len() > 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0
	default:
		return true
	}
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
