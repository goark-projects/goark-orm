package ormgen

import (
	"fmt"
	"strings"

	"goark.dev/orm"
)

// SchemaNamingStrategy 将数据库命名转换为 Go 导出标识符。
type SchemaNamingStrategy interface {
	EntityTypeName(table SchemaTable, tablePrefix string) (string, error)
	FieldName(table SchemaTable, column SchemaColumn) (string, error)
}

// SchemaNamingStrategyFuncs 使用函数组合 schema 命名策略。
type SchemaNamingStrategyFuncs struct {
	EntityTypeNameFunc func(SchemaTable, string) (string, error)
	FieldNameFunc      func(SchemaTable, SchemaColumn) (string, error)
}

// EntityTypeName 返回数据库表对应的 Go 实体类型名。
func (s SchemaNamingStrategyFuncs) EntityTypeName(table SchemaTable, tablePrefix string) (string, error) {
	if s.EntityTypeNameFunc != nil {
		return s.EntityTypeNameFunc(table, tablePrefix)
	}
	return DefaultSchemaNamingStrategy().EntityTypeName(table, tablePrefix)
}

// FieldName 返回数据库列对应的 Go 字段名。
func (s SchemaNamingStrategyFuncs) FieldName(table SchemaTable, column SchemaColumn) (string, error) {
	if s.FieldNameFunc != nil {
		return s.FieldNameFunc(table, column)
	}
	return DefaultSchemaNamingStrategy().FieldName(table, column)
}

// DefaultSchemaNamingStrategy 返回内置下划线转 PascalCase 的命名策略。
func DefaultSchemaNamingStrategy() SchemaNamingStrategy {
	return defaultSchemaNamingStrategy{}
}

type defaultSchemaNamingStrategy struct{}

func (defaultSchemaNamingStrategy) EntityTypeName(table SchemaTable, tablePrefix string) (string, error) {
	tableName := strings.TrimSpace(table.Name)
	if tableName == "" {
		return "", fmt.Errorf("goark-orm: schema table name is required")
	}
	name := strings.TrimPrefix(tableName, strings.TrimSpace(tablePrefix))
	return exportedSchemaIdentifier(name), nil
}

func (defaultSchemaNamingStrategy) FieldName(_ SchemaTable, column SchemaColumn) (string, error) {
	columnName := strings.TrimSpace(column.Name)
	if columnName == "" {
		return "", fmt.Errorf("goark-orm: schema column name is required")
	}
	return exportedSchemaIdentifier(columnName), nil
}

// SchemaColumnFilter 判断反向工程是否保留指定列。
type SchemaColumnFilter interface {
	IncludeColumn(table SchemaTable, column SchemaColumn) bool
}

// SchemaColumnFilterFunc 将函数适配为列过滤器。
type SchemaColumnFilterFunc func(table SchemaTable, column SchemaColumn) bool

// IncludeColumn 执行函数式列过滤。
func (f SchemaColumnFilterFunc) IncludeColumn(table SchemaTable, column SchemaColumn) bool {
	if f == nil {
		return true
	}
	return f(table, column)
}

// SchemaColumnOverride 覆盖反向工程阶段单列生成策略。
type SchemaColumnOverride struct {
	FieldName      string
	GoType         string
	TypeHandler    string
	Condition      string
	SelectDisabled *bool
	IDType         orm.IDType
	InsertStrategy orm.FieldStrategy
	UpdateStrategy orm.FieldStrategy
	WhereStrategy  orm.FieldStrategy
	Version        *bool
	SoftDelete     *bool
	CreatedAt      *bool
	UpdatedAt      *bool
	Fill           orm.FieldFill
}

type reverseColumnSelection struct {
	ignore map[string]struct{}
	filter SchemaColumnFilter
}

func newReverseColumnSelection(spec ReverseEngineerSpec) reverseColumnSelection {
	ignore := make(map[string]struct{}, len(spec.IgnoreColumns)*2)
	for _, value := range spec.IgnoreColumns {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		ignore[foldReverseSchemaKey(value)] = struct{}{}
	}
	return reverseColumnSelection{ignore: ignore, filter: spec.ColumnFilter}
}

func (s reverseColumnSelection) include(table SchemaTable, column SchemaColumn) bool {
	for _, key := range reverseColumnLookupKeys(table.Name, column.Name) {
		if _, ok := s.ignore[key]; ok {
			return false
		}
	}
	if s.filter != nil && !s.filter.IncludeColumn(table, column) {
		return false
	}
	return true
}

func schemaColumnOverride(spec ReverseEngineerSpec, table SchemaTable, column SchemaColumn) (SchemaColumnOverride, bool) {
	if len(spec.ColumnOverrides) == 0 {
		return SchemaColumnOverride{}, false
	}
	for _, key := range reverseColumnLookupKeys(table.Name, column.Name) {
		override, ok := spec.ColumnOverrides[key]
		if ok {
			return override, true
		}
	}
	return SchemaColumnOverride{}, false
}

func applySchemaColumnOverride(column SchemaColumn, override SchemaColumnOverride) SchemaColumn {
	if value := strings.TrimSpace(override.FieldName); value != "" {
		column.FieldName = value
	}
	if value := strings.TrimSpace(override.GoType); value != "" {
		column.GoType = value
	}
	if value := strings.TrimSpace(override.TypeHandler); value != "" {
		column.TypeHandler = value
	}
	return column
}

func applyColumnModelOverride(column ColumnModel, override SchemaColumnOverride) ColumnModel {
	if value := strings.TrimSpace(override.Condition); value != "" {
		column.Condition = value
	}
	if override.SelectDisabled != nil {
		column.SelectDisabled = *override.SelectDisabled
	}
	if override.IDType != orm.IDTypeNone {
		column.IDType = override.IDType
	}
	if override.InsertStrategy != orm.FieldStrategyDefault {
		column.InsertStrategy = override.InsertStrategy
	}
	if override.UpdateStrategy != orm.FieldStrategyDefault {
		column.UpdateStrategy = override.UpdateStrategy
	}
	if override.WhereStrategy != orm.FieldStrategyDefault {
		column.WhereStrategy = override.WhereStrategy
	}
	if override.Version != nil {
		column.Version = *override.Version
	}
	if override.SoftDelete != nil {
		column.SoftDelete = *override.SoftDelete
	}
	if override.CreatedAt != nil {
		column.CreatedAt = *override.CreatedAt
	}
	if override.UpdatedAt != nil {
		column.UpdatedAt = *override.UpdatedAt
	}
	if override.Fill != orm.FieldFillDefault {
		column.Fill = override.Fill
	}
	return column
}

func normalizeColumnOverrides(overrides map[string]SchemaColumnOverride) map[string]SchemaColumnOverride {
	if len(overrides) == 0 {
		return nil
	}
	out := make(map[string]SchemaColumnOverride, len(overrides))
	for key, value := range overrides {
		key = foldReverseSchemaKey(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func reverseColumnLookupKeys(table string, column string) []string {
	table = strings.TrimSpace(table)
	column = strings.TrimSpace(column)
	keys := make([]string, 0, 2)
	if table != "" && column != "" {
		keys = append(keys, foldReverseSchemaKey(table+"."+column))
	}
	if column != "" {
		keys = append(keys, foldReverseSchemaKey(column))
	}
	return keys
}

func foldReverseSchemaKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToLower(value)
}
