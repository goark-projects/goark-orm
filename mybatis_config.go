package orm

import (
	"strings"
	"time"
)

// MyBatisSettings 描述运行期 settings 配置子集。
type MyBatisSettings struct {
	CacheEnabled               *bool
	LocalCacheEnabled          *bool
	LocalCacheScope            LocalCacheScope
	MapUnderscoreToCamelCase   bool
	UseGeneratedKeys           bool
	LazyLoadingEnabled         bool
	DefaultExecutorType        ExecutorType
	PreparedStatementCacheSize int
	DefaultStatementTimeout    time.Duration
	DefaultFetchSize           int
	DatabaseID                 string
}

// MyBatisEnvironment 描述数据库环境配置。Dialect 显式指定时优先于 DbType。
type MyBatisEnvironment struct {
	ID         string
	DbType     DbType
	Dialect    Dialect
	DatabaseID string
}

// TypeAlias 描述 Go 类型别名。运行时不扫描包，别名由生成器或业务显式注册。
type TypeAlias struct {
	Alias    string `json:"alias"`
	TypeName string `json:"typeName"`
}

// TypeHandlerRef 描述配置中声明的 TypeHandler 名称。
type TypeHandlerRef struct {
	Name string `json:"name"`
}

// MapperRef 描述 Mapper 声明引用。Resource 供生成器使用，Namespace 供运行期校验。
type MapperRef struct {
	Resource  string `json:"resource,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// MyBatisConfig 是运行期配置声明模型。
type MyBatisConfig struct {
	Properties         ConfigProperties
	Settings           MyBatisSettings
	Environment        MyBatisEnvironment
	DatabaseIDProvider DatabaseIDProvider
	TypeAliases        []TypeAlias
	TypeHandlers       []TypeHandlerRef
	Mappers            []MapperRef
	Plugins            []PluginRef
	Global             GlobalConfig
}

// DefaultMyBatisConfig 返回可直接构建运行期配置的默认声明模型。
func DefaultMyBatisConfig() MyBatisConfig {
	return MyBatisConfig{
		Global: DefaultGlobalConfig(),
	}
}

// ParseLocalCacheScope 解析 localCacheScope 配置值。
func ParseLocalCacheScope(value string) (LocalCacheScope, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", string(LocalCacheScopeSession):
		return LocalCacheScopeSession, nil
	case string(LocalCacheScopeStatement):
		return LocalCacheScopeStatement, nil
	default:
		return "", configurationErrorf("localCacheScope %q is invalid", value)
	}
}

// ParseExecutorType 解析 defaultExecutorType 配置值。
func ParseExecutorType(value string) (ExecutorType, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", string(ExecutorTypeSimple):
		return ExecutorTypeSimple, nil
	case string(ExecutorTypeReuse):
		return ExecutorTypeReuse, nil
	case string(ExecutorTypeBatch):
		return ExecutorTypeBatch, nil
	default:
		return "", configurationErrorf("defaultExecutorType %q is invalid", value)
	}
}

// BuildConfiguration 将声明模型转换为运行期 Configuration。
func (c MyBatisConfig) BuildConfiguration() (Configuration, error) {
	if err := c.Validate(); err != nil {
		return Configuration{}, err
	}
	out := DefaultConfiguration()
	out.GlobalConfig = c.Global
	if c.Environment.Dialect != nil {
		out.Dialect = c.Environment.Dialect
	} else if c.Environment.DbType != "" {
		dialect, err := NewDialect(c.Environment.DbType)
		if err != nil {
			return Configuration{}, err
		}
		out.Dialect = dialect
	}
	databaseID, err := c.resolvedDatabaseID()
	if err != nil {
		return Configuration{}, err
	}
	out.DatabaseID = databaseID
	out.CacheEnabled = cloneBoolPointer(c.Settings.CacheEnabled)
	out.LocalCacheEnabled = cloneBoolPointer(c.Settings.LocalCacheEnabled)
	if c.Settings.LocalCacheScope != "" {
		out.LocalCacheScope = c.Settings.LocalCacheScope
	}
	out.MapUnderscoreToCamelCase = c.Settings.MapUnderscoreToCamelCase
	out.UseGeneratedKeys = c.Settings.UseGeneratedKeys
	out.LazyLoadingEnabled = c.Settings.LazyLoadingEnabled
	if c.Settings.DefaultExecutorType != "" {
		out.DefaultExecutorType = c.Settings.DefaultExecutorType
	}
	out.PreparedStatementCacheSize = c.Settings.PreparedStatementCacheSize
	out.DefaultStatementTimeout = c.Settings.DefaultStatementTimeout
	out.DefaultFetchSize = c.Settings.DefaultFetchSize
	return normalizeConfiguration(out, nil)
}

// Validate 校验声明模型中的稳定契约，不触发任何文件系统扫描。
func (c MyBatisConfig) Validate() error {
	if c.Environment.DbType != "" {
		if _, err := NewDialect(c.Environment.DbType); err != nil {
			return err
		}
	}
	if c.Settings.LocalCacheScope != "" {
		if _, err := ParseLocalCacheScope(string(c.Settings.LocalCacheScope)); err != nil {
			return err
		}
	}
	if c.Settings.DefaultExecutorType != "" {
		if _, err := ParseExecutorType(string(c.Settings.DefaultExecutorType)); err != nil {
			return err
		}
	}
	if c.Settings.DefaultFetchSize < 0 {
		return configurationErrorf("defaultFetchSize must be >= 0")
	}
	if c.Settings.PreparedStatementCacheSize < 0 {
		return configurationErrorf("preparedStatementCacheSize must be >= 0")
	}
	if c.Settings.DefaultStatementTimeout < 0 {
		return configurationErrorf("defaultStatementTimeout must be >= 0")
	}
	if _, err := c.TypeAliasMap(); err != nil {
		return err
	}
	if _, err := c.TypeHandlerNames(); err != nil {
		return err
	}
	if err := c.DatabaseIDProvider.validate(); err != nil {
		return err
	}
	if err := validatePluginRefs(c.Plugins); err != nil {
		return err
	}
	return validateMapperRefs(c.Mappers)
}

// TypeAliasMap 返回规范化后的别名映射，并拒绝重复别名。
func (c MyBatisConfig) TypeAliasMap() (map[string]string, error) {
	out := make(map[string]string, len(c.TypeAliases))
	for _, item := range c.TypeAliases {
		alias := strings.TrimSpace(item.Alias)
		typeName := strings.TrimSpace(item.TypeName)
		if alias == "" {
			return nil, configurationErrorf("typeAlias alias is required")
		}
		if typeName == "" {
			return nil, configurationErrorf("typeAlias %q typeName is required", alias)
		}
		key := strings.ToLower(alias)
		if _, exists := out[key]; exists {
			return nil, configurationErrorf("duplicate typeAlias %q", alias)
		}
		out[key] = typeName
	}
	return out, nil
}

// TypeHandlerNames 返回规范化后的 TypeHandler 名称，并拒绝重复声明。
func (c MyBatisConfig) TypeHandlerNames() ([]string, error) {
	out := make([]string, 0, len(c.TypeHandlers))
	seen := make(map[string]struct{}, len(c.TypeHandlers))
	for _, item := range c.TypeHandlers {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, configurationErrorf("typeHandler name is required")
		}
		if _, exists := seen[name]; exists {
			return nil, configurationErrorf("duplicate typeHandler %q", name)
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

func validateMapperRefs(mappers []MapperRef) error {
	namespaces := make(map[string]struct{}, len(mappers))
	for _, mapper := range mappers {
		resource := strings.TrimSpace(mapper.Resource)
		namespace := strings.TrimSpace(mapper.Namespace)
		if resource == "" && namespace == "" {
			return configurationErrorf("mapper resource or namespace is required")
		}
		if namespace == "" {
			continue
		}
		if _, exists := namespaces[namespace]; exists {
			return configurationErrorf("duplicate mapper namespace %q", namespace)
		}
		namespaces[namespace] = struct{}{}
	}
	return nil
}

func (c MyBatisConfig) resolvedDatabaseID() (string, error) {
	providerID, err := c.DatabaseIDProvider.resolve(c.Environment)
	if err != nil {
		return "", err
	}
	return firstNonBlank(c.Settings.DatabaseID, c.Environment.DatabaseID, providerID), nil
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
