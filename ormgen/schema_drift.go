package ormgen

import (
	"context"
	"fmt"
	"sort"
	"strings"

	orm "goark.dev/orm"
)

// SchemaDriftKind 描述 schema 漂移问题类型。
type SchemaDriftKind string

const (
	// SchemaDriftMissingTable 表示实体表在数据库中不存在。
	SchemaDriftMissingTable SchemaDriftKind = "missing_table"
	// SchemaDriftMissingColumn 表示实体列在数据库中不存在。
	SchemaDriftMissingColumn SchemaDriftKind = "missing_column"
	// SchemaDriftExtraColumn 表示数据库列未被实体元数据覆盖。
	SchemaDriftExtraColumn SchemaDriftKind = "extra_column"
	// SchemaDriftPrimaryKeyMismatch 表示主键标记与数据库不一致。
	SchemaDriftPrimaryKeyMismatch SchemaDriftKind = "primary_key_mismatch"
	// SchemaDriftAutoIncrementMismatch 表示自增标记与数据库不一致。
	SchemaDriftAutoIncrementMismatch SchemaDriftKind = "auto_increment_mismatch"
	// SchemaDriftNullableMismatch 表示空值约束与数据库不一致。
	SchemaDriftNullableMismatch SchemaDriftKind = "nullable_mismatch"
	// SchemaDriftSizeMismatch 表示字符长度与数据库不一致。
	SchemaDriftSizeMismatch SchemaDriftKind = "size_mismatch"
	// SchemaDriftNumericScaleMismatch 表示数值小数位与数据库不一致。
	SchemaDriftNumericScaleMismatch SchemaDriftKind = "numeric_scale_mismatch"
	// SchemaDriftTypeMismatch 表示数据库类型与实体元数据不一致。
	SchemaDriftTypeMismatch SchemaDriftKind = "type_mismatch"
)

// SchemaDriftIssue 描述一个实体元数据和真实数据库 schema 的差异。
type SchemaDriftIssue struct {
	Kind     SchemaDriftKind
	Entity   string
	Table    string
	Field    string
	Column   string
	Expected string
	Actual   string
}

// SchemaDriftReport 汇总一次 schema 漂移检测结果。
type SchemaDriftReport struct {
	Issues []SchemaDriftIssue
}

// HasDrift 返回是否发现 schema 漂移。
func (r SchemaDriftReport) HasDrift() bool {
	return len(r.Issues) > 0
}

// Error 返回适合测试或 CLI 输出的简短漂移摘要。
func (r SchemaDriftReport) Error() string {
	if len(r.Issues) == 0 {
		return "goark-orm: schema drift not found"
	}
	first := r.Issues[0]
	return fmt.Sprintf("goark-orm: schema drift detected: %s table=%s column=%s expected=%s actual=%s", first.Kind, first.Table, first.Column, first.Expected, first.Actual)
}

type schemaDriftOptions struct {
	reportExtraColumns bool
}

// SchemaDriftOption 调整 schema 漂移检测行为。
type SchemaDriftOption func(*schemaDriftOptions)

// WithSchemaDriftExtraColumns 控制是否报告数据库中存在但实体未映射的列。
func WithSchemaDriftExtraColumns(enabled bool) SchemaDriftOption {
	return func(options *schemaDriftOptions) {
		options.reportExtraColumns = enabled
	}
}

// DetectSchemaDrift 读取真实 schema 并与运行时实体元数据比对。
func DetectSchemaDrift(ctx context.Context, introspector SchemaIntrospector, registry *orm.Registry, request SchemaIntrospectionRequest, options ...SchemaDriftOption) (SchemaDriftReport, error) {
	if ctx == nil {
		return SchemaDriftReport{}, fmt.Errorf("goark-orm: context is nil")
	}
	if introspector == nil {
		return SchemaDriftReport{}, fmt.Errorf("goark-orm: schema introspector is nil")
	}
	if registry == nil {
		return SchemaDriftReport{}, fmt.Errorf("goark-orm: registry is nil")
	}
	if len(request.Tables) == 0 {
		request.Tables = registryEntityTables(registry)
	}
	schema, err := introspector.IntrospectSchema(ctx, request)
	if err != nil {
		return SchemaDriftReport{}, err
	}
	return CompareSchemaDrift(registry, schema, options...)
}

