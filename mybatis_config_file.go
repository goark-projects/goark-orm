package orm

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"goark.dev/orm/internal/jsoncodec"
)

// MyBatisConfigFile 描述可提交的 ORM 运行期 JSON 配置文件。
type MyBatisConfigFile struct {
	Properties         ConfigProperties         `json:"properties,omitempty"`
	Settings           MyBatisSettingsFile      `json:"settings,omitempty"`
	Environment        MyBatisEnvironmentFile   `json:"environment,omitempty"`
	DatabaseIDProvider DatabaseIDProviderFile   `json:"databaseIdProvider,omitempty"`
	TypeAliases        []TypeAlias              `json:"typeAliases,omitempty"`
	TypeHandlers       []TypeHandlerRef         `json:"typeHandlers,omitempty"`
	Mappers            []MapperRef              `json:"mappers,omitempty"`
	Plugins            []PluginRef              `json:"plugins,omitempty"`
	Global             *MyBatisGlobalConfigFile `json:"global,omitempty"`
	GlobalConfig       *MyBatisGlobalConfigFile `json:"globalConfig,omitempty"`
}

// MyBatisSettingsFile 使用字符串承载需要解析的枚举和时间配置。
type MyBatisSettingsFile struct {
	CacheEnabled                       *bool    `json:"cacheEnabled,omitempty"`
	LocalCacheEnabled                  *bool    `json:"localCacheEnabled,omitempty"`
	UseColumnLabel                     *bool    `json:"useColumnLabel,omitempty"`
	LocalCacheScope                    string   `json:"localCacheScope,omitempty"`
	MapUnderscoreToCamelCase           bool     `json:"mapUnderscoreToCamelCase,omitempty"`
	UseGeneratedKeys                   bool     `json:"useGeneratedKeys,omitempty"`
	LazyLoadingEnabled                 bool     `json:"lazyLoadingEnabled,omitempty"`
	DefaultExecutorType                string   `json:"defaultExecutorType,omitempty"`
	PreparedStatementCacheSize         int      `json:"preparedStatementCacheSize,omitempty"`
	DefaultStatementTimeout            string   `json:"defaultStatementTimeout,omitempty"`
	DefaultFetchSize                   int      `json:"defaultFetchSize,omitempty"`
	DefaultResultSetType               string   `json:"defaultResultSetType,omitempty"`
	NullableOnForEach                  *bool    `json:"nullableOnForEach,omitempty"`
	ShrinkWhitespacesInSQL             bool     `json:"shrinkWhitespacesInSql,omitempty"`
	JDBCTypeForNull                    string   `json:"jdbcTypeForNull,omitempty"`
	AutoMappingBehavior                string   `json:"autoMappingBehavior,omitempty"`
	AutoMappingUnknownColumnBehavior   string   `json:"autoMappingUnknownColumnBehavior,omitempty"`
	DatabaseID                         string   `json:"databaseId,omitempty"`
	SafeRowBoundsEnabled               bool     `json:"safeRowBoundsEnabled,omitempty"`
	SafeResultHandlerEnabled           *bool    `json:"safeResultHandlerEnabled,omitempty"`
	AggressiveLazyLoading              bool     `json:"aggressiveLazyLoading,omitempty"`
	LazyLoadTriggerMethods             []string `json:"lazyLoadTriggerMethods,omitempty"`
	DefaultScriptingLanguage           string   `json:"defaultScriptingLanguage,omitempty"`
	DefaultEnumTypeHandler             string   `json:"defaultEnumTypeHandler,omitempty"`
	CallSettersOnNulls                 bool     `json:"callSettersOnNulls,omitempty"`
	ReturnInstanceForEmptyRow          bool     `json:"returnInstanceForEmptyRow,omitempty"`
	LogPrefix                          string   `json:"logPrefix,omitempty"`
	LogImpl                            string   `json:"logImpl,omitempty"`
	ProxyFactory                       string   `json:"proxyFactory,omitempty"`
	VFSImpl                            []string `json:"vfsImpl,omitempty"`
	UseActualParamName                 *bool    `json:"useActualParamName,omitempty"`
	ConfigurationFactory               string   `json:"configurationFactory,omitempty"`
	DefaultSQLProviderType             string   `json:"defaultSqlProviderType,omitempty"`
	ArgNameBasedConstructorAutoMapping bool     `json:"argNameBasedConstructorAutoMapping,omitempty"`
}

// MyBatisEnvironmentFile 描述 JSON 中的数据库环境。
type MyBatisEnvironmentFile struct {
	ID         string `json:"id,omitempty"`
	DbType     string `json:"dbType,omitempty"`
	DatabaseID string `json:"databaseId,omitempty"`
}

