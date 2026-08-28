package runtime

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// MetaObjectHandler 按 INSERT/UPDATE 时机填充实体字段或命名参数。
type MetaObjectHandler interface {
	InsertFill(ctx context.Context, meta *MetaObject) error
	UpdateFill(ctx context.Context, meta *MetaObject) error
}

// MetaObjectHandlerFuncs 使用函数组合构造 MetaObjectHandler。
type MetaObjectHandlerFuncs struct {
	Insert func(ctx context.Context, meta *MetaObject) error
	Update func(ctx context.Context, meta *MetaObject) error
}

// InsertFill 执行 INSERT 填充函数。
func (h MetaObjectHandlerFuncs) InsertFill(ctx context.Context, meta *MetaObject) error {
	if h.Insert == nil {
		return nil
	}
	return h.Insert(ctx, meta)
}

// UpdateFill 执行 UPDATE 填充函数。
func (h MetaObjectHandlerFuncs) UpdateFill(ctx context.Context, meta *MetaObject) error {
	if h.Update == nil {
		return nil
	}
	return h.Update(ctx, meta)
}

// MetaObject 暴露实体字段和 SQL 命名参数的统一填充视图。
type MetaObject struct {
	entity EntityMeta
	value  reflect.Value
	args   NamedArgs
}

// NewMetaObject 基于实体指针创建可直接填充的元对象。
func NewMetaObject(entity EntityMeta, target any) (*MetaObject, error) {
	value, err := entityStructValue(target)
	if err != nil {
		return nil, err
	}
	return &MetaObject{
		entity: copyEntityMeta(entity),
		value:  value,
	}, nil
}

func newMetaObject(entity EntityMeta, value reflect.Value, args NamedArgs) *MetaObject {
	return &MetaObject{
		entity: copyEntityMeta(entity),
		value:  value,
		args:   args,
	}
}

// Entity 返回当前填充目标的实体元数据快照。
func (m *MetaObject) Entity() EntityMeta {
	if m == nil {
		return EntityMeta{}
	}
	return copyEntityMeta(m.entity)
}

// Args 返回当前命名参数快照。
func (m *MetaObject) Args() NamedArgs {
	if m == nil {
		return nil
	}
	return copyNamedArgs(m.args)
}

// HasField 判断实体元数据中是否存在指定字段或列。
func (m *MetaObject) HasField(name string) bool {
	_, ok := m.column(name)
	return ok
}

// FieldValue 读取字段当前值，优先读取实体指针，其次读取命名参数。
func (m *MetaObject) FieldValue(name string) (any, bool, error) {
	column, ok := m.column(name)
	if !ok {
		return nil, false, fmt.Errorf("goark-orm: meta object field %q is not mapped", name)
	}
	if m != nil && m.value.IsValid() {
		field := m.value.FieldByName(column.FieldName)
		if !field.IsValid() {
			return nil, false, fmt.Errorf("goark-orm: entity %s missing field %s", m.value.Type().Name(), column.FieldName)
		}
		if !field.CanInterface() {
			return nil, false, fmt.Errorf("goark-orm: entity field %s.%s is not exported", m.value.Type().Name(), column.FieldName)
		}
		return field.Interface(), true, nil
	}
	if value, ok := metaObjectArgValue(m.args, column); ok {
		return value, true, nil
	}
	return nil, false, nil
}

// SetField 覆盖写入指定字段，并同步命名参数中的字段名和 lower-camel 别名。
func (m *MetaObject) SetField(name string, value any) error {
	return m.setField(name, value, true)
}

// SetFieldIfZero 仅在当前字段为零值时写入指定字段。
func (m *MetaObject) SetFieldIfZero(name string, value any) error {
	return m.setField(name, value, false)
}

// StrictInsertFill 在字段允许 INSERT 填充且当前为零值时写入。
func (m *MetaObject) StrictInsertFill(name string, value any) error {
	column, ok := m.column(name)
	if !ok {
		return fmt.Errorf("goark-orm: meta object field %q is not mapped", name)
	}
	if !fieldFillAllowsInsert(effectiveFieldFill(column)) {
		return nil
	}
	return m.setColumn(column, value, false)
}

// StrictUpdateFill 在字段允许 UPDATE 填充时覆盖写入。
func (m *MetaObject) StrictUpdateFill(name string, value any) error {
	column, ok := m.column(name)
	if !ok {
		return fmt.Errorf("goark-orm: meta object field %q is not mapped", name)
	}
	if !fieldFillAllowsUpdate(effectiveFieldFill(column)) {
		return nil
	}
	return m.setColumn(column, value, true)
}

