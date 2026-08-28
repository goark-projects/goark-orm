package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMyBatisConfig_BuildConfiguration_whenGoNativeModelProvided_shouldApplyRuntimeSettings(t *testing.T) {
	t.Parallel()

	cacheEnabled := false
	localCacheEnabled := false
	useColumnLabel := false
	nullableOnForEach := false
	config, err := MyBatisConfig{
		Settings: MyBatisSettings{
			CacheEnabled:                     &cacheEnabled,
			LocalCacheEnabled:                &localCacheEnabled,
			LocalCacheScope:                  LocalCacheScopeStatement,
			MapUnderscoreToCamelCase:         true,
			UseGeneratedKeys:                 true,
			LazyLoadingEnabled:               true,
			DefaultExecutorType:              ExecutorTypeReuse,
			PreparedStatementCacheSize:       64,
			DefaultStatementTimeout:          3 * time.Second,
			DefaultFetchSize:                 128,
			DefaultResultSetType:             ResultSetTypeForwardOnly,
			UseColumnLabel:                   &useColumnLabel,
			NullableOnForEach:                &nullableOnForEach,
			ShrinkWhitespacesInSQL:           true,
			JDBCTypeForNull:                  "NULL",
			AutoMappingBehavior:              AutoMappingBehaviorNone,
			AutoMappingUnknownColumnBehavior: AutoMappingUnknownColumnBehaviorFailing,
			DatabaseID:                       "mysql8",
		},
		Environment: MyBatisEnvironment{
			ID:     "prod",
			DbType: DbTypeMySQL,
		},
		Global: GlobalConfig{
			DbConfig: DbConfig{
				IDType:      IDTypeAssignID,
				TablePrefix: "sys_",
			},
		},
		TypeAliases: []TypeAlias{{Alias: "User", TypeName: "system.User"}},
		Mappers:     []MapperRef{{Resource: "mapper/user_mapper.xml", Namespace: "system.user.UserMapper"}},
	}.BuildConfiguration()
	if err != nil {
		t.Fatalf("build configuration failed: %v", err)
	}

	if config.Dialect.Name() != string(DbTypeMySQL) {
		t.Fatalf("unexpected dialect %q", config.Dialect.Name())
	}
	if boolValue(config.CacheEnabled, true) {
		t.Fatalf("expected cache to be disabled")
	}
	if boolValue(config.LocalCacheEnabled, true) {
		t.Fatalf("expected local cache to be disabled")
	}
	if config.LocalCacheScope != LocalCacheScopeStatement {
		t.Fatalf("unexpected local cache scope %q", config.LocalCacheScope)
	}
	if !config.MapUnderscoreToCamelCase || !config.UseGeneratedKeys || !config.LazyLoadingEnabled {
		t.Fatalf("expected boolean settings to be applied")
	}
	if config.DefaultExecutorType != ExecutorTypeReuse {
		t.Fatalf("unexpected executor type %q", config.DefaultExecutorType)
	}
	if config.PreparedStatementCacheSize != 64 {
		t.Fatalf("unexpected prepared statement cache size %d", config.PreparedStatementCacheSize)
	}
	if config.DefaultStatementTimeout != 3*time.Second || config.DefaultFetchSize != 128 {
		t.Fatalf("unexpected timeout or fetch size")
	}
	if config.DefaultResultSetType != ResultSetTypeForwardOnly {
		t.Fatalf("unexpected default result set type %q", config.DefaultResultSetType)
	}
	if boolValue(config.UseColumnLabel, true) || boolValue(config.NullableOnForEach, true) {
		t.Fatalf("expected nullable/useColumnLabel settings to be disabled")
	}
	if !config.ShrinkWhitespacesInSQL || config.JDBCTypeForNull != "NULL" {
		t.Fatalf("unexpected SQL whitespace or JDBC null settings")
	}
	if config.AutoMappingBehavior != AutoMappingBehaviorNone ||
		config.AutoMappingUnknownColumnBehavior != AutoMappingUnknownColumnBehaviorFailing {
		t.Fatalf("unexpected auto mapping settings")
	}
	if config.DatabaseID != "mysql8" {
		t.Fatalf("unexpected database id %q", config.DatabaseID)
	}
	if config.GlobalConfig.DbConfig.IDType != IDTypeAssignID || config.GlobalConfig.DbConfig.TablePrefix != "sys_" {
		t.Fatalf("unexpected global db config %#v", config.GlobalConfig.DbConfig)
	}
}

