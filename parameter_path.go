package orm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type parameterPath struct {
	segments []parameterPathSegment
}

type parameterPathSegment struct {
	name    string
	indexes []parameterPathIndex
}

type parameterPathIndex struct {
	key     string
	numeric int
}

func (p parameterPath) String() string {
	if len(p.segments) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, segment := range p.segments {
		if index > 0 {
			builder.WriteByte('.')
		}
		builder.WriteString(segment.name)
		for _, item := range segment.indexes {
			if item.key == "" {
				builder.WriteByte('[')
				builder.WriteString(strconv.Itoa(item.numeric))
				builder.WriteByte(']')
				continue
			}
			builder.WriteByte('[')
			builder.WriteString(strconv.Quote(item.key))
			builder.WriteByte(']')
		}
	}
	return builder.String()
}

// resolveNamedArg 按 MyBatis 参数路径语义解析命名参数，禁止方法调用等不可控行为。
func resolveNamedArg(args NamedArgs, raw string) (any, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, fmt.Errorf("goark-orm: SQL parameter path is empty")
	}
	if args == nil {
		return nil, false, nil
	}
	if value, ok := args[raw]; ok {
		return value, true, nil
	}
	path, err := parseParameterPath(raw)
	if err != nil {
		return nil, false, err
	}
	if len(path.segments) == 0 {
		return nil, false, nil
	}
	root := path.segments[0]
	value, ok := args[root.name]
	if !ok {
		return nil, false, nil
	}
	resolved, ok, err := applyParameterIndexes(value, root.indexes)
	if err != nil || !ok {
		return nil, ok, err
	}
	for _, segment := range path.segments[1:] {
		resolved, ok, err = parameterProperty(resolved, segment.name)
		if err != nil || !ok {
			return nil, ok, err
		}
		resolved, ok, err = applyParameterIndexes(resolved, segment.indexes)
		if err != nil || !ok {
			return nil, ok, err
		}
	}
	return resolved, true, nil
}

func parseParameterPath(raw string) (parameterPath, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parameterPath{}, fmt.Errorf("goark-orm: SQL parameter path is empty")
	}
	segments := make([]parameterPathSegment, 0, 2)
	index := 0
	for index < len(raw) {
		name, next := scanPathIdentifier(raw, index)
		if name == "" {
			return parameterPath{}, fmt.Errorf("goark-orm: invalid SQL parameter path %q", raw)
		}
		segment := parameterPathSegment{name: name}
		index = next
		for index < len(raw) && raw[index] == '[' {
			item, next, err := scanPathIndex(raw, index)
			if err != nil {
				return parameterPath{}, err
			}
			segment.indexes = append(segment.indexes, item)
			index = next
		}
		segments = append(segments, segment)
		if index >= len(raw) {
			break
		}
		if raw[index] != '.' {
			return parameterPath{}, fmt.Errorf("goark-orm: invalid SQL parameter path %q", raw)
		}
		index++
		if index >= len(raw) {
			return parameterPath{}, fmt.Errorf("goark-orm: invalid SQL parameter path %q", raw)
		}
	}
	return parameterPath{segments: segments}, nil
}

func scanPathIdentifier(raw string, start int) (string, int) {
	if start >= len(raw) || !isPathIdentifierStart(raw[start]) {
		return "", start
	}
	index := start + 1
	for index < len(raw) && isPathIdentifierPart(raw[index]) {
		index++
	}
	return raw[start:index], index
}

func scanPathIndex(raw string, start int) (parameterPathIndex, int, error) {
	end := strings.IndexByte(raw[start:], ']')
	if end < 0 {
		return parameterPathIndex{}, start, fmt.Errorf("goark-orm: invalid SQL parameter path %q", raw)
	}
	end += start
	content := strings.TrimSpace(raw[start+1 : end])
	if content == "" {
		return parameterPathIndex{}, start, fmt.Errorf("goark-orm: empty SQL parameter index in %q", raw)
	}
	if parsed, err := strconv.Atoi(content); err == nil {
		if parsed < 0 {
			return parameterPathIndex{}, start, fmt.Errorf("goark-orm: negative SQL parameter index in %q", raw)
		}
		return parameterPathIndex{numeric: parsed}, end + 1, nil
	}
	if unquoted, ok := unquotePathIndex(content); ok {
		return parameterPathIndex{key: unquoted}, end + 1, nil
	}
	if !validIdentifierPart(content) {
		return parameterPathIndex{}, start, fmt.Errorf("goark-orm: invalid SQL parameter index %q", content)
	}
	return parameterPathIndex{key: content}, end + 1, nil
}