func (m *MetaObject) setField(name string, value any, overwrite bool) error {
	column, ok := m.column(name)
	if !ok {
		return fmt.Errorf("goark-orm: meta object field %q is not mapped", name)
	}
	return m.setColumn(column, value, overwrite)
}

func (m *MetaObject) setColumn(column ColumnMeta, value any, overwrite bool) error {
	if m == nil {
		return fmt.Errorf("goark-orm: meta object is nil")
	}
	current, ok, err := m.FieldValue(column.FieldName)
	if err != nil {
		return err
	}
	if !overwrite && ok && !isZeroValue(current) {
		return nil
	}
	if m.value.IsValid() {
		field := m.value.FieldByName(column.FieldName)
		if !field.IsValid() {
			return fmt.Errorf("goark-orm: entity %s missing field %s", m.value.Type().Name(), column.FieldName)
		}
		if !field.CanSet() {
			return fmt.Errorf("goark-orm: entity field %s.%s is not settable", m.value.Type().Name(), column.FieldName)
		}
		if err := setReflectField(field, value); err != nil {
			return err
		}
	}
	if m.args != nil {
		setMetaObjectArgs(m.args, column, value)
	}
	if !m.value.IsValid() && m.args == nil {
		return fmt.Errorf("goark-orm: meta object has no writable target")
	}
	return nil
}

func (m *MetaObject) column(name string) (ColumnMeta, bool) {
	if m == nil {
		return ColumnMeta{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ColumnMeta{}, false
	}
	key := normalizeColumnKey(name)
	for _, column := range m.entity.Columns {
		if column.FieldName == name || column.ColumnName == name || parameterPropertyAlias(column.FieldName) == name {
			return column, true
		}
		if normalizeColumnKey(column.FieldName) == key || normalizeColumnKey(column.ColumnName) == key {
			return column, true
		}
	}
	return ColumnMeta{}, false
}

func applyMetaObjectHandler(ctx context.Context, handler MetaObjectHandler, command StatementCommand, entity EntityMeta, value reflect.Value, args NamedArgs) error {
	if handler == nil {
		return nil
	}
	meta := newMetaObject(entity, value, args)
	switch command {
	case StatementCommandInsert:
		return handler.InsertFill(ctx, meta)
	case StatementCommandUpdate:
		return handler.UpdateFill(ctx, meta)
	default:
		return nil
	}
}

func metaObjectArgValue(args NamedArgs, column ColumnMeta) (any, bool) {
	if args == nil {
		return nil, false
	}
	for _, name := range metaObjectArgNames(column) {
		if value, ok := args[name]; ok {
			return value, true
		}
	}
	return nil, false
}

func setMetaObjectArgs(args NamedArgs, column ColumnMeta, value any) {
	for _, name := range metaObjectArgNames(column) {
		args[name] = value
	}
}

func metaObjectArgNames(column ColumnMeta) []string {
	names := []string{column.FieldName}
	if alias := parameterPropertyAlias(column.FieldName); alias != "" && alias != column.FieldName {
		names = append(names, alias)
	}
	if column.ColumnName != "" && column.ColumnName != column.FieldName {
		names = append(names, column.ColumnName)
	}
	return names
}

func effectiveFieldFill(column ColumnMeta) FieldFill {
	if column.Fill != FieldFillDefault {
		return column.Fill
	}
	switch {
	case column.UpdatedAt:
		return FieldFillInsertUpdate
	case column.CreatedAt:
		return FieldFillInsert
	default:
		return FieldFillDefault
	}
}

func fieldFillAllowsInsert(fill FieldFill) bool {
	return fill == FieldFillInsert || fill == FieldFillInsertUpdate
}

func fieldFillAllowsUpdate(fill FieldFill) bool {
	return fill == FieldFillUpdate || fill == FieldFillInsertUpdate
}

// ParseFieldFill 解析字段填充策略，兼容大小写、短横线和下划线。
func ParseFieldFill(value string) (FieldFill, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "", "DEFAULT", "NONE":
		return FieldFillDefault, nil
	case string(FieldFillInsert):
		return FieldFillInsert, nil
	case string(FieldFillUpdate):
		return FieldFillUpdate, nil
	case string(FieldFillInsertUpdate):
		return FieldFillInsertUpdate, nil
	default:
		return "", fmt.Errorf("goark-orm: unsupported field fill %q", value)
	}
}