// DatabaseIDProviderFile 描述 JSON 中的 databaseIdProvider。
type DatabaseIDProviderFile struct {
	Type       string           `json:"type,omitempty"`
	Properties ConfigProperties `json:"properties,omitempty"`
	DefaultID  string           `json:"defaultId,omitempty"`
}

// LoadMyBatisConfig 从 JSON 文件读取运行期配置声明。
func LoadMyBatisConfig(path string) (MyBatisConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return MyBatisConfig{}, configurationErrorf("config path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return MyBatisConfig{}, err
	}
	defer file.Close()
	return DecodeMyBatisConfig(file)
}

// DecodeMyBatisConfig 从 Reader 严格解码运行期配置声明。
func DecodeMyBatisConfig(reader io.Reader) (MyBatisConfig, error) {
	if reader == nil {
		return MyBatisConfig{}, configurationErrorf("config reader is nil")
	}
	var file MyBatisConfigFile
	if err := jsoncodec.DecodeStrict(reader, &file); err != nil {
		return MyBatisConfig{}, fmt.Errorf("goark-orm: decode runtime config failed: %w", err)
	}
	return file.Build()
}

// Build 将 JSON 文档模型转换为运行期声明模型。
func (f MyBatisConfigFile) Build() (MyBatisConfig, error) {
	resolver, err := newConfigPropertyResolver(f.Properties)
	if err != nil {
		return MyBatisConfig{}, err
	}
	resolved, err := f.resolveProperties(resolver)
	if err != nil {
		return MyBatisConfig{}, err
	}
	settings, err := resolved.Settings.Build()
	if err != nil {
		return MyBatisConfig{}, err
	}
	environment, err := resolved.Environment.Build()
	if err != nil {
		return MyBatisConfig{}, err
	}
	provider, err := resolved.DatabaseIDProvider.Build()
	if err != nil {
		return MyBatisConfig{}, err
	}
	global, err := resolved.BuildGlobalConfig()
	if err != nil {
		return MyBatisConfig{}, err
	}
	config := MyBatisConfig{
		Properties:         copyConfigProperties(resolved.Properties),
		Settings:           settings,
		Environment:        environment,
		DatabaseIDProvider: provider,
		TypeAliases:        append([]TypeAlias(nil), resolved.TypeAliases...),
		TypeHandlers:       append([]TypeHandlerRef(nil), resolved.TypeHandlers...),
		Mappers:            append([]MapperRef(nil), resolved.Mappers...),
		Plugins:            append([]PluginRef(nil), resolved.Plugins...),
		Global:             global,
	}
	if err := config.Validate(); err != nil {
		return MyBatisConfig{}, err
	}
	return config, nil
}

// LoadAndAssembleMyBatisConfig 读取 JSON 配置并完成运行期装配。
func LoadAndAssembleMyBatisConfig(path string, assembly MyBatisAssembly) (MyBatisAssemblyResult, error) {
	config, err := LoadMyBatisConfig(path)
	if err != nil {
		return MyBatisAssemblyResult{}, err
	}
	assembly.Config = config
	return AssembleMyBatisConfig(assembly)
}