// ValidateSchemaDrift 在发现漂移时返回 SchemaDriftReport 作为错误值。
func ValidateSchemaDrift(ctx context.Context, introspector SchemaIntrospector, registry *orm.Registry, request SchemaIntrospectionRequest, options ...SchemaDriftOption) error {
	report, err := DetectSchemaDrift(ctx, introspector, registry, request, options...)
	if err != nil {
		return err
	}
	if report.HasDrift() {
		return report
	}
	return nil
}

// CompareSchemaDrift 将已读取的 schema 与运行时实体元数据比对。
func CompareSchemaDrift(registry *orm.Registry, schema SchemaModel, options ...SchemaDriftOption) (SchemaDriftReport, error) {
	if registry == nil {
		return SchemaDriftReport{}, fmt.Errorf("goark-orm: registry is nil")
	}
	opts := schemaDriftOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	tables := schemaTableIndex(schema.Tables)
	issues := make([]SchemaDriftIssue, 0)
	for _, entity := range sortedRegistryEntities(registry) {
		tableName := entitySchemaTableName(entity.Table)
		table, ok := tables[foldSchemaName(tableName)]
		if !ok {
			issues = append(issues, SchemaDriftIssue{
				Kind:     SchemaDriftMissingTable,
				Entity:   entity.TypeName,
				Table:    tableName,
				Expected: "table exists",
				Actual:   "missing",
			})
			continue
		}
		issues = append(issues, compareEntityColumns(entity, table, opts)...)
	}
	sortSchemaDriftIssues(issues)
	return SchemaDriftReport{Issues: issues}, nil
}

func compareEntityColumns(entity orm.EntityMeta, table SchemaTable, opts schemaDriftOptions) []SchemaDriftIssue {
	columns := schemaColumnIndex(table.Columns)
	mapped := make(map[string]struct{}, len(entity.Columns))
	issues := make([]SchemaDriftIssue, 0)
	for _, column := range entity.Columns {
		columnName := schemaColumnName(column)
		if columnName == "" {
			continue
		}
		key := foldSchemaName(columnName)
		mapped[key] = struct{}{}
		actual, ok := columns[key]
		if !ok {
			issues = append(issues, newSchemaDriftIssue(SchemaDriftMissingColumn, entity, table, column, "column exists", "missing"))
			continue
		}
		issues = append(issues, compareColumnShape(entity, table, column, actual)...)
	}
	if opts.reportExtraColumns {
		for _, column := range table.Columns {
			key := foldSchemaName(column.Name)
			if _, ok := mapped[key]; ok {
				continue
			}
			issues = append(issues, SchemaDriftIssue{
				Kind:     SchemaDriftExtraColumn,
				Entity:   entity.TypeName,
				Table:    table.Name,
				Column:   column.Name,
				Expected: "unmapped",
				Actual:   "column exists",
			})
		}
	}
	return issues
}

func compareColumnShape(entity orm.EntityMeta, table SchemaTable, expected orm.ColumnMeta, actual SchemaColumn) []SchemaDriftIssue {
	issues := make([]SchemaDriftIssue, 0)
	if expected.PrimaryKey != actual.PrimaryKey {
		issues = append(issues, newSchemaDriftIssue(SchemaDriftPrimaryKeyMismatch, entity, table, expected, boolSchemaString(expected.PrimaryKey), boolSchemaString(actual.PrimaryKey)))
	}
	if expected.AutoIncrement != actual.AutoIncrement {
		issues = append(issues, newSchemaDriftIssue(SchemaDriftAutoIncrementMismatch, entity, table, expected, boolSchemaString(expected.AutoIncrement), boolSchemaString(actual.AutoIncrement)))
	}
	if expected.Nullable != nil && actual.Nullable != nil && *expected.Nullable != *actual.Nullable {
		issues = append(issues, newSchemaDriftIssue(SchemaDriftNullableMismatch, entity, table, expected, boolSchemaString(*expected.Nullable), boolSchemaString(*actual.Nullable)))
	}
	if expected.Size != nil && actual.Size != nil && *expected.Size != *actual.Size {
		issues = append(issues, newSchemaDriftIssue(SchemaDriftSizeMismatch, entity, table, expected, intSchemaString(*expected.Size), intSchemaString(*actual.Size)))
	}
	if expected.NumericScale != nil && actual.NumericScale != nil && *expected.NumericScale != *actual.NumericScale {
		issues = append(issues, newSchemaDriftIssue(SchemaDriftNumericScaleMismatch, entity, table, expected, intSchemaString(*expected.NumericScale), intSchemaString(*actual.NumericScale)))
	}
	if expected.DBType != "" && actual.DBType != "" && !sameSchemaDBType(expected.DBType, actual.DBType) {
		issues = append(issues, newSchemaDriftIssue(SchemaDriftTypeMismatch, entity, table, expected, expected.DBType, actual.DBType))
	}
	return issues
}

