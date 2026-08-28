package orm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeConfigAliases_whenUsed_shouldMatchExistingRuntimeConfigBehavior(t *testing.T) {
	t.Parallel()

	config := DefaultRuntimeConfig()
	config.Environment = RuntimeEnvironment{DbType: DbTypePostgres}
	config.Settings = RuntimeSettings{DefaultExecutorType: ExecutorTypeReuse}

	runtimeConfig, err := config.BuildConfiguration()
	if err != nil {
		t.Fatalf("build runtime configuration failed: %v", err)
	}
	if runtimeConfig.Dialect.Name() != "postgres" {
		t.Fatalf("unexpected dialect %s", runtimeConfig.Dialect.Name())
	}
	if runtimeConfig.DefaultExecutorType != ExecutorTypeReuse {
		t.Fatalf("unexpected executor type %s", runtimeConfig.DefaultExecutorType)
	}
}

func TestLoadAndAssembleRuntimeConfig_whenFileProvided_shouldCreateSessionFactory(t *testing.T) {
	state := openTestSQLState(t)
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{TypeName: "UserMapper", Namespace: "example.user.UserMapper"}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "goark-orm-runtime.json")
	if err := os.WriteFile(path, []byte(`{
  "settings": {"defaultExecutorType": "REUSE"},
  "environment": {"dbType": "postgres"},
  "mappers": [{"namespace": "example.user.UserMapper"}]
}`), 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	assembled, err := LoadAndAssembleRuntimeConfig(path, RuntimeAssembly{
		Registry: registry,
		DB:       state.db,
	})
	if err != nil {
		t.Fatalf("load and assemble runtime config failed: %v", err)
	}
	if assembled.Session == nil || assembled.SessionFactory == nil {
		t.Fatalf("expected session and factory, got %#v", assembled)
	}
	defer assembled.Session.Close()
	if assembled.Session.Configuration().DefaultExecutorType != ExecutorTypeReuse {
		t.Fatalf("runtime configuration was not applied")
	}
}
