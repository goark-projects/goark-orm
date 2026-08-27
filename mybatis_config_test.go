package orm

import (
	"errors"
	"testing"
	"time"
)

func TestMyBatisConfig_BuildConfiguration_whenGoNativeModelProvided_shouldApplyRuntimeSettings(t *testing.T) {
	t.Parallel()

	cacheEnabled := false
	localCacheEnabled := false
	config, err := MyBatisConfig{
		Settings: MyBatisSettings{
			CacheEnabled:               &cacheEnabled,
			LocalCacheEnabled:          &localCacheEnabled,
			LocalCacheScope:            LocalCacheScopeStatement,
			MapUnderscoreToCamelCase:   true,
			UseGeneratedKeys:           true,
			LazyLoadingEnabled:         true,
			DefaultExecutorType:        ExecutorTypeReuse,
			PreparedStatementCacheSize: 64,
			DefaultStatementTimeout:    3 * time.Second,
			DefaultFetchSize:           128,
			DatabaseID:                 "mysql8",
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
	if config.DatabaseID != "mysql8" {
		t.Fatalf("unexpected database id %q", config.DatabaseID)
	}
	if config.GlobalConfig.DbConfig.IDType != IDTypeAssignID || config.GlobalConfig.DbConfig.TablePrefix != "sys_" {
		t.Fatalf("unexpected global db config %#v", config.GlobalConfig.DbConfig)
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
}
