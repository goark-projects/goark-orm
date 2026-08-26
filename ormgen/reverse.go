package ormgen

import (
	"context"
	"fmt"
	"strings"
)

// SchemaIntrospector 由数据库适配层实现，core 不直接连接数据库。
type SchemaIntrospector interface {
	IntrospectSchema(ctx context.Context, request SchemaIntrospectionRequest) (SchemaModel, error)
}

// SchemaIntrospectionRequest 描述一次 schema 读取请求。
type SchemaIntrospectionRequest struct {
	DatabaseID string
	Schema     string
	Tables     []string
}

// ReverseEngineerSpec 描述 schema 到 ORM 生成模型的转换规则。
type ReverseEngineerSpec struct {
	PackageName string
	DatabaseID  string
	Schema      string
	Tables      []string
	TablePrefix string
	TypeMapper  SchemaTypeMapper
}

// SchemaModel 是数据库结构的 Go 化中间模型。
type SchemaModel struct {
	Tables []SchemaTable
}

// SchemaTable 描述一张数据库表。
type SchemaTable struct {
	Name     string
	TypeName string
	Columns  []SchemaColumn
}

// SchemaColumn 描述数据库列到实体字段的候选映射。
type SchemaColumn struct {
	Name          string
	FieldName     string
	DBType        string
	GoType        string
	PrimaryKey    bool
	AutoIncrement bool
	Nullable      *bool
	Size          *int
	NumericScale  *int
	DefaultValue  string
	TypeHandler   string
}

// SchemaTypeMapper 把数据库类型映射为 Go 字段类型。
type SchemaTypeMapper interface {
	GoType(column SchemaColumn) (string, error)
}

// SchemaTypeMapperFunc 将函数适配为 SchemaTypeMapper。
type SchemaTypeMapperFunc func(column SchemaColumn) (string, error)

// GoType 执行函数式类型映射器。
func (f SchemaTypeMapperFunc) GoType(column SchemaColumn) (string, error) {
	if f == nil {
		return "", fmt.Errorf("goark-orm: schema type mapper is nil")
	}
	return f(column)
}

// DefaultSchemaTypeMapper 返回内置保守类型映射器。
func DefaultSchemaTypeMapper() SchemaTypeMapper {
	return SchemaTypeMapperFunc(defaultSchemaGoType)
}

// ReverseEngineer 读取 schema 并转换为 PackageModel，调用方可继续使用 Render 或自定义模板。
func ReverseEngineer(ctx context.Context, introspector SchemaIntrospector, spec ReverseEngineerSpec) (*PackageModel, error) {
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm: context is nil")
	}
	if introspector == nil {
		return nil, fmt.Errorf("goark-orm: schema introspector is nil")
	}
	schema, err := introspector.IntrospectSchema(ctx, SchemaIntrospectionRequest{
		DatabaseID: spec.DatabaseID,
		Schema:     spec.Schema,
		Tables:     append([]string(nil), spec.Tables...),
	})
	if err != nil {
		return nil, err
	}
	return BuildPackageModelFromSchema(spec, schema)
}

// BuildPackageModelFromSchema 把 schema 中间模型转换为 ORM 包模型。
func BuildPackageModelFromSchema(spec ReverseEngineerSpec, schema SchemaModel) (*PackageModel, error) {
	packageName := strings.TrimSpace(spec.PackageName)
	if packageName == "" {
		return nil, fmt.Errorf("goark-orm: reverse package name is required")
	}
	mapper := spec.TypeMapper
	if mapper == nil {
		mapper = DefaultSchemaTypeMapper()
	}
	model := &PackageModel{
		PackageName: packageName,
		Entities:    make([]EntityModel, 0, len(schema.Tables)),
	}
	for _, table := range schema.Tables {
		entity, err := schemaTableToEntity(table, spec.TablePrefix, mapper)
		if err != nil {
			return nil, err
		}
		model.Entities = append(model.Entities, entity)
	}
	return model, nil
}

