package orm

import (
	"context"
	"fmt"
	"strings"
)

// SelectFieldValues 按类型化字段查询单列值列表。
func SelectFieldValues[T any, ID any, V any](ctx context.Context, mapper *BaseMapper[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) ([]V, error) {
	if mapper == nil {
		return nil, fmt.Errorf("goark-orm: base mapper is nil")
	}
	sqlText, args, err := mapper.selectFieldSQL(field.Field(), wrapper, true)
	if err != nil {
		return nil, err
	}
	var out []V
	if err := mapper.session.QueryStatement(ctx, mapper.statement("SelectFieldValues", StatementCommandSelect, sqlText), args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SelectFieldValue 按类型化字段查询单个单列值，多行时返回错误。
func SelectFieldValue[T any, ID any, V any](ctx context.Context, mapper *BaseMapper[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	var zero V
	if mapper == nil {
		return zero, fmt.Errorf("goark-orm: base mapper is nil")
	}
	sqlText, args, err := mapper.selectFieldSQL(field.Field(), wrapper, true)
	if err != nil {
		return zero, err
	}
	var out V
	if err := mapper.session.QueryOneStatement(ctx, mapper.statement("SelectFieldValue", StatementCommandSelect, sqlText), args, &out); err != nil {
		return zero, err
	}
	return out, nil
}

// SelectFirstFieldValue 按类型化字段查询第一条单列值，无记录时返回 sql.ErrNoRows。
func SelectFirstFieldValue[T any, ID any, V any](ctx context.Context, mapper *BaseMapper[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	var zero V
	if mapper == nil {
		return zero, fmt.Errorf("goark-orm: base mapper is nil")
	}
	sqlText, args, err := mapper.selectFirstFieldSQL(field.Field(), wrapper)
	if err != nil {
		return zero, err
	}
	var out V
	if err := mapper.session.QueryOneStatement(ctx, mapper.statement("SelectFirstFieldValue", StatementCommandSelect, sqlText), args, &out); err != nil {
		return zero, err
	}
	return out, nil
}

// ListFieldValues 按类型化字段查询单列值列表。
func ListFieldValues[T any, ID any, V any](ctx context.Context, service *Service[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) ([]V, error) {
	mapper, err := requireServiceMapper(service)
	if err != nil {
		return nil, err
	}
	return SelectFieldValues(ctx, mapper, field, wrapper)
}

// GetFieldValue 按类型化字段查询单个单列值，多行时返回错误。
func GetFieldValue[T any, ID any, V any](ctx context.Context, service *Service[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	mapper, err := requireServiceMapper(service)
	if err != nil {
		var zero V
		return zero, err
	}
	return SelectFieldValue(ctx, mapper, field, wrapper)
}

// GetFirstFieldValue 按类型化字段查询第一条单列值。
func GetFirstFieldValue[T any, ID any, V any](ctx context.Context, service *Service[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	mapper, err := requireServiceMapper(service)
	if err != nil {
		var zero V
		return zero, err
	}
	return SelectFirstFieldValue(ctx, mapper, field, wrapper)
}

// SelectIDs 按主键类型查询 ID 列表。
func (m *BaseMapper[T, ID]) SelectIDs(ctx context.Context, wrapper *QueryWrapper[T]) ([]ID, error) {
	if m == nil {
		return nil, fmt.Errorf("goark-orm: base mapper is nil")
	}
	return SelectFieldValues(ctx, m, NewTypedField[T, ID](m.primary.ColumnName), wrapper)
}

// ListIDs 按主键类型查询 ID 列表。
func (s *Service[T, ID]) ListIDs(ctx context.Context, wrapper *QueryWrapper[T]) ([]ID, error) {
	mapper, err := s.requireMapper()
	if err != nil {
		return nil, err
	}
	return mapper.SelectIDs(ctx, wrapper)
}

// IDs 执行链式 ID 列表查询。
func (c *QueryChain[T, ID]) IDs(ctx context.Context) ([]ID, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("goark-orm: query chain service is nil")
	}
	return c.service.ListIDs(ctx, c.wrapper)
}

func requireServiceMapper[T any, ID any](service *Service[T, ID]) (*BaseMapper[T, ID], error) {
	if service == nil {
		return nil, fmt.Errorf("goark-orm: service is nil")
	}
	return service.requireMapper()
}

func (m *BaseMapper[T, ID]) selectFieldSQL(field Field[T], wrapper *QueryWrapper[T], includeOrder bool) (string, NamedArgs, error) {
	projection, err := m.singleFieldProjection(field)
	if err != nil {
		return "", nil, err
	}
	sqlText, args, _, err := m.selectProjectionSQL(projection, wrapper, includeOrder, 0)
	return sqlText, args, err
}

func (m *BaseMapper[T, ID]) selectFirstFieldSQL(field Field[T], wrapper *QueryWrapper[T]) (string, NamedArgs, error) {
	projection, err := m.singleFieldProjection(field)
	if err != nil {
		return "", nil, err
	}
	sqlText, args, next, err := m.selectProjectionSQL(projection, wrapper, true, 0)
	if err != nil {
		return "", nil, err
	}
	limitName := wrapperArgName(next)
	offsetName := wrapperArgName(next + 1)
	args[limitName] = int64(1)
	args[offsetName] = int64(0)
	return limitOffsetSQL(m.dialect, sqlText, "#{"+limitName+"}", "#{"+offsetName+"}"), args, nil
}

func (m *BaseMapper[T, ID]) singleFieldProjection(field Field[T]) (string, error) {
	if m == nil {
		return "", fmt.Errorf("goark-orm: base mapper is nil")
	}
	column := strings.TrimSpace(field.Column)
	if column == "" {
		return "", fmt.Errorf("goark-orm: typed field column is empty")
	}
	if _, ok := m.columnByName(column); !ok {
		return "", fmt.Errorf("goark-orm: typed field column %q is not mapped by entity %s", column, m.entity.TypeName)
	}
	return quoteIdentifierPath(m.dialect, column)
}
