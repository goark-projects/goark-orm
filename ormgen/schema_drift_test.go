package ormgen

import (
	"context"
	"errors"
	"reflect"
	"testing"

	orm "goark.dev/orm"
)

func TestCompareSchemaDrift_whenSchemaMatchesRegistry_shouldReturnCleanReport(t *testing.T) {
	nullable := false
	size := 64
	registry := newSchemaDriftRegistry(t, orm.EntityMeta{
		TypeName: "User",
		Table:    "sys_user",
		Columns: []orm.ColumnMeta{
			{FieldName: "ID", ColumnName: "id", PrimaryKey: true, AutoIncrement: true, Nullable: &nullable, DBType: "bigint"},
			{FieldName: "Name", ColumnName: "user_name", Nullable: &nullable, Size: &size, DBType: "varchar"},
		},
	})

	report, err := CompareSchemaDrift(registry, SchemaModel{Tables: []SchemaTable{
		{
			Name: "sys_user",
			Columns: []SchemaColumn{
				{Name: "id", PrimaryKey: true, AutoIncrement: true, Nullable: &nullable, DBType: "int8"},
				{Name: "user_name", Nullable: &nullable, Size: &size, DBType: "character varying"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("compare schema drift failed: %v", err)
	}
	if report.HasDrift() {
		t.Fatalf("unexpected drift %#v", report.Issues)
	}
}

func TestCompareSchemaDrift_whenTableOrColumnDiffers_shouldReportIssues(t *testing.T) {
	nullable := false
	schemaNullable := true
	size := 64
	registry := newSchemaDriftRegistry(t,
		orm.EntityMeta{
			TypeName: "User",
			Table:    "public.sys_user",
			Columns: []orm.ColumnMeta{
				{FieldName: "ID", ColumnName: "id", PrimaryKey: true, AutoIncrement: true, Nullable: &nullable, DBType: "bigint"},
				{FieldName: "Name", ColumnName: "user_name", Nullable: &nullable, Size: &size, DBType: "varchar"},
				{FieldName: "Status", ColumnName: "status"},
			},
		},
		orm.EntityMeta{TypeName: "Role", Table: "sys_role", Columns: []orm.ColumnMeta{{FieldName: "ID", ColumnName: "id"}}},
	)

	report, err := CompareSchemaDrift(registry, SchemaModel{Tables: []SchemaTable{
		{
			Name: "sys_user",
			Columns: []SchemaColumn{
				{Name: "id", PrimaryKey: false, AutoIncrement: false, Nullable: &schemaNullable, DBType: "bigint"},
				{Name: "user_name", Nullable: &nullable, Size: &size, DBType: "text"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("compare schema drift failed: %v", err)
	}

	kinds := schemaDriftKinds(report.Issues)
	expected := []SchemaDriftKind{
		SchemaDriftMissingTable,
		SchemaDriftAutoIncrementMismatch,
		SchemaDriftNullableMismatch,
		SchemaDriftPrimaryKeyMismatch,
		SchemaDriftMissingColumn,
		SchemaDriftTypeMismatch,
	}
	if !reflect.DeepEqual(kinds, expected) {
		t.Fatalf("unexpected drift kinds %#v", kinds)
	}
	if report.Error() == "" {
		t.Fatalf("expected report error summary")
	}
}

func TestCompareSchemaDrift_whenExtraColumnEnabled_shouldReportUnmappedColumn(t *testing.T) {
	registry := newSchemaDriftRegistry(t, orm.EntityMeta{
		TypeName: "User",
		Table:    "sys_user",
		Columns:  []orm.ColumnMeta{{FieldName: "ID", ColumnName: "id"}},
	})

	report, err := CompareSchemaDrift(registry, SchemaModel{Tables: []SchemaTable{
		{Name: "sys_user", Columns: []SchemaColumn{{Name: "id"}, {Name: "shadow_col"}}},
	}}, WithSchemaDriftExtraColumns(true))
	if err != nil {
		t.Fatalf("compare schema drift failed: %v", err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Kind != SchemaDriftExtraColumn || report.Issues[0].Column != "shadow_col" {
		t.Fatalf("unexpected extra column report %#v", report.Issues)
	}
}

func TestDetectSchemaDrift_whenTablesNotSpecified_shouldUseRegistryTables(t *testing.T) {
	registry := newSchemaDriftRegistry(t, orm.EntityMeta{
		TypeName: "User",
		Table:    "`sys_user`",
		Columns:  []orm.ColumnMeta{{FieldName: "ID", ColumnName: "id"}},
	})
	introspector := &fakeSchemaIntrospector{
		schema: SchemaModel{Tables: []SchemaTable{{Name: "sys_user", Columns: []SchemaColumn{{Name: "id"}}}}},
	}

	report, err := DetectSchemaDrift(context.Background(), introspector, registry, SchemaIntrospectionRequest{Schema: "public"})
	if err != nil {
		t.Fatalf("detect schema drift failed: %v", err)
	}
	if report.HasDrift() {
		t.Fatalf("unexpected drift %#v", report.Issues)
	}
	expectedRequest := SchemaIntrospectionRequest{Schema: "public", Tables: []string{"sys_user"}}
	if !reflect.DeepEqual(introspector.request, expectedRequest) {
		t.Fatalf("unexpected introspection request %#v", introspector.request)
	}
}

func TestValidateSchemaDrift_whenReportHasIssues_shouldReturnReportError(t *testing.T) {
	registry := newSchemaDriftRegistry(t, orm.EntityMeta{
		TypeName: "User",
		Table:    "sys_user",
		Columns:  []orm.ColumnMeta{{FieldName: "ID", ColumnName: "id"}},
	})
	introspector := &fakeSchemaIntrospector{schema: SchemaModel{}}

	err := ValidateSchemaDrift(context.Background(), introspector, registry, SchemaIntrospectionRequest{})
	if err == nil {
		t.Fatalf("expected schema drift error")
	}
	var report SchemaDriftReport
	if !errors.As(err, &report) {
		t.Fatalf("expected SchemaDriftReport error, got %T", err)
	}
	if !report.HasDrift() {
		t.Fatalf("expected report with drift")
	}
}

func newSchemaDriftRegistry(t *testing.T, entities ...orm.EntityMeta) *orm.Registry {
	t.Helper()
	registry := orm.NewRegistry()
	for _, entity := range entities {
		if err := registry.RegisterEntity(entity); err != nil {
			t.Fatalf("register entity failed: %v", err)
		}
	}
	return registry
}

func schemaDriftKinds(issues []SchemaDriftIssue) []SchemaDriftKind {
	out := make([]SchemaDriftKind, 0, len(issues))
	for _, issue := range issues {
		out = append(out, issue.Kind)
	}
	return out
}
