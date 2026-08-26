package ormgen

import (
	"context"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"goark.dev/orm"
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
	if !entity.DeclareStruct {
		t.Fatalf("expected reverse engineered entity to declare struct")
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
	if !strings.Contains(string(rendered), `type User struct`) || !strings.Contains(string(rendered), `goark-orm:\"column='id';primary-key=true;auto-increment=true;nullable=false;type='bigint'\"`) {
		t.Fatalf("rendered source does not contain declared entity:\n%s", rendered)
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

func TestBuildPackageModelFromSchema_whenReverseOptionsProvided_shouldApplyNamingFilterOverridesAndTags(t *testing.T) {
	selectDisabled := true
	version := true
	updatedAt := true
	nullable := false
	model, err := BuildPackageModelFromSchema(ReverseEngineerSpec{
		PackageName: "account",
		TablePrefix: "sys_",
		NamingStrategy: SchemaNamingStrategyFuncs{
			EntityTypeNameFunc: func(table SchemaTable, tablePrefix string) (string, error) {
				if table.Name == "sys_user" && tablePrefix == "sys_" {
					return "AccountUser", nil
				}
				return DefaultSchemaNamingStrategy().EntityTypeName(table, tablePrefix)
			},
		},
		IgnoreColumns: []string{"sys_user.password_hash"},
		ColumnFilter: SchemaColumnFilterFunc(func(_ SchemaTable, column SchemaColumn) bool {
			return column.Name != "shadow_flag"
		}),
		ColumnOverrides: map[string]SchemaColumnOverride{
			"tenant_id": {
				FieldName:      "TenantID",
				SelectDisabled: &selectDisabled,
			},
			"profile": {
				GoType:        "map[string]any",
				TypeHandler:   "json",
				WhereStrategy: orm.FieldStrategyNotNull,
			},
			"version": {
				Version: &version,
			},
			"updated_at": {
				UpdatedAt: &updatedAt,
				Fill:      orm.FieldFillInsertUpdate,
			},
		},
	}, SchemaModel{Tables: []SchemaTable{{
		Name: "sys_user",
		Columns: []SchemaColumn{
			{Name: "id", DBType: "bigint", PrimaryKey: true, AutoIncrement: true, Nullable: &nullable},
			{Name: "tenant_id", DBType: "bigint"},
			{Name: "profile", DBType: "jsonb"},
			{Name: "version", DBType: "bigint"},
			{Name: "updated_at", DBType: "timestamp"},
			{Name: "password_hash", DBType: "varchar"},
			{Name: "shadow_flag", DBType: "boolean"},
		},
	}}})
	if err != nil {
		t.Fatalf("build package model failed: %v", err)
	}

	entity := model.Entities[0]
	if entity.TypeName != "AccountUser" || len(entity.Columns) != 5 {
		t.Fatalf("unexpected reverse entity %#v", entity)
	}
	if containsColumn(entity.Columns, "password_hash") || containsColumn(entity.Columns, "shadow_flag") {
		t.Fatalf("expected ignored columns to be filtered: %#v", entity.Columns)
	}
	profile := findColumn(t, entity.Columns, "profile")
	if profile.FieldType != "map[string]any" || profile.TypeHandler != "json" || profile.WhereStrategy != orm.FieldStrategyNotNull {
		t.Fatalf("unexpected profile column %#v", profile)
	}
	tenantID := findColumn(t, entity.Columns, "tenant_id")
	if tenantID.FieldName != "TenantID" || !tenantID.SelectDisabled {
		t.Fatalf("unexpected tenant column %#v", tenantID)
	}

	rendered, err := Render(model)
	if err != nil {
		t.Fatalf("render reverse model failed: %v", err)
	}
	source := string(rendered)
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_orm_account_gen.go", rendered, parser.ParseComments); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, source)
	}
	expected := []string{
		`"time"`,
		`type AccountUser struct`,
		`type-handler='json'`,
		`where-strategy='not-null'`,
		`select=false`,
		`version=true`,
		`updated-at=true`,
		`fill='insert_update'`,
	}
	for _, fragment := range expected {
		if !strings.Contains(source, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, source)
		}
	}
	if strings.Contains(source, "PasswordHash") || strings.Contains(source, "ShadowFlag") {
		t.Fatalf("generated source contains filtered columns:\n%s", source)
	}
}

func TestRender_whenReverseTagValueInvalid_shouldReject(t *testing.T) {
	_, err := Render(&PackageModel{
		PackageName: "badtag",
		Entities: []EntityModel{{
			TypeName:      "BadTag",
			Table:         "bad_tag",
			DeclareStruct: true,
			Columns: []ColumnModel{{
				FieldName:  "Name",
				FieldType:  "string",
				ColumnName: "bad'name",
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "contains unsupported character") {
		t.Fatalf("expected invalid tag error, got %v", err)
	}
}

func containsColumn(columns []ColumnModel, columnName string) bool {
	for _, column := range columns {
		if column.ColumnName == columnName {
			return true
		}
	}
	return false
}

func findColumn(t *testing.T, columns []ColumnModel, columnName string) ColumnModel {
	t.Helper()
	for _, column := range columns {
		if column.ColumnName == columnName {
			return column
		}
	}
	t.Fatalf("column %s not found in %#v", columnName, columns)
	return ColumnModel{}
}
