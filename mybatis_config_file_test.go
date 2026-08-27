package orm

import (
	"context"
	"os"
	"path/filepath"
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
    "preparedStatementCacheSize": 64,
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
  "global": {
    "dbConfig": {
      "idType": "assign_id",
      "tablePrefix": "${systemPackage}_",
      "schema": "public",
      "logicDeleteField": "Deleted",
      "logicDeleteValue": 1,
      "logicNotDeleteValue": 0,
      "insertStrategy": "not_empty",
      "updateStrategy": "not_null",
      "whereStrategy": "not_zero"
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
	if config.Settings.PreparedStatementCacheSize != 64 {
		t.Fatalf("unexpected prepared statement cache size %d", config.Settings.PreparedStatementCacheSize)
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
	if config.Global.DbConfig.IDType != IDTypeAssignID || config.Global.DbConfig.TablePrefix != "system_" {
		t.Fatalf("unexpected global db config %#v", config.Global.DbConfig)
	}
	if config.Global.DbConfig.InsertStrategy != FieldStrategyNotEmpty ||
		config.Global.DbConfig.UpdateStrategy != FieldStrategyNotNull ||
		config.Global.DbConfig.WhereStrategy != FieldStrategyNotZero {
		t.Fatalf("unexpected field strategies %#v", config.Global.DbConfig)
	}
	runtimeConfig, err := config.BuildConfiguration()
	if err != nil {
		t.Fatalf("build runtime config failed: %v", err)
	}
	if runtimeConfig.DatabaseID != "postgres" {
		t.Fatalf("explicit databaseId should win over provider, got %q", runtimeConfig.DatabaseID)
	}
	if runtimeConfig.GlobalConfig.DbConfig.Schema != "public" ||
		runtimeConfig.GlobalConfig.DbConfig.LogicDeleteField != "Deleted" {
		t.Fatalf("global db config not applied %#v", runtimeConfig.GlobalConfig.DbConfig)
	}
}

func TestLoadAndAssembleMyBatisConfig_whenFileProvided_shouldReturnFactoryAndSession(t *testing.T) {
	state := openTestSQLState(t)
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{TypeName: "UserMapper", Namespace: "system.user.UserMapper"}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "orm-runtime.json")
	if err := os.WriteFile(path, []byte(`{
  "settings": {"defaultExecutorType": "REUSE"},
  "environment": {"dbType": "postgres"},
  "mappers": [{"namespace": "system.user.UserMapper"}]
}`), 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	assembled, err := LoadAndAssembleMyBatisConfig(path, MyBatisAssembly{
		Registry: registry,
		DB:       state.db,
	})
	if err != nil {
		t.Fatalf("load and assemble config failed: %v", err)
	}
	if assembled.Session == nil || assembled.SessionFactory == nil {
		t.Fatalf("expected session and factory, got %#v", assembled)
	}
	defer assembled.Session.Close()
	if assembled.Session.Configuration().DefaultExecutorType != ExecutorTypeReuse {
		t.Fatalf("runtime configuration was not applied")
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

func TestDecodeMyBatisConfig_whenGlobalAliasesBothProvided_shouldReject(t *testing.T) {
	_, err := DecodeMyBatisConfig(strings.NewReader(`{
  "global": {"dbConfig": {"idType": "input"}},
  "globalConfig": {"dbConfig": {"idType": "assign_id"}}
}`))
	if err == nil || !strings.Contains(err.Error(), "global and globalConfig") {
		t.Fatalf("expected duplicate global config error, got %v", err)
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
	if assembled.Session == nil {
		t.Fatalf("expected default session")
	}
	defer assembled.Session.Close()
	if assembled.Session.Dialect().Name() != "postgres" {
		t.Fatalf("unexpected default session dialect %s", assembled.Session.Dialect().Name())
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

func TestAssembleMyBatisConfig_whenDBMissing_shouldNotCreateRuntimeSessions(t *testing.T) {
	registry := NewRegistry()

	assembled, err := AssembleMyBatisConfig(MyBatisAssembly{
		Config: MyBatisConfig{
			Environment: MyBatisEnvironment{DbType: DbTypePostgres},
		},
		Registry: registry,
	})
	if err != nil {
		t.Fatalf("assemble config failed: %v", err)
	}
	if assembled.Session != nil || assembled.SessionFactory != nil {
		t.Fatalf("expected no runtime sessions without DB, got %#v", assembled)
	}
	if assembled.Configuration.Dialect.Name() != "postgres" {
		t.Fatalf("unexpected dialect %s", assembled.Configuration.Dialect.Name())
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
