package orm

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"goark.dev/orm/internal/jsoncodec"
)

// TypeHandler 负责 Go 值与数据库值之间的双向转换。
type TypeHandler interface {
	ToDB(ctx context.Context, value any) (any, error)
	FromDB(ctx context.Context, value any, target any) error
}

// NewJSONTypeHandler 创建 JSON 字段转换器。
func NewJSONTypeHandler() TypeHandler {
	return jsonTypeHandler{}
}

// NewTimeTypeHandler 创建时间字段转换器。
func NewTimeTypeHandler() TypeHandler {
	return timeTypeHandler{}
}

// NewDecimalTypeHandler 创建无外部依赖的 Decimal 字段转换器。
func NewDecimalTypeHandler() TypeHandler {
	return decimalTypeHandler{}
}

func defaultTypeHandlers() map[string]TypeHandler {
	return map[string]TypeHandler{
		"json":    NewJSONTypeHandler(),
		"time":    NewTimeTypeHandler(),
		"decimal": NewDecimalTypeHandler(),
		"string":  NewStringTypeHandler(),
		"bool":    NewBoolTypeHandler(),
		"bytes":   NewBytesTypeHandler(),
	}
}

type jsonTypeHandler struct{}

func (jsonTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return typed, nil
	case string:
		return typed, nil
	default:
		data, err := jsoncodec.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal json failed: %w", err)
		}
		return data, nil
	}
}

func (jsonTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	if target == nil {
		return fmt.Errorf("json target is nil")
	}
	if value == nil {
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		if err := jsoncodec.UnmarshalString(typed, target); err != nil {
			return fmt.Errorf("unmarshal json failed: %w", err)
		}
		return nil
	default:
		encoded, err := jsoncodec.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal database json value failed: %w", err)
		}
		data = encoded
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := jsoncodec.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal json failed: %w", err)
	}
	return nil
}

type timeTypeHandler struct{}

func (timeTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	return value, nil
}

func (timeTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	if target == nil {
		return fmt.Errorf("time target is nil")
	}
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case time.Time:
		return assignTypeHandlerValue(target, typed)
	case string:
		parsed, err := parseTimeString(typed)
		if err != nil {
			return err
		}
		return assignTypeHandlerValue(target, parsed)
	case []byte:
		parsed, err := parseTimeString(string(typed))
		if err != nil {
			return err
		}
		return assignTypeHandlerValue(target, parsed)
	default:
		return assignTypeHandlerValue(target, value)
	}
}

func parseTimeString(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time %q failed", value)
}

type decimalTypeHandler struct{}

func (decimalTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	_ = ctx
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case string, []byte:
		return typed, nil
	case fmt.Stringer:
		return typed.String(), nil
	default:
		return value, nil
	}
}

func (decimalTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	_ = ctx
	if target == nil {
		return fmt.Errorf("decimal target is nil")
	}
	if value == nil {
		return nil
	}
	text := decimalString(value)
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return fmt.Errorf("decimal target must be non-nil pointer")
	}
	field := targetValue.Elem()
	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(text)
		return nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(text, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("parse decimal %q failed: %w", text, err)
		}
		field.SetFloat(parsed)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(text, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("parse decimal %q failed: %w", text, err)
		}
		field.SetInt(parsed)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(text, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("parse decimal %q failed: %w", text, err)
		}
		field.SetUint(parsed)
		return nil
	default:
		return assignTypeHandlerValue(target, value)
	}
}

func decimalString(value any) string {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func assignTypeHandlerValue(target any, value any) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return fmt.Errorf("type-handler target must be non-nil pointer")
	}
	return setReflectField(targetValue.Elem(), value)
}
