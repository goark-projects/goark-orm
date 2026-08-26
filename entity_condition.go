package orm

import (
	"context"
	"fmt"
	"strings"
)

const entityConditionArgPrefix = "__goark_orm_entity_"

// EntityQueryWrapper 按实体字段值构造查询条件，字段 whereStrategy 决定是否参与 WHERE。
func (m *BaseMapper[T, ID]) EntityQueryWrapper(entity *T) (*QueryWrapper[T], error) {
	value, err := entityStructValue(entity)
	if err != nil {
		return nil, err
	}
	wrapper := NewQueryWrapper[T]()
	for index, column := range m.entity.Columns {
		rawValue, err := fieldValue(value, column)
		if err != nil {
			return nil, err
		}
		strategy := effectiveWhereFieldStrategy(column.WhereStrategy, m.dbConfig.WhereStrategy)
		if !fieldIncludedByStrategy(rawValue, strategy) {
			continue
		}
		quotedColumn, err := quoteIdentifierPath(m.dialect, column.ColumnName)
		if err != nil {
			return nil, err
		}
		argName := fmt.Sprintf("%s%d", entityConditionArgPrefix, index)
		condition, err := entityColumnCondition(column, quotedColumn, argName)
		if err != nil {
			return nil, err
		}
		wrapper.Apply(condition, NamedArgs{argName: rawValue})
	}
	return wrapper, nil
}

// SelectListByEntity 按实体非零字段查询列表。
func (m *BaseMapper[T, ID]) SelectListByEntity(ctx context.Context, entity *T) ([]T, error) {
	wrapper, err := m.EntityQueryWrapper(entity)
	if err != nil {
		return nil, err
	}
	return m.SelectList(ctx, wrapper)
}

// SelectCountByEntity 按实体非零字段统计记录数。
func (m *BaseMapper[T, ID]) SelectCountByEntity(ctx context.Context, entity *T) (int64, error) {
	wrapper, err := m.EntityQueryWrapper(entity)
	if err != nil {
		return 0, err
	}
	return m.SelectCount(ctx, wrapper)
}

// DeleteByEntity 按实体非零字段删除记录，空条件仍由 Delete 保持 fail-fast。
func (m *BaseMapper[T, ID]) DeleteByEntity(ctx context.Context, entity *T) (int64, error) {
	wrapper, err := m.EntityQueryWrapper(entity)
	if err != nil {
		return 0, err
	}
	return m.Delete(ctx, wrapper)
}

func effectiveWhereFieldStrategy(column FieldStrategy, global FieldStrategy) FieldStrategy {
	if column != FieldStrategyDefault {
		return column
	}
	if global != FieldStrategyDefault {
		return global
	}
	return FieldStrategyNotZero
}

func entityColumnCondition(column ColumnMeta, quotedColumn string, argName string) (string, error) {
	format := strings.TrimSpace(column.Condition)
	if format == "" {
		return quotedColumn + " = #{" + argName + "}", nil
	}
	var sqlText string
	if strings.Contains(format, "{column}") || strings.Contains(format, "{value}") {
		sqlText = strings.ReplaceAll(format, "{column}", quotedColumn)
		sqlText = strings.ReplaceAll(sqlText, "{value}", "#{"+argName+"}")
	} else if strings.Count(format, "%s") == 2 {
		sqlText = fmt.Sprintf(format, quotedColumn, argName)
	} else {
		return "", fmt.Errorf("goark-orm: entity condition for field %s must use {column}/{value} or two %%s placeholders", column.FieldName)
	}
	if err := validateRawSQLFragment(sqlText); err != nil {
		return "", err
	}
	return sqlText, nil
}
