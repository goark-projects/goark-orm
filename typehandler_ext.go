package orm

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// TypeHandlerAdapter 用函数适配自定义 TypeHandler。
type TypeHandlerAdapter struct {
	ToDBFunc   func(context.Context, any) (any, error)
	FromDBFunc func(context.Context, any, any) error
}

// NewTypeHandler 使用函数创建 TypeHandler，nil 函数会采用保守默认行为。
func NewTypeHandler(toDB func(context.Context, any) (any, error), fromDB func(context.Context, any, any) error) TypeHandler {
	return TypeHandlerAdapter{ToDBFunc: toDB, FromDBFunc: fromDB}
}

// ToDB 执行入库转换。
func (h TypeHandlerAdapter) ToDB(ctx context.Context, value any) (any, error) {
	if h.ToDBFunc == nil {
		return value, nil
	}
	return h.ToDBFunc(ctx, value)
}

// FromDB 执行出库转换。
func (h TypeHandlerAdapter) FromDB(ctx context.Context, value any, target any) error {
	if h.FromDBFunc == nil {
		return assignTypeHandlerValue(target, value)
	}
	return h.FromDBFunc(ctx, value, target)
}

// NewStringTypeHandler 创建字符串字段转换器。
func NewStringTypeHandler() TypeHandler {
	return stringTypeHandler{}
}

// NewBoolTypeHandler 创建布尔字段转换器。
func NewBoolTypeHandler() TypeHandler {
	return boolTypeHandler{}
}

// NewBytesTypeHandler 创建字节切片字段转换器。
func NewBytesTypeHandler() TypeHandler {
	return bytesTypeHandler{}
}

type stringTypeHandler struct{}

func (stringTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	case fmt.Stringer:
		return typed.String(), nil
	default:
		return fmt.Sprint(value), nil
	}
}

func (stringTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	if target == nil {
		return fmt.Errorf("string target is nil")
	}
	if value == nil {
		return assignTypeHandlerValue(target, "")
	}
	switch typed := value.(type) {
	case string:
		return assignTypeHandlerValue(target, typed)
	case []byte:
		return assignTypeHandlerValue(target, string(typed))
	case fmt.Stringer:
		return assignTypeHandlerValue(target, typed.String())
	default:
		return assignTypeHandlerValue(target, fmt.Sprint(value))
	}
}

type boolTypeHandler struct{}

func (boolTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return parseBoolTypeHandlerValue(typed)
	case []byte:
		return parseBoolTypeHandlerValue(string(typed))
	default:
		parsed, ok := boolFromNumeric(value)
		if ok {
			return parsed, nil
		}
		return value, nil
	}
}

func (boolTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	if target == nil {
		return fmt.Errorf("bool target is nil")
	}
	if value == nil {
		return assignTypeHandlerValue(target, false)
	}
	switch typed := value.(type) {
	case bool:
		return assignTypeHandlerValue(target, typed)
	case string:
		parsed, err := parseBoolTypeHandlerValue(typed)
		if err != nil {
			return err
		}
		return assignTypeHandlerValue(target, parsed)
	case []byte:
		parsed, err := parseBoolTypeHandlerValue(string(typed))
		if err != nil {
			return err
		}
		return assignTypeHandlerValue(target, parsed)
	default:
		parsed, ok := boolFromNumeric(value)
		if !ok {
			return assignTypeHandlerValue(target, value)
		}
		return assignTypeHandlerValue(target, parsed)
	}
}

type bytesTypeHandler struct{}

func (bytesTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []byte:
		return append([]byte(nil), typed...), nil
	case string:
		return []byte(typed), nil
	default:
		return value, nil
	}
}

func (bytesTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	if target == nil {
		return fmt.Errorf("bytes target is nil")
	}
	if value == nil {
		return assignTypeHandlerValue(target, []byte(nil))
	}
	switch typed := value.(type) {
	case []byte:
		return assignTypeHandlerValue(target, append([]byte(nil), typed...))
	case string:
		return assignTypeHandlerValue(target, []byte(typed))
	default:
		return assignTypeHandlerValue(target, value)
	}
}

func parseBoolTypeHandlerValue(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("parse bool %q failed", value)
	}
}

func boolFromNumeric(value any) (bool, bool) {
	reflectValue := reflect.ValueOf(value)
	if !reflectValue.IsValid() {
		return false, false
	}
	switch reflectValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflectValue.Int() != 0, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflectValue.Uint() != 0, true
	case reflect.Float32, reflect.Float64:
		return reflectValue.Float() != 0, true
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return false, false
		}
		if parsed, err := strconv.ParseFloat(text, 64); err == nil {
			return parsed != 0, true
		}
		return false, false
	}
}

func sortedTypeHandlerNames(handlers map[string]TypeHandler) []string {
	names := make([]string, 0, len(handlers))
	for name := range handlers {
		names = append(names, strings.TrimSpace(name))
	}
	sort.Strings(names)
	return names
}
