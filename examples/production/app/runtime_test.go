package app

import (
	"path/filepath"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/examples/production/account"
)

func TestRuntimeConfig_shouldAssembleWithoutDatabase(t *testing.T) {
	registry := orm.NewRegistry()
	if err := account.RegisterGoarkORMMetadata(registry); err != nil {
		t.Fatalf("register metadata failed: %v", err)
	}
	if err := account.RegisterSQLProviders(registry); err != nil {
		t.Fatalf("register providers failed: %v", err)
	}
	config, err := orm.LoadRuntimeConfig(filepath.Join("..", "goark-orm-runtime.json"))
	if err != nil {
		t.Fatalf("load runtime config failed: %v", err)
	}
	assembled, err := orm.AssembleRuntimeConfig(orm.RuntimeAssembly{
		Config:       config,
		Registry:     registry,
		TypeHandlers: map[string]orm.TypeHandler{"json": orm.NewJSONTypeHandler()},
	})
	if err != nil {
		t.Fatalf("assemble runtime config failed: %v", err)
	}
	if assembled.Configuration.DatabaseID != "postgres" {
		t.Fatalf("unexpected database id %q", assembled.Configuration.DatabaseID)
	}
	if assembled.Session != nil || assembled.SessionFactory != nil {
		t.Fatalf("metadata-only assembly should not create sessions")
	}
}
