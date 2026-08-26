package ormgen

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type fakeSchemaIntrospector struct {
	request SchemaIntrospectionRequest
	schema  SchemaModel
}

func (f *fakeSchemaIntrospector) IntrospectSchema(ctx context.Context, request SchemaIntrospectionRequest) (SchemaModel, error) {
	f.request = request
	return f.schema, nil
}

func TestReverseEngineer_whenSchemaProvided_shouldBuildPackageModel(t *testing.T) {
	nullable := false
	size := 64
	introspector := &fakeSchemaIntrospector{
		schema: SchemaModel{
			Tables: []SchemaTable{
				{
					Name: "sys_user",
					Columns: []SchemaColumn{
						{Name: "id", DBType: "bigint", PrimaryKey: true, AutoIncrement: true, Nullable: &nullable},
						{Name: "user_name", DBType: "varchar", Size: &size},
						{Name: "created_at", DBType: "timestamp"},
					},
				},
			},
		},
	}

	model, err := ReverseEngineer(context.Background(), introspector, ReverseEngineerSpec{
		PackageName: "user",
		DatabaseID:  "postgres",
		Schema:      "public",
		Tables:      []string{"sys_user"},
		TablePrefix: "sys_",
	})
	if err != nil {
		t.Fatalf("reverse engineer failed: %v", err)
	}

	expectedRequest := SchemaIntrospectionRequest{
		DatabaseID: "postgres",
		Schema:     "public",
		Tables:     []string{"sys_user"},
	}
	if !reflect.DeepEqual(introspector.request, expectedRequest) {
		t.Fatalf("unexpected introspection request %#v", introspector.request)
	}
	if model.PackageName != "user" || len(model.Entities) != 1 {
		t.Fatalf("unexpected package model %#v", model)
	}
	entity := model.Entities[0]
	if entity.TypeName != "User" || entity.Table != "sys_user" {
		t.Fatalf("unexpected entity %#v", entity)
	}
	if len(entity.Columns) != 3 {
		t.Fatalf("unexpected columns %#v", entity.Columns)
	}
	if entity.Columns[0].FieldName != "ID" || entity.Columns[0].FieldType != "int64" || !entity.Columns[0].PrimaryKey || !entity.Columns[0].AutoIncrement {
		t.Fatalf("unexpected id column %#v", entity.Columns[0])
	}
	if entity.Columns[1].FieldName != "UserName" || entity.Columns[1].FieldType != "string" {
		t.Fatalf("unexpected name column %#v", entity.Columns[1])
	}
	if entity.Columns[2].FieldName != "CreatedAt" || entity.Columns[2].FieldType != "time.Time" {
		t.Fatalf("unexpected time column %#v", entity.Columns[2])
	}

	rendered, err := Render(model)
	if err != nil {
		t.Fatalf("render reverse engineered model failed: %v", err)
	}
	if !strings.Contains(string(rendered), `TypeName: "User"`) || !strings.Contains(string(rendered), `ColumnName: "user_name"`) {
		t.Fatalf("rendered source does not contain expected metadata:\n%s", rendered)
	}
}

func TestBuildPackageModelFromSchema_whenCustomTypeMapperProvided_shouldUseMapper(t *testing.T) {
	model, err := BuildPackageModelFromSchema(ReverseEngineerSpec{
		PackageName: "event",
		TypeMapper: SchemaTypeMapperFunc(func(column SchemaColumn) (string, error) {
			if column.Name == "payload" {
				return "json.RawMessage", nil
			}
			return DefaultSchemaTypeMapper().GoType(column)
		}),
	}, SchemaModel{
		Tables: []SchemaTable{
			{
				Name: "event_log",
				Columns: []SchemaColumn{
					{Name: "payload", DBType: "jsonb"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build package model failed: %v", err)
	}

	if got := model.Entities[0].Columns[0].FieldType; got != "json.RawMessage" {
		t.Fatalf("expected custom mapped type, got %q", got)
	}
}