func unquotePathIndex(content string) (string, bool) {
	if len(content) < 2 {
		return "", false
	}
	quote := content[0]
	if quote != '\'' && quote != '"' || content[len(content)-1] != quote {
		return "", false
	}
	value := content[1 : len(content)-1]
	if quote == '"' {
		unquoted, err := strconv.Unquote(content)
		if err == nil {
			return unquoted, true
		}
		return "", false
	}
	if strings.ContainsAny(value, `'\`) {
		return "", false
	}
	return value, true
}

func isPathIdentifierStart(ch byte) bool {
	return ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isPathIdentifierPart(ch byte) bool {
	return isPathIdentifierStart(ch) || (ch >= '0' && ch <= '9')
}

func applyParameterIndexes(value any, indexes []parameterPathIndex) (any, bool, error) {
	resolved := value
	for _, index := range indexes {
		next, ok, err := parameterIndex(resolved, index)
		if err != nil || !ok {
			return nil, ok, err
		}
		resolved = next
	}
	return resolved, true, nil
}

func parameterProperty(value any, property string) (any, bool, error) {
	if value == nil {
		return nil, true, nil
	}
	current := reflect.ValueOf(value)
	current, nilValue := dereferenceParameterValue(current)
	if nilValue {
		return nil, true, nil
	}
	switch current.Kind() {
	case reflect.Struct:
		field, ok := exportedFieldByProperty(current, property)
		if !ok {
			return nil, false, nil
		}
		return field.Interface(), true, nil
	case reflect.Map:
		return mapParameterValue(current, property)
	default:
		return nil, false, nil
	}
}

func parameterIndex(value any, index parameterPathIndex) (any, bool, error) {
	if value == nil {
		return nil, true, nil
	}
	current := reflect.ValueOf(value)
	current, nilValue := dereferenceParameterValue(current)
	if nilValue {
		return nil, true, nil
	}
	switch current.Kind() {
	case reflect.Array, reflect.Slice:
		if index.key != "" {
			return nil, false, fmt.Errorf("goark-orm: indexed SQL parameter requires numeric index")
		}
		if index.numeric < 0 || index.numeric >= current.Len() {
			return nil, false, nil
		}
		return current.Index(index.numeric).Interface(), true, nil
	case reflect.Map:
		if index.key == "" {
			return mapParameterValueByReflectKey(current, reflect.ValueOf(index.numeric))
		}
		return mapParameterValue(current, index.key)
	default:
		return nil, false, nil
	}
}

func dereferenceParameterValue(value reflect.Value) (reflect.Value, bool) {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}, true
		}
		value = value.Elem()
	}
	return value, false
}

func mapParameterValue(value reflect.Value, key string) (any, bool, error) {
	if value.Type().Key().Kind() == reflect.String {
		return mapParameterValueByReflectKey(value, reflect.ValueOf(key))
	}
	return mapParameterValueByReflectKey(value, reflect.ValueOf(key))
}

func mapParameterValueByReflectKey(value reflect.Value, key reflect.Value) (any, bool, error) {
	keyType := value.Type().Key()
	if !key.IsValid() {
		return nil, false, nil
	}
	if !key.Type().AssignableTo(keyType) {
		if !key.Type().ConvertibleTo(keyType) {
			return nil, false, nil
		}
		key = key.Convert(keyType)
	}
	item := value.MapIndex(key)
	if !item.IsValid() {
		return nil, false, nil
	}
	if item.Kind() == reflect.Interface && !item.IsNil() {
		item = item.Elem()
	}
	return item.Interface(), true, nil
}

func parameterPropertyAlias(fieldName string) string {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return ""
	}
	leadingUpper := 0
	for leadingUpper < len(fieldName) {
		ch := fieldName[leadingUpper]
		if ch < 'A' || ch > 'Z' {
			break
		}
		leadingUpper++
	}
	switch {
	case leadingUpper == 0:
		return fieldName
	case leadingUpper == len(fieldName):
		return strings.ToLower(fieldName)
	case leadingUpper > 1:
		leadingUpper--
	}
	return strings.ToLower(fieldName[:leadingUpper]) + fieldName[leadingUpper:]
}