func schemaTableToEntity(table SchemaTable, tablePrefix string, mapper SchemaTypeMapper) (EntityModel, error) {
	tableName := strings.TrimSpace(table.Name)
	if tableName == "" {
		return EntityModel{}, fmt.Errorf("goark-orm: schema table name is required")
	}
	typeName := strings.TrimSpace(table.TypeName)
	if typeName == "" {
		typeName = pascalIdentifier(strings.TrimPrefix(tableName, strings.TrimSpace(tablePrefix)))
	}
	if typeName == "" {
		return EntityModel{}, fmt.Errorf("goark-orm: schema table %s cannot derive type name", tableName)
	}
	entity := EntityModel{
		TypeName: typeName,
		Table:    tableName,
		Columns:  make([]ColumnModel, 0, len(table.Columns)),
	}
	for _, column := range table.Columns {
		mapped, err := schemaColumnToModel(column, mapper)
		if err != nil {
			return EntityModel{}, err
		}
		entity.Columns = append(entity.Columns, mapped)
	}
	return entity, nil
}

func schemaColumnToModel(column SchemaColumn, mapper SchemaTypeMapper) (ColumnModel, error) {
	columnName := strings.TrimSpace(column.Name)
	if columnName == "" {
		return ColumnModel{}, fmt.Errorf("goark-orm: schema column name is required")
	}
	fieldName := strings.TrimSpace(column.FieldName)
	if fieldName == "" {
		fieldName = pascalIdentifier(columnName)
	}
	goType := strings.TrimSpace(column.GoType)
	if goType == "" {
		mapped, err := mapper.GoType(column)
		if err != nil {
			return ColumnModel{}, err
		}
		goType = strings.TrimSpace(mapped)
	}
	if goType == "" {
		return ColumnModel{}, fmt.Errorf("goark-orm: schema column %s cannot derive Go type", columnName)
	}
	return ColumnModel{
		FieldName:     fieldName,
		FieldType:     goType,
		ColumnName:    columnName,
		PrimaryKey:    column.PrimaryKey,
		AutoIncrement: column.AutoIncrement,
		Nullable:      cloneBoolPointer(column.Nullable),
		Size:          cloneIntPointer(column.Size),
		NumericScale:  cloneIntPointer(column.NumericScale),
		DBType:        strings.TrimSpace(column.DBType),
		DefaultValue:  strings.TrimSpace(column.DefaultValue),
		TypeHandler:   strings.TrimSpace(column.TypeHandler),
	}, nil
}

func defaultSchemaGoType(column SchemaColumn) (string, error) {
	dbType := strings.ToLower(strings.TrimSpace(column.DBType))
	switch {
	case strings.Contains(dbType, "bigint"):
		return "int64", nil
	case strings.Contains(dbType, "smallint"):
		return "int16", nil
	case strings.Contains(dbType, "int"):
		return "int", nil
	case strings.Contains(dbType, "bool"):
		return "bool", nil
	case strings.Contains(dbType, "decimal"), strings.Contains(dbType, "numeric"):
		return "string", nil
	case strings.Contains(dbType, "double"), strings.Contains(dbType, "float"), strings.Contains(dbType, "real"):
		return "float64", nil
	case strings.Contains(dbType, "date"), strings.Contains(dbType, "time"):
		return "time.Time", nil
	case strings.Contains(dbType, "blob"), strings.Contains(dbType, "binary"), strings.Contains(dbType, "bytea"):
		return "[]byte", nil
	default:
		return "string", nil
	}
}

func pascalIdentifier(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})
	var builder strings.Builder
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		builder.WriteString(schemaIdentifierPart(string(runes)))
	}
	return builder.String()
}

func schemaIdentifierPart(value string) string {
	switch strings.ToLower(value) {
	case "id":
		return "ID"
	case "api":
		return "API"
	case "db":
		return "DB"
	case "http":
		return "HTTP"
	case "https":
		return "HTTPS"
	case "ip":
		return "IP"
	case "json":
		return "JSON"
	case "sql":
		return "SQL"
	case "url":
		return "URL"
	case "uuid":
		return "UUID"
	case "xml":
		return "XML"
	default:
		runes := []rune(value)
		if len(runes) == 0 {
			return ""
		}
		return strings.ToUpper(string(runes[0])) + string(runes[1:])
	}
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
