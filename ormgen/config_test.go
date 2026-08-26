package ormgen

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateConfig_Resolve_whenPackagesProvided_shouldApplyDefaultsAndResolvePaths(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "goark-orm.json")
	if err := os.WriteFile(configPath, []byte(`{
  "databaseId": "postgres",
  "typeHandlers": ["json"],
  "packages": [
    {
      "dir": "internal/user",
      "output": "internal/user/zz_goark_orm_user_gen.go",
      "typeHandlers": ["decimal", "json"]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	config, err := LoadGenerateConfig(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	items, err := config.Resolve(filepath.Dir(configPath))
	if err != nil {
		t.Fatalf("resolve config failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("unexpected item count %d", len(items))
	}
	item := items[0]
	if item.Spec.Dir != filepath.Join(root, "internal", "user") {
		t.Fatalf("unexpected dir %q", item.Spec.Dir)
	}
	if item.Output != filepath.Join(root, "internal", "user", "zz_goark_orm_user_gen.go") {
		t.Fatalf("unexpected output %q", item.Output)
	}
	if item.Spec.DatabaseID != "postgres" {
		t.Fatalf("unexpected database id %q", item.Spec.DatabaseID)
	}
	if !reflect.DeepEqual(item.Spec.TypeHandlers, []string{"json", "decimal"}) {
		t.Fatalf("unexpected type handlers %#v", item.Spec.TypeHandlers)
	}
}

func TestGenerateConfig_Resolve_whenTopLevelSinglePackageProvided_shouldResolveOneSpec(t *testing.T) {
	root := t.TempDir()
	config := GenerateConfig{
		Dir:          "internal/user",
		Output:       "gen/user.go",
		PackageName:  "sample",
		TypeHandlers: []string{"json"},
	}

	items, err := config.Resolve(root)
	if err != nil {
		t.Fatalf("resolve config failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("unexpected item count %d", len(items))
	}
	if items[0].Spec.PackageName != "sample" {
		t.Fatalf("unexpected package %q", items[0].Spec.PackageName)
	}
	if items[0].Spec.Dir != filepath.Join(root, "internal", "user") {
		t.Fatalf("unexpected dir %q", items[0].Spec.Dir)
	}
	if items[0].Output != filepath.Join(root, "gen", "user.go") {
		t.Fatalf("unexpected output %q", items[0].Output)
	}
}

func TestGenerateConfig_Resolve_whenTopLevelOutputWithMultiplePackages_shouldReject(t *testing.T) {
	config := GenerateConfig{
		Output: "zz_goark_orm_gen.go",
		Packages: []GeneratePackageSpec{
			{Dir: "a"},
			{Dir: "b"},
		},
	}

	_, err := config.Resolve(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "top-level output") {
		t.Fatalf("expected top-level output error, got %v", err)
	}
}

func TestLoadGenerateConfig_whenUnknownFieldProvided_shouldReject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goark-orm.json")
	if err := os.WriteFile(path, []byte(`{"unknown": true}`), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	_, err := LoadGenerateConfig(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}
