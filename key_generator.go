package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// executeSelectKey 执行当前写语句内联的 selectKey，并返回数据库生成的主键值。
func (s *SQLSession) executeSelectKey(ctx context.Context, parent StatementMeta, args NamedArgs) (any, error) {
	selectKey := parent.SelectKey
	if !selectKey.Enabled {
		return nil, fmt.Errorf("goark-orm: statement %s selectKey is disabled", parent.FullName)
	}
	meta := StatementMeta{
		ID:            parent.ID + "!selectKey",
		Namespace:     parent.Namespace,
		FullName:      parent.FullName + "!selectKey",
		Command:       StatementCommandSelect,
		Source:        parent.Source,
		SQL:           selectKey.SQL,
		ResultType:    selectKey.ResultType,
		ParameterType: parent.ParameterType,
		DynamicSQL:    selectKey.DynamicSQL,
	}
	compiled, err := s.compileSelectKey(ctx, meta, args)
	if err != nil {
		return nil, err
	}
	rows, err := s.querySQL(ctx, compiled)
	if err != nil {
		return nil, executorFailure(meta, "query selectKey", compiled, err)
	}
	defer rows.Close()

	dest := selectKeyDestination(selectKey.ResultType)
	if err := s.resultSetHandler.ScanOne(ctx, rows, meta, dest); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("goark-orm: selectKey %s returned no rows", parent.FullName)
		}
		return nil, mappingFailure(meta, err)
	}
	return reflect.Indirect(reflect.ValueOf(dest)).Interface(), nil
}

func (s *SQLSession) compileSelectKey(ctx context.Context, meta StatementMeta, args NamedArgs) (CompiledSQL, error) {
	sqlText := meta.SQL
	renderArgs := args
	if len(meta.DynamicSQL) > 0 {
		rendered, err := RenderDynamicSQL(meta.DynamicSQL, args)
		if err != nil {
			return CompiledSQL{}, bindingFailure(meta.FullName, "render dynamic selectKey", err)
		}
		sqlText = rendered.SQL
		renderArgs = rendered.Args
	}
	return s.statementHandler.Compile(ctx, &StatementRuntime{
		Meta:    meta,
		SQL:     sqlText,
		Args:    copyNamedArgs(renderArgs),
		Dialect: s.Dialect(),
	})
}

func selectKeyDestination(resultType string) any {
	switch normalizeTypeIdentifier(resultType) {
	case "int":
		var out int
		return &out
	case "int8":
		var out int8
		return &out
	case "int16":
		var out int16
		return &out
	case "int32":
		var out int32
		return &out
	case "int64", "":
		var out int64
		return &out
	case "uint":
		var out uint
		return &out
	case "uint8":
		var out uint8
		return &out
	case "uint16":
		var out uint16
		return &out
	case "uint32":
		var out uint32
		return &out
	case "uint64":
		var out uint64
		return &out
	case "string":
		var out string
		return &out
	case "bool":
		var out bool
		return &out
	case "float32":
		var out float32
		return &out
	case "float64":
		var out float64
		return &out
	default:
		var out any
		return &out
	}
}

func normalizeSelectKeyOrder(order SelectKeyOrder) SelectKeyOrder {
	switch SelectKeyOrder(strings.ToUpper(strings.TrimSpace(string(order)))) {
	case SelectKeyOrderBefore:
		return SelectKeyOrderBefore
	default:
		return SelectKeyOrderAfter
	}
}

func selectKeyProperty(parent StatementMeta) string {
	if property := strings.TrimSpace(parent.SelectKey.KeyProperty); property != "" {
		return property
	}
	return strings.TrimSpace(parent.KeyProperty)
}

func applyKeyProperty(args NamedArgs, keyProperty string, value any) error {
	keyProperty = strings.TrimSpace(keyProperty)
	if keyProperty == "" {
		return nil
	}
	if args == nil {
		return fmt.Errorf("goark-orm: keyProperty %s requires named arguments", keyProperty)
	}
	parts := propertyPath(keyProperty)
	if len(parts) == 0 {
		return nil
	}
	args[parts[len(parts)-1]] = value
	args[keyProperty] = value

	if len(parts) > 1 {
		if target, ok := args[parts[0]]; ok {
			if updated, err := setPropertyPath(target, parts[1:], value); err != nil || updated {
				return err
			}
		}
		return nil
	}
	for _, candidate := range args {
		updated, err := setPropertyPath(candidate, parts, value)
		if err != nil {
			return err
		}
		if updated {
			return nil
		}
	}
	return nil
}

func propertyPath(path string) []string {
	raw := strings.Split(path, ".")
	parts := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			parts = append(parts, item)
		}
	}
	return parts
}

func setPropertyPath(target any, parts []string, value any) (bool, error) {
	if len(parts) == 0 || target == nil {
		return false, nil
	}
	current := reflect.ValueOf(target)
	for current.Kind() == reflect.Interface || current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return false, nil
		}
		current = current.Elem()
	}
	if current.Kind() != reflect.Struct {
		return false, nil
	}
	field, ok := exportedFieldByProperty(current, parts[0])
	if !ok {
		return false, nil
	}
	if len(parts) > 1 {
		if field.Kind() == reflect.Pointer && field.IsNil() && field.CanSet() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setPropertyPath(field.Addr().Interface(), parts[1:], value)
	}
	if !field.CanSet() {
		return false, nil
	}
	if err := setReflectField(field, value); err != nil {
		return false, err
	}
	return true, nil
}

func exportedFieldByProperty(value reflect.Value, property string) (reflect.Value, bool) {
	key := normalizeColumnKey(property)
	typ := value.Type()
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
		if field.Name == property || normalizeColumnKey(field.Name) == key {
			return value.Field(index), true
		}
	}
	return reflect.Value{}, false
}

func keyAsInt64(value any) (int64, bool) {
	if value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		if uint64(typed) > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(typed), true
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(typed), true
	case []byte:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		source := reflect.ValueOf(value)
		if source.IsValid() && source.Type().ConvertibleTo(reflect.TypeOf(int64(0))) {
			return source.Convert(reflect.TypeOf(int64(0))).Int(), true
		}
		return 0, false
	}
}
