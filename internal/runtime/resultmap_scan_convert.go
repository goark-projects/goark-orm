package runtime

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// setReflectScalarFromText 处理部分驱动把数值列返回为文本的情况。
func setReflectScalarFromText(field reflect.Value, value any) (bool, error) {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case []byte:
		text = string(typed)
	default:
		return false, nil
	}
	text = strings.TrimSpace(text)
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(text, 10, field.Type().Bits())
		if err != nil {
			return true, scalarConversionError(value, field.Type(), err)
		}
		field.SetInt(parsed)
		return true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		parsed, err := strconv.ParseUint(text, 10, field.Type().Bits())
		if err != nil {
			return true, scalarConversionError(value, field.Type(), err)
		}
		field.SetUint(parsed)
		return true, nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(text, field.Type().Bits())
		if err != nil {
			return true, scalarConversionError(value, field.Type(), err)
		}
		field.SetFloat(parsed)
		return true, nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(text)
		if err != nil {
			return true, scalarConversionError(value, field.Type(), err)
		}
		field.SetBool(parsed)
		return true, nil
	default:
		return false, nil
	}
}

func scalarConversionError(value any, target reflect.Type, cause error) error {
	return fmt.Errorf("goark-orm: database value %T cannot parse as field %s: %w", value, target, cause)
}
