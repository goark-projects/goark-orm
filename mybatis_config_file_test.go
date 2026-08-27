package orm

import (
	"strings"
	"testing"
)

func TestDecodeMyBatisConfig_whenJSONProvided_shouldBuildConfig(t *testing.T) {
	source := strings.NewReader(`{
  "settings": {
    "cacheEnabled": false,
    "localCacheScope": "STATEMENT",
    "mapUnderscoreToCamelCase": true,
    "defaultExecutorType": "REUSE",
    "defaultStatementTimeout": "2s",
    "defaultFetchSize": 128,
    "databaseId": "postgres"
  },
  "environment": {
    "id": "dev",
    "dbType": "postgres"
  },
  "typeAliases": [
    {"alias": "User", "typeName": "system.User"}
  ],
  "typeHandlers": [
    {"name": "json"},
    {"name": "profile"}
  ],
  "mappers": [
    {"resource": "mapper/user.xml", "namespace": "system.user.UserMapper"}
  ]
}`)

	config, err := DecodeMyBatisConfig(source)
	if err != nil {
		t.Fatalf("decode config failed: %v", err)
	}
	if config.Settings.LocalCacheScope != LocalCacheScopeStatement {
		t.Fatalf("unexpected local cache scope %q", config.Settings.LocalCacheScope)
	}
	if config.Settings.DefaultStatementTimeout.String() != "2s" {
		t.Fatalf("unexpected statement timeout %s", config.Settings.DefaultStatementTimeout)
	}
	if config.Environment.DbType != DbTypePostgres {
		t.Fatalf("unexpected db type %q", config.Environment.DbType)
	}
	if len(config.TypeHandlers) != 2 || config.TypeHandlers[1].Name != "profile" {
		t.Fatalf("unexpected type handlers %#v", config.TypeHandlers)
	}
}

func TestDecodeMyBatisConfig_whenUnknownFieldProvided_shouldReject(t *testing.T) {
	_, err := DecodeMyBatisConfig(strings.NewReader(`{"settings": {}, "unknown": true}`))
	if err == nil {
		t.Fatalf("expected strict decode error")
	}
}

func TestAssembleMyBatisConfig_whenRegistryConfigured_shouldCreateFactory(t *testing.T) {
	state := openTestSQLState(t)
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{TypeName: "UserMapper", Namespace: "system.user.UserMapper"}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	config := MyBatisConfig{
		Settings: MyBatisSettings{
			DefaultExecutorType: ExecutorTypeReuse,
			DatabaseID:          "postgres",
		},
		Environment: MyBatisEnvironment{DbType: DbTypePostgres},
		TypeHandlers: []TypeHandlerRef{
			{Name: "json"},
			{Name: "profile"},
		},
		Mappers: []MapperRef{{Namespace: "system.user.UserMapper"}},
	}

	assembled, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config:       config,
		Registry:     registry,
		DB:           state.db,
		TypeHandlers: map[string]TypeHandler{"profile": NewJSONTypeHandler()},
	})
	if err != nil {
		t.Fatalf("assemble config failed: %v", err)
	}
	if assembled.SessionFactory == nil {
		t.Fatalf("expected session factory")
	}
	session, err := assembled.SessionFactory.OpenSession()
	if err != nil {
		t.Fatalf("open session failed: %v", err)
	}
	defer session.Close()
	if session.Dialect().Name() != "postgres" {
		t.Fatalf("unexpected dialect %s", session.Dialect().Name())
	}
	if assembled.TypeAliases == nil || len(assembled.Mappers) != 1 {
		t.Fatalf("unexpected assembly result %#v", assembled)
	}
}

func TestAssembleMyBatisConfig_whenTypeHandlerMissing_shouldReject(t *testing.T) {
	_, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config:   MyBatisConfig{TypeHandlers: []TypeHandlerRef{{Name: "missing"}}},
		Registry: NewRegistry(),
	})
	if err == nil {
		t.Fatalf("expected missing type-handler error")
	}
}

func TestAssembleMyBatisConfig_whenMapperMissing_shouldReject(t *testing.T) {
	_, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config:   MyBatisConfig{Mappers: []MapperRef{{Namespace: "missing.Mapper"}}},
		Registry: NewRegistry(),
	})
	if err == nil {
		t.Fatalf("expected missing mapper error")
	}
}
