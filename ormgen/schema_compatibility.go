package ormgen

import (
	"context"
	"strings"

	orm "goark.dev/orm"
)

// SQLSchemaCompatibilityConfig 描述一次 schema 读取、反向工程和漂移校验。
type SQLSchemaCompatibilityConfig struct {
	DBType       orm.DbType
	SQLQueryer   SQLQueryer
	Queryer      SchemaQueryer
	Schema       string
	Tables       []string
	PackageName  string
	ReverseSpec  ReverseEngineerSpec
	Registry     *orm.Registry
	DriftOptions []SchemaDriftOption
}

// SQLSchemaCompatibilityReport 汇总 schema 兼容性验证结果。
type SQLSchemaCompatibilityReport struct {
	Schema SchemaModel
	Model  *PackageModel
	Source []byte
	Drift  SchemaDriftReport
}

// ValidateSQLSchemaCompatibility 执行真实 schema 到生成模型的端到端 smoke。
func ValidateSQLSchemaCompatibility(ctx context.Context, config SQLSchemaCompatibilityConfig) (SQLSchemaCompatibilityReport, error) {
	if ctx == nil {
		return SQLSchemaCompatibilityReport{}, errSchemaCompatibility("context is nil")
	}
	introspector, err := newCompatibilityIntrospector(config)
	if err != nil {
		return SQLSchemaCompatibilityReport{}, err
	}
	request := schemaCompatibilityRequest(config)
	schema, err := introspector.IntrospectSchema(ctx, request)
	if err != nil {
		return SQLSchemaCompatibilityReport{}, err
	}
	spec := schemaCompatibilityReverseSpec(config, request)
	model, err := BuildPackageModelFromSchema(spec, schema)
	if err != nil {
		return SQLSchemaCompatibilityReport{}, err
	}
	source, err := Render(model)
	if err != nil {
		return SQLSchemaCompatibilityReport{}, err
	}
	report := SQLSchemaCompatibilityReport{
		Schema: schema,
		Model:  model,
		Source: source,
	}
	if config.Registry != nil {
		drift, err := CompareSchemaDrift(config.Registry, schema, config.DriftOptions...)
		if err != nil {
			return report, err
		}
		report.Drift = drift
		if drift.HasDrift() {
			return report, drift
		}
	}
	return report, nil
}

func newCompatibilityIntrospector(config SQLSchemaCompatibilityConfig) (*SQLSchemaIntrospector, error) {
	dialect, err := NewSQLSchemaDialect(config.DBType)
	if err != nil {
		return nil, err
	}
	if config.Queryer != nil {
		return NewSQLSchemaIntrospectorWithQueryer(config.Queryer, dialect)
	}
	return NewSQLSchemaIntrospector(config.SQLQueryer, dialect)
}

func schemaCompatibilityRequest(config SQLSchemaCompatibilityConfig) SchemaIntrospectionRequest {
	spec := config.ReverseSpec
	databaseID := strings.TrimSpace(spec.DatabaseID)
	if databaseID == "" {
		databaseID = strings.TrimSpace(string(config.DBType))
	}
	schema := strings.TrimSpace(config.Schema)
	if schema == "" {
		schema = strings.TrimSpace(spec.Schema)
	}
	tables := append([]string(nil), config.Tables...)
	if len(tables) == 0 {
		tables = append([]string(nil), spec.Tables...)
	}
	return SchemaIntrospectionRequest{
		DatabaseID: databaseID,
		Schema:     schema,
		Tables:     tables,
	}
}

func schemaCompatibilityReverseSpec(config SQLSchemaCompatibilityConfig, request SchemaIntrospectionRequest) ReverseEngineerSpec {
	spec := config.ReverseSpec
	if strings.TrimSpace(spec.PackageName) == "" {
		spec.PackageName = strings.TrimSpace(config.PackageName)
	}
	if strings.TrimSpace(spec.DatabaseID) == "" {
		spec.DatabaseID = request.DatabaseID
	}
	if strings.TrimSpace(spec.Schema) == "" {
		spec.Schema = request.Schema
	}
	if len(spec.Tables) == 0 {
		spec.Tables = append([]string(nil), request.Tables...)
	}
	return spec
}

func errSchemaCompatibility(message string) error {
	return &SchemaCompatibilityError{Message: message}
}

// SchemaCompatibilityError 描述 schema 兼容性 helper 的输入错误。
type SchemaCompatibilityError struct {
	Message string
}

func (e *SchemaCompatibilityError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "goark-orm: schema compatibility error"
	}
	return "goark-orm: schema compatibility " + e.Message
}
