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
	Settings     MyBatisSettingsFile    `json:"settings,omitempty"`
	Environment  MyBatisEnvironmentFile `json:"environment,omitempty"`
	TypeAliases  []TypeAlias            `json:"typeAliases,omitempty"`
	TypeHandlers []TypeHandlerRef       `json:"typeHandlers,omitempty"`
	Mappers      []MapperRef            `json:"mappers,omitempty"`
}

// MyBatisSettingsFile 使用字符串承载需要解析的枚举和时间配置。
type MyBatisSettingsFile struct {
	CacheEnabled             *bool  `json:"cacheEnabled,omitempty"`
	LocalCacheEnabled        *bool  `json:"localCacheEnabled,omitempty"`
	LocalCacheScope          string `json:"localCacheScope,omitempty"`
	MapUnderscoreToCamelCase bool   `json:"mapUnderscoreToCamelCase,omitempty"`
	UseGeneratedKeys         bool   `json:"useGeneratedKeys,omitempty"`
	LazyLoadingEnabled       bool   `json:"lazyLoadingEnabled,omitempty"`
	DefaultExecutorType      string `json:"defaultExecutorType,omitempty"`
	DefaultStatementTimeout  string `json:"defaultStatementTimeout,omitempty"`
	DefaultFetchSize         int    `json:"defaultFetchSize,omitempty"`
	DatabaseID               string `json:"databaseId,omitempty"`
}

// MyBatisEnvironmentFile 描述 JSON 中的数据库环境。
type MyBatisEnvironmentFile struct {
	ID         string `json:"id,omitempty"`
	DbType     string `json:"dbType,omitempty"`
	DatabaseID string `json:"databaseId,omitempty"`
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
	settings, err := f.Settings.Build()
	if err != nil {
		return MyBatisConfig{}, err
	}
	environment, err := f.Environment.Build()
	if err != nil {
		return MyBatisConfig{}, err
	}
	config := MyBatisConfig{
		Settings:     settings,
		Environment:  environment,
		TypeAliases:  append([]TypeAlias(nil), f.TypeAliases...),
		TypeHandlers: append([]TypeHandlerRef(nil), f.TypeHandlers...),
		Mappers:      append([]MapperRef(nil), f.Mappers...),
		Global:       DefaultGlobalConfig(),
	}
	if err := config.Validate(); err != nil {
		return MyBatisConfig{}, err
	}
	return config, nil
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
	timeout, err := parseConfigDuration(f.DefaultStatementTimeout)
	if err != nil {
		return MyBatisSettings{}, err
	}
	return MyBatisSettings{
		CacheEnabled:             cloneBoolPointer(f.CacheEnabled),
		LocalCacheEnabled:        cloneBoolPointer(f.LocalCacheEnabled),
		LocalCacheScope:          scope,
		MapUnderscoreToCamelCase: f.MapUnderscoreToCamelCase,
		UseGeneratedKeys:         f.UseGeneratedKeys,
		LazyLoadingEnabled:       f.LazyLoadingEnabled,
		DefaultExecutorType:      executorType,
		DefaultStatementTimeout:  timeout,
		DefaultFetchSize:         f.DefaultFetchSize,
		DatabaseID:               strings.TrimSpace(f.DatabaseID),
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