func newSchemaDriftIssue(kind SchemaDriftKind, entity orm.EntityMeta, table SchemaTable, column orm.ColumnMeta, expected string, actual string) SchemaDriftIssue {
	return SchemaDriftIssue{
		Kind:     kind,
		Entity:   entity.TypeName,
		Table:    table.Name,
		Field:    column.FieldName,
		Column:   schemaColumnName(column),
		Expected: expected,
		Actual:   actual,
	}
}

func sortedRegistryEntities(registry *orm.Registry) []orm.EntityMeta {
	entities := registry.Entities()
	sort.Slice(entities, func(i, j int) bool {
		return strings.TrimSpace(entities[i].TypeName) < strings.TrimSpace(entities[j].TypeName)
	})
	return entities
}

func registryEntityTables(registry *orm.Registry) []string {
	entities := sortedRegistryEntities(registry)
	out := make([]string, 0, len(entities))
	seen := make(map[string]struct{}, len(entities))
	for _, entity := range entities {
		table := entitySchemaTableName(entity.Table)
		if table == "" {
			continue
		}
		key := foldSchemaName(table)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, table)
	}
	return out
}

func schemaTableIndex(tables []SchemaTable) map[string]SchemaTable {
	out := make(map[string]SchemaTable, len(tables))
	for _, table := range tables {
		name := strings.TrimSpace(table.Name)
		if name == "" {
			continue
		}
		out[foldSchemaName(name)] = table
	}
	return out
}

func schemaColumnIndex(columns []SchemaColumn) map[string]SchemaColumn {
	out := make(map[string]SchemaColumn, len(columns))
	for _, column := range columns {
		name := strings.TrimSpace(column.Name)
		if name == "" {
			continue
		}
		out[foldSchemaName(name)] = column
	}
	return out
}

func schemaColumnName(column orm.ColumnMeta) string {
	if name := strings.TrimSpace(column.ColumnName); name != "" {
		return unquoteSchemaNamePart(name)
	}
	return strings.TrimSpace(column.FieldName)
}

func entitySchemaTableName(table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		return ""
	}
	parts := strings.Split(table, ".")
	return unquoteSchemaNamePart(parts[len(parts)-1])
}

func unquoteSchemaNamePart(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	first := value[0]
	last := value[len(value)-1]
	if first == last && (first == '"' || first == '`') {
		return strings.ReplaceAll(value[1:len(value)-1], string(first)+string(first), string(first))
	}
	if first == '[' && last == ']' {
		return strings.ReplaceAll(value[1:len(value)-1], "]]", "]")
	}
	return value
}

func foldSchemaName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sameSchemaDBType(expected string, actual string) bool {
	return normalizeSchemaDBType(expected) == normalizeSchemaDBType(actual)
}

func normalizeSchemaDBType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if index := strings.IndexByte(value, '('); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	value = strings.Join(strings.Fields(value), " ")
	switch value {
	case "int8", "bigserial", "serial8":
		return "bigint"
	case "int4", "integer", "serial", "serial4", "mediumint":
		return "int"
	case "int2", "tinyint":
		return "smallint"
	case "bool":
		return "boolean"
	case "character varying", "nvarchar", "varchar2":
		return "varchar"
	case "character", "nchar":
		return "char"
	case "float", "double precision", "real":
		return "double"
	case "numeric", "number":
		return "decimal"
	case "timestamp without time zone", "datetime":
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"
	case "bytea", "varbinary", "binary", "blob", "longblob":
		return "bytes"
	default:
		return value
	}
}

func boolSchemaString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func intSchemaString(value int) string {
	return fmt.Sprintf("%d", value)
}

func sortSchemaDriftIssues(issues []SchemaDriftIssue) {
	sort.Slice(issues, func(i, j int) bool {
		left := schemaDriftSortKey(issues[i])
		right := schemaDriftSortKey(issues[j])
		return left < right
	})
}

func schemaDriftSortKey(issue SchemaDriftIssue) string {
	return strings.Join([]string{
		issue.Entity,
		issue.Table,
		issue.Column,
		issue.Field,
		string(issue.Kind),
	}, "\x00")
}
