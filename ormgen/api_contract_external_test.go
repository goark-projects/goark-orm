package ormgen_test

import (
	"context"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/ormgen"
)

type contractSchemaIntrospector struct {
	schema ormgen.SchemaModel
}

func (i contractSchemaIntrospector) IntrospectSchema(context.Context, ormgen.SchemaIntrospectionRequest) (ormgen.SchemaModel, error) {
	return i.schema, nil
}

func TestV1GeneratorPublicAPIContract_shouldCompileExternalUsage(t *testing.T) {
	nullable := false
	size := 64
	schema := ormgen.SchemaModel{Tables: []ormgen.SchemaTable{{
		Name:     "sys_user",
		TypeName: "User",
		Columns: []ormgen.SchemaColumn{
			{Name: "id", FieldName: "ID", DBType: "bigint", GoType: "int64", PrimaryKey: true, AutoIncrement: true, Nullable: &nullable},
			{Name: "name", FieldName: "Name", DBType: "varchar", GoType: "string", Nullable: &nullable, Size: &size},
		},
	}}}

	mapper := ormgen.SchemaTypeMapperFunc(func(column ormgen.SchemaColumn) (string, error) {
		if column.GoType != "" {
			return column.GoType, nil
		}
		return "string", nil
	})
	spec := ormgen.ReverseEngineerSpec{
		PackageName: "contract",
		DatabaseID:  "postgres",
		TypeMapper:  mapper,
	}
	model, err := ormgen.BuildPackageModelFromSchema(spec, schema)
	if err != nil {
		t.Fatalf("build package model failed: %v", err)
	}
	if len(model.Entities) != 1 || model.Entities[0].TypeName != "User" {
		t.Fatalf("unexpected model %#v", model)
	}

	introspector := contractSchemaIntrospector{schema: schema}
	reversed, err := ormgen.ReverseEngineer(context.Background(), introspector, spec)
	if err != nil {
		t.Fatalf("reverse engineer failed: %v", err)
	}
	if reversed.PackageName != "contract" {
		t.Fatalf("unexpected reversed package %q", reversed.PackageName)
	}

	renderer := ormgen.TemplateRendererFunc(func(model *ormgen.PackageModel) ([]byte, error) {
		return []byte("package " + model.PackageName + "\n"), nil
	})
	rendered, err := renderer.RenderPackage(model)
	if err != nil {
		t.Fatalf("render package failed: %v", err)
	}
	if string(rendered) != "package contract\n" {
		t.Fatalf("unexpected rendered source %q", rendered)
	}

	registry := orm.NewRegistry()
	if err := registry.RegisterEntity(orm.EntityMeta{
		TypeName: "User",
		Table:    "sys_user",
		Columns:  []orm.ColumnMeta{{FieldName: "ID", ColumnName: "id", PrimaryKey: true, AutoIncrement: true}},
	}); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	report, err := ormgen.CompareSchemaDrift(registry, schema, ormgen.WithSchemaDriftExtraColumns(true))
	if err != nil {
		t.Fatalf("compare schema drift failed: %v", err)
	}
	if !report.HasDrift() {
		t.Fatalf("expected extra column drift")
	}

	var _ ormgen.SchemaIntrospector = introspector
	var _ ormgen.SchemaTypeMapper = mapper
	var _ ormgen.TemplateRenderer = renderer
	var _ func(ormgen.GenerateSpec) ([]byte, error) = ormgen.Generate
	var _ func(ormgen.GenerateSpec, ormgen.TemplateRenderer) ([]byte, error) = ormgen.GenerateWithRenderer
	_ = ormgen.GenerateSpec{PackageName: "contract", TypeHandlers: []string{"json"}}
	_ = ormgen.PackageModel{PackageName: "contract"}
	_ = ormgen.MapperModel{Namespace: "contract.UserMapper", Cache: orm.CacheMeta{Enabled: true, Blocking: true}}
	_ = ormgen.StatementModel{Command: orm.StatementCommandSelect, UseCache: orm.StatementCacheEnabled}
	_ = ormgen.GeneratedPackage{PackageName: "contract", Source: rendered}
	_ = ormgen.DefaultOutputName("contract")
	if _, err := ormgen.NewSQLSchemaDialect(orm.DbTypePostgres); err != nil {
		t.Fatalf("new SQL schema dialect failed: %v", err)
	}
}
