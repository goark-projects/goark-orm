package ormgen

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateWithRenderer_whenCustomRendererProvided_shouldUseRenderer(t *testing.T) {
	dir := t.TempDir()
	source := []byte("package sample\n\ntype User struct{}\n")
	if err := os.WriteFile(filepath.Join(dir, "user.go"), source, 0o600); err != nil {
		t.Fatalf("write source failed: %v", err)
	}
	called := false
	renderer := TemplateRendererFunc(func(model *PackageModel) ([]byte, error) {
		called = true
		if model.PackageName != "sample" {
			t.Fatalf("unexpected package name %q", model.PackageName)
		}
		return []byte("custom"), nil
	})

	out, err := GenerateWithRenderer(GenerateSpec{Dir: dir}, renderer)
	if err != nil {
		t.Fatalf("generate with renderer failed: %v", err)
	}

	if !called {
		t.Fatalf("expected custom renderer to be called")
	}
	if string(out) != "custom" {
		t.Fatalf("unexpected rendered output %q", out)
	}
}

func TestReverseEngineerWithRenderer_whenCustomRendererProvided_shouldUseRenderer(t *testing.T) {
	introspector := &fakeSchemaIntrospector{
		schema: SchemaModel{
			Tables: []SchemaTable{
				{Name: "sys_user", Columns: []SchemaColumn{{Name: "id", DBType: "bigint", PrimaryKey: true}}},
			},
		},
	}
	called := false
	renderer := TemplateRendererFunc(func(model *PackageModel) ([]byte, error) {
		called = true
		if model.PackageName != "sample" || len(model.Entities) != 1 {
			t.Fatalf("unexpected model %#v", model)
		}
		return []byte("reverse"), nil
	})

	out, err := ReverseEngineerWithRenderer(context.Background(), introspector, ReverseEngineerSpec{PackageName: "sample"}, renderer)
	if err != nil {
		t.Fatalf("reverse engineer with renderer failed: %v", err)
	}

	if !called {
		t.Fatalf("expected renderer to be called")
	}
	if string(out) != "reverse" {
		t.Fatalf("unexpected rendered output %q", out)
	}
}