func TestMyBatisConfig_BuildConfiguration_whenCompatibilitySettingsProvided_shouldCarryStableSettings(t *testing.T) {
	t.Parallel()

	safeResultHandler := false
	useActualParamName := false
	config, err := MyBatisConfig{
		Settings: MyBatisSettings{
			SafeRowBoundsEnabled:               true,
			SafeResultHandlerEnabled:           &safeResultHandler,
			AggressiveLazyLoading:              true,
			LazyLoadTriggerMethods:             []string{"Close", "String"},
			DefaultScriptingLanguage:           "goarkXML",
			DefaultEnumTypeHandler:             "enum",
			CallSettersOnNulls:                 true,
			ReturnInstanceForEmptyRow:          true,
			LogPrefix:                          "orm.",
			LogImpl:                            "slog",
			ProxyFactory:                       "none",
			VFSImpl:                            []string{"goark.dev/orm/internal/vfs"},
			UseActualParamName:                 &useActualParamName,
			ConfigurationFactory:               "goark.dev/app.NewORMConfiguration",
			DefaultSQLProviderType:             "goark.dev/app.SQLProvider",
			ArgNameBasedConstructorAutoMapping: true,
		},
	}.BuildConfiguration()
	if err != nil {
		t.Fatalf("build configuration failed: %v", err)
	}

	if !config.SafeRowBoundsEnabled || boolValue(config.SafeResultHandlerEnabled, true) {
		t.Fatalf("safe settings were not applied: %#v", config)
	}
	if !config.AggressiveLazyLoading || strings.Join(config.LazyLoadTriggerMethods, ",") != "Close,String" {
		t.Fatalf("lazy loading compatibility settings were not applied: %#v", config.LazyLoadTriggerMethods)
	}
	if config.DefaultScriptingLanguage != "goarkXML" || config.DefaultEnumTypeHandler != "enum" {
		t.Fatalf("default handler settings were not applied")
	}
	if !config.CallSettersOnNulls || !config.ReturnInstanceForEmptyRow {
		t.Fatalf("null or empty-row settings were not applied")
	}
	if config.LogPrefix != "orm." || config.LogImpl != "slog" || config.ProxyFactory != "none" {
		t.Fatalf("logging/proxy settings were not applied")
	}
	if len(config.VFSImpl) != 1 || config.VFSImpl[0] != "goark.dev/orm/internal/vfs" {
		t.Fatalf("unexpected vfs settings %#v", config.VFSImpl)
	}
	if boolValue(config.UseActualParamName, true) {
		t.Fatalf("useActualParamName should be explicitly disabled")
	}
	if config.ConfigurationFactory != "goark.dev/app.NewORMConfiguration" ||
		config.DefaultSQLProviderType != "goark.dev/app.SQLProvider" ||
		!config.ArgNameBasedConstructorAutoMapping {
		t.Fatalf("provider/constructor compatibility settings were not applied")
	}
}

func TestMyBatisConfig_Validate_whenDuplicateAliasOrMapperProvided_shouldReject(t *testing.T) {
	t.Parallel()

	_, err := MyBatisConfig{
		TypeAliases: []TypeAlias{
			{Alias: "User", TypeName: "system.User"},
			{Alias: "user", TypeName: "other.User"},
		},
	}.BuildConfiguration()
	if err == nil || !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected configuration error for duplicate alias, got %v", err)
	}

	_, err = MyBatisConfig{
		Mappers: []MapperRef{
			{Namespace: "system.user.UserMapper"},
			{Namespace: "system.user.UserMapper"},
		},
	}.BuildConfiguration()
	if err == nil || !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected configuration error for duplicate mapper, got %v", err)
	}
}

func TestMyBatisConfig_Validate_whenPreparedStatementCacheSizeNegative_shouldReject(t *testing.T) {
	t.Parallel()

	_, err := MyBatisConfig{
		Settings: MyBatisSettings{PreparedStatementCacheSize: -1},
	}.BuildConfiguration()
	if err == nil || !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected configuration error for negative prepared statement cache size, got %v", err)
	}
}

func TestParseConfigurationEnums_whenValuesProvided_shouldMatchMyBatisNames(t *testing.T) {
	t.Parallel()

	scope, err := ParseLocalCacheScope("statement")
	if err != nil {
		t.Fatalf("parse local cache scope failed: %v", err)
	}
	if scope != LocalCacheScopeStatement {
		t.Fatalf("unexpected scope %q", scope)
	}
	executor, err := ParseExecutorType("batch")
	if err != nil {
		t.Fatalf("parse executor type failed: %v", err)
	}
	if executor != ExecutorTypeBatch {
		t.Fatalf("unexpected executor type %q", executor)
	}
	resultSetType, err := ParseResultSetType("default")
	if err != nil {
		t.Fatalf("parse default result set type failed: %v", err)
	}
	if resultSetType != "" {
		t.Fatalf("DEFAULT result set type should map to empty runtime hint, got %q", resultSetType)
	}
	autoMapping, err := ParseAutoMappingBehavior("partial")
	if err != nil {
		t.Fatalf("parse auto mapping behavior failed: %v", err)
	}
	if autoMapping != AutoMappingBehaviorPartial {
		t.Fatalf("unexpected auto mapping behavior %q", autoMapping)
	}
	unknownColumn, err := ParseAutoMappingUnknownColumnBehavior("failing")
	if err != nil {
		t.Fatalf("parse unknown column behavior failed: %v", err)
	}
	if unknownColumn != AutoMappingUnknownColumnBehaviorFailing {
		t.Fatalf("unexpected unknown column behavior %q", unknownColumn)
	}
}
