package orm

import (
	"context"
	"strings"
	"testing"
)

func TestDecodeMyBatisConfig_whenJSONProvided_shouldBuildConfig(t *testing.T) {
	source := strings.NewReader(`{
  "properties": {
    "systemPackage": "system",
    "mapperNamespace": "${systemPackage}.user.UserMapper"
  },
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
  "databaseIdProvider": {
    "type": "vendor",
    "properties": {
      "postgres": "postgresql",
      "mysql": "mysql8"
    }
  },
  "typeAliases": [
    {"alias": "User", "typeName": "${systemPackage}.User"}
  ],
  "typeHandlers": [
    {"name": "json"},
    {"name": "profile"}
  ],
  "mappers": [
    {"resource": "mapper/${systemPackage}/user.xml", "namespace": "${mapperNamespace}"}
  ],
  "plugins": [
    {"name": "pagination"},
    {"name": "illegalSQL", "options": {"denySelectWildcard": "false"}}
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
	if len(config.Plugins) != 2 || config.Plugins[1].Name != "illegalSQL" {
		t.Fatalf("unexpected plugins %#v", config.Plugins)
	}
	if config.TypeAliases[0].TypeName != "system.User" || config.Mappers[0].Namespace != "system.user.UserMapper" {
		t.Fatalf("properties were not resolved, aliases=%#v mappers=%#v", config.TypeAliases, config.Mappers)
	}
	runtimeConfig, err := config.BuildConfiguration()
	if err != nil {
		t.Fatalf("build runtime config failed: %v", err)
	}
	if runtimeConfig.DatabaseID != "postgres" {
		t.Fatalf("explicit databaseId should win over provider, got %q", runtimeConfig.DatabaseID)
	}
}

func TestDecodeMyBatisConfig_whenUnknownFieldProvided_shouldReject(t *testing.T) {
	_, err := DecodeMyBatisConfig(strings.NewReader(`{"settings": {}, "unknown": true}`))
	if err == nil {
		t.Fatalf("expected strict decode error")
	}
}

func TestDecodeMyBatisConfig_whenPropertyMissing_shouldReject(t *testing.T) {
	_, err := DecodeMyBatisConfig(strings.NewReader(`{"mappers": [{"namespace": "${missing}"}]}`))
	if err == nil || !strings.Contains(err.Error(), "property") {
		t.Fatalf("expected missing property error, got %v", err)
	}
}

func TestMyBatisConfig_BuildConfiguration_whenDatabaseIDProviderConfigured_shouldUseDbTypeMapping(t *testing.T) {
	config := MyBatisConfig{
		Environment: MyBatisEnvironment{DbType: DbTypePostgres},
		DatabaseIDProvider: DatabaseIDProvider{
			Type:       DatabaseIDProviderVendor,
			Properties: map[string]string{"PostgreSQL": "postgresql", "MySQL": "mysql8"},
		},
	}

	runtimeConfig, err := config.BuildConfiguration()
	if err != nil {
		t.Fatalf("build runtime config failed: %v", err)
	}
	if runtimeConfig.DatabaseID != "postgresql" {
		t.Fatalf("unexpected database id %q", runtimeConfig.DatabaseID)
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

func TestAssembleMyBatisConfig_whenPluginConfigured_shouldApplyStatementInterceptor(t *testing.T) {
	state := openTestSQLState(t)
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{{
			ID:        "UpdateAll",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateAll",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name}",
		}},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	assembled, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config: MyBatisConfig{
			Environment: MyBatisEnvironment{DbType: DbTypePostgres},
			Plugins: []PluginRef{{
				Name: "illegalSQL",
				Options: map[string]string{
					"denyWriteWithoutWhere": "true",
				},
			}},
		},
		Registry: registry,
		DB:       state.db,
	})
	if err != nil {
		t.Fatalf("assemble config failed: %v", err)
	}
	session, err := assembled.SessionFactory.OpenSession()
	if err != nil {
		t.Fatalf("open session failed: %v", err)
	}
	defer session.Close()

	_, err = session.Exec(context.Background(), "system.user.UserMapper.UpdateAll", NamedArgs{"name": "Alice"})
	if err == nil || !strings.Contains(err.Error(), "without WHERE") {
		t.Fatalf("expected illegal SQL plugin to reject update, got %v", err)
	}
}

func TestAssembleMyBatisConfig_whenPluginOptionDisablesRule_shouldRespectOption(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 3}
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{{
			ID:        "UpdateAll",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateAll",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name}",
		}},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	assembled, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config: MyBatisConfig{
			Environment: MyBatisEnvironment{DbType: DbTypePostgres},
			Plugins: []PluginRef{{
				Name: "illegalSQL",
				Options: map[string]string{
					"denyWriteWithoutWhere": "false",
				},
			}},
		},
		Registry: registry,
		DB:       state.db,
	})
	if err != nil {
		t.Fatalf("assemble config failed: %v", err)
	}
	session, err := assembled.SessionFactory.OpenSession()
	if err != nil {
		t.Fatalf("open session failed: %v", err)
	}
	defer session.Close()

	result, err := session.Exec(context.Background(), "system.user.UserMapper.UpdateAll", NamedArgs{"name": "Alice"})
	if err != nil {
		t.Fatalf("expected disabled illegal SQL rule to allow update, got %v", err)
	}
	if result.RowsAffected != 3 {
		t.Fatalf("unexpected rows affected %d", result.RowsAffected)
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