// Build 转换 settings 子配置。
func (f MyBatisSettingsFile) Build() (MyBatisSettings, error) {
	var scope LocalCacheScope
	var err error
	if strings.TrimSpace(f.LocalCacheScope) != "" {
		scope, err = ParseLocalCacheScope(f.LocalCacheScope)
		if err != nil {
			return MyBatisSettings{}, err
		}
	}
	var executorType ExecutorType
	if strings.TrimSpace(f.DefaultExecutorType) != "" {
		executorType, err = ParseExecutorType(f.DefaultExecutorType)
		if err != nil {
			return MyBatisSettings{}, err
		}
	}
	var resultSetType ResultSetType
	if strings.TrimSpace(f.DefaultResultSetType) != "" {
		resultSetType, err = ParseResultSetType(f.DefaultResultSetType)
		if err != nil {
			return MyBatisSettings{}, err
		}
	}
	var autoMapping AutoMappingBehavior
	if strings.TrimSpace(f.AutoMappingBehavior) != "" {
		autoMapping, err = ParseAutoMappingBehavior(f.AutoMappingBehavior)
		if err != nil {
			return MyBatisSettings{}, err
		}
	}
	var unknownColumn AutoMappingUnknownColumnBehavior
	if strings.TrimSpace(f.AutoMappingUnknownColumnBehavior) != "" {
		unknownColumn, err = ParseAutoMappingUnknownColumnBehavior(f.AutoMappingUnknownColumnBehavior)
		if err != nil {
			return MyBatisSettings{}, err
		}
	}
	jdbcTypeForNull, err := normalizeJDBCTypeName(f.JDBCTypeForNull)
	if err != nil {
		return MyBatisSettings{}, err
	}
	timeout, err := parseConfigDuration(f.DefaultStatementTimeout)
	if err != nil {
		return MyBatisSettings{}, err
	}
	return MyBatisSettings{
		CacheEnabled:                       cloneBoolPointer(f.CacheEnabled),
		LocalCacheEnabled:                  cloneBoolPointer(f.LocalCacheEnabled),
		UseColumnLabel:                     cloneBoolPointer(f.UseColumnLabel),
		LocalCacheScope:                    scope,
		MapUnderscoreToCamelCase:           f.MapUnderscoreToCamelCase,
		UseGeneratedKeys:                   f.UseGeneratedKeys,
		LazyLoadingEnabled:                 f.LazyLoadingEnabled,
		DefaultExecutorType:                executorType,
		PreparedStatementCacheSize:         f.PreparedStatementCacheSize,
		DefaultStatementTimeout:            timeout,
		DefaultFetchSize:                   f.DefaultFetchSize,
		DefaultResultSetType:               resultSetType,
		NullableOnForEach:                  cloneBoolPointer(f.NullableOnForEach),
		ShrinkWhitespacesInSQL:             f.ShrinkWhitespacesInSQL,
		JDBCTypeForNull:                    jdbcTypeForNull,
		AutoMappingBehavior:                autoMapping,
		AutoMappingUnknownColumnBehavior:   unknownColumn,
		DatabaseID:                         strings.TrimSpace(f.DatabaseID),
		SafeRowBoundsEnabled:               f.SafeRowBoundsEnabled,
		SafeResultHandlerEnabled:           cloneBoolPointer(f.SafeResultHandlerEnabled),
		AggressiveLazyLoading:              f.AggressiveLazyLoading,
		LazyLoadTriggerMethods:             cloneStringSlice(f.LazyLoadTriggerMethods),
		DefaultScriptingLanguage:           strings.TrimSpace(f.DefaultScriptingLanguage),
		DefaultEnumTypeHandler:             strings.TrimSpace(f.DefaultEnumTypeHandler),
		CallSettersOnNulls:                 f.CallSettersOnNulls,
		ReturnInstanceForEmptyRow:          f.ReturnInstanceForEmptyRow,
		LogPrefix:                          strings.TrimSpace(f.LogPrefix),
		LogImpl:                            strings.TrimSpace(f.LogImpl),
		ProxyFactory:                       strings.TrimSpace(f.ProxyFactory),
		VFSImpl:                            cloneStringSlice(f.VFSImpl),
		UseActualParamName:                 cloneBoolPointer(f.UseActualParamName),
		ConfigurationFactory:               strings.TrimSpace(f.ConfigurationFactory),
		DefaultSQLProviderType:             strings.TrimSpace(f.DefaultSQLProviderType),
		ArgNameBasedConstructorAutoMapping: f.ArgNameBasedConstructorAutoMapping,
	}, nil
}

// Build 转换 environment 子配置。
func (f MyBatisEnvironmentFile) Build() (MyBatisEnvironment, error) {
	var dbType DbType
	if strings.TrimSpace(f.DbType) != "" {
		parsed, err := ParseDbType(f.DbType)
		if err != nil {
			return MyBatisEnvironment{}, err
		}
		dbType = parsed
	}
	return MyBatisEnvironment{
		ID:         strings.TrimSpace(f.ID),
		DbType:     dbType,
		DatabaseID: strings.TrimSpace(f.DatabaseID),
	}, nil
}

// Build 转换 databaseIdProvider 子配置。
func (f DatabaseIDProviderFile) Build() (DatabaseIDProvider, error) {
	provider := DatabaseIDProvider{
		Type:       strings.TrimSpace(f.Type),
		Properties: copyConfigProperties(f.Properties),
		DefaultID:  strings.TrimSpace(f.DefaultID),
	}
	if err := provider.validate(); err != nil {
		return DatabaseIDProvider{}, err
	}
	return provider, nil
}

func parseConfigDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err == nil {
		if duration < 0 {
			return 0, configurationErrorf("duration must be >= 0")
		}
		return duration, nil
	}
	seconds, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		return 0, configurationErrorf("duration %q requires Go duration or integer seconds", value)
	}
	if seconds < 0 {
		return 0, configurationErrorf("duration must be >= 0")
	}
	return time.Duration(seconds) * time.Second, nil
}
