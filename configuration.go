package orm

import (
	"strings"
	"time"
)

// LocalCacheScope 描述一级缓存作用域。
type LocalCacheScope string

const (
	// LocalCacheScopeSession 表示一级缓存跨同一 Session 的多次语句复用。
	LocalCacheScopeSession LocalCacheScope = "SESSION"
	// LocalCacheScopeStatement 表示一级缓存只服务单次语句执行。
	LocalCacheScopeStatement LocalCacheScope = "STATEMENT"
)

// ExecutorType 描述默认执行器类型。
type ExecutorType string

const (
	// DefaultPreparedStatementCacheSize 是 REUSE 执行器的默认预编译语句缓存容量。
	DefaultPreparedStatementCacheSize = 256
)

const (
	// ExecutorTypeSimple 表示普通即时执行器。
	ExecutorTypeSimple ExecutorType = "SIMPLE"
	// ExecutorTypeReuse 表示可复用预编译语句执行器。
	ExecutorTypeReuse ExecutorType = "REUSE"
	// ExecutorTypeBatch 表示批量执行器。
	ExecutorTypeBatch ExecutorType = "BATCH"
)

// Configuration 描述 ORM 运行期配置。
type Configuration struct {
	Dialect                          Dialect
	DatabaseID                       string
	GlobalConfig                     GlobalConfig
	LocalCacheEnabled                *bool
	LocalCacheScope                  LocalCacheScope
	CacheEnabled                     *bool
	MapUnderscoreToCamelCase         bool
	UseGeneratedKeys                 bool
	LazyLoadingEnabled               bool
	DefaultExecutorType              ExecutorType
	PreparedStatementCacheSize       int
	DefaultStatementTimeout          time.Duration
	DefaultFetchSize                 int
	DefaultResultSetType             ResultSetType
	UseColumnLabel                   *bool
	NullableOnForEach                *bool
	ShrinkWhitespacesInSQL           bool
	JDBCTypeForNull                  string
	AutoMappingBehavior              AutoMappingBehavior
	AutoMappingUnknownColumnBehavior AutoMappingUnknownColumnBehavior
	MetaObjectHandler                MetaObjectHandler
}

// DefaultConfiguration 返回独立 ORM 的默认运行期配置。
func DefaultConfiguration() Configuration {
	localCacheEnabled := true
	cacheEnabled := true
	useColumnLabel := true
	nullableOnForEach := true
	return Configuration{
		LocalCacheEnabled:                &localCacheEnabled,
		LocalCacheScope:                  LocalCacheScopeSession,
		CacheEnabled:                     &cacheEnabled,
		DefaultExecutorType:              ExecutorTypeSimple,
		PreparedStatementCacheSize:       DefaultPreparedStatementCacheSize,
		UseColumnLabel:                   &useColumnLabel,
		NullableOnForEach:                &nullableOnForEach,
		JDBCTypeForNull:                  "OTHER",
		AutoMappingBehavior:              AutoMappingBehaviorFull,
		AutoMappingUnknownColumnBehavior: AutoMappingUnknownColumnBehaviorNone,
		GlobalConfig:                     DefaultGlobalConfig(),
	}
}

// WithGlobalConfig 返回设置全局配置后的配置副本。
func (c Configuration) WithGlobalConfig(global GlobalConfig) Configuration {
	c.GlobalConfig = global
	return c
}

// WithLocalCache 返回显式设置一级缓存开关后的配置副本。
func (c Configuration) WithLocalCache(enabled bool) Configuration {
	c.LocalCacheEnabled = boolPointer(enabled)
	return c
}

// WithSecondLevelCache 返回显式设置二级缓存总开关后的配置副本。
func (c Configuration) WithSecondLevelCache(enabled bool) Configuration {
	c.CacheEnabled = boolPointer(enabled)
	return c
}

// WithMapUnderscoreToCamelCase 返回设置下划线转驼峰自动映射后的配置副本。
func (c Configuration) WithMapUnderscoreToCamelCase(enabled bool) Configuration {
	c.MapUnderscoreToCamelCase = enabled
	return c
}

// WithDefaultResultSetType 返回设置默认结果集类型后的配置副本。
func (c Configuration) WithDefaultResultSetType(value ResultSetType) Configuration {
	c.DefaultResultSetType = value
	return c
}

// WithNullableOnForEach 返回设置 foreach 空集合策略后的配置副本。
func (c Configuration) WithNullableOnForEach(enabled bool) Configuration {
	c.NullableOnForEach = boolPointer(enabled)
	return c
}

// WithAutoMappingBehavior 返回设置自动映射策略后的配置副本。
func (c Configuration) WithAutoMappingBehavior(value AutoMappingBehavior) Configuration {
	c.AutoMappingBehavior = value
	return c
}

// WithAutoMappingUnknownColumnBehavior 返回设置未知自动映射列策略后的配置副本。
func (c Configuration) WithAutoMappingUnknownColumnBehavior(value AutoMappingUnknownColumnBehavior) Configuration {
	c.AutoMappingUnknownColumnBehavior = value
	return c
}

// WithConfiguration 应用独立 ORM 配置。
func WithConfiguration(config Configuration) SQLSessionOption {
	return func(session *SQLSession) error {
		if session == nil {
			return configurationErrorf("session is nil")
		}
		normalized, err := normalizeConfiguration(config, session.dialect)
		if err != nil {
			return err
		}
		session.configuration = normalized
		session.dialect = normalized.Dialect
		session.localCacheScope = normalized.LocalCacheScope
		session.cacheEnabled = boolValue(normalized.CacheEnabled, true)
		session.mapUnderscoreToCamelCase = normalized.MapUnderscoreToCamelCase
		session.metaObjectHandler = normalized.MetaObjectHandler
		if normalized.GlobalConfig.IdentifierGenerator != nil {
			session.identifierGenerator = normalized.GlobalConfig.IdentifierGenerator
		}
		if boolValue(normalized.LocalCacheEnabled, true) {
			if session.localCache == nil {
				session.localCache = newLocalCache()
			}
		} else {
			session.localCache = nil
		}
		return nil
	}
}

func normalizeConfiguration(config Configuration, fallbackDialect Dialect) (Configuration, error) {
	defaults := DefaultConfiguration()
	out := config
	if out.Dialect == nil {
		out.Dialect = fallbackDialect
	}
	if out.Dialect == nil {
		out.Dialect = NewQuestionDialect()
	}
	out.DatabaseID = strings.TrimSpace(out.DatabaseID)
	global, err := normalizeGlobalConfig(out.GlobalConfig)
	if err != nil {
		return Configuration{}, err
	}
	out.GlobalConfig = global
	if out.MetaObjectHandler == nil {
		out.MetaObjectHandler = out.GlobalConfig.MetaObjectHandler
	} else if out.GlobalConfig.MetaObjectHandler == nil {
		out.GlobalConfig.MetaObjectHandler = out.MetaObjectHandler
	}
	if out.LocalCacheEnabled == nil {
		out.LocalCacheEnabled = boolPointer(boolValue(defaults.LocalCacheEnabled, true))
	} else {
		out.LocalCacheEnabled = boolPointer(*out.LocalCacheEnabled)
	}
	if out.CacheEnabled == nil {
		out.CacheEnabled = boolPointer(boolValue(defaults.CacheEnabled, true))
	} else {
		out.CacheEnabled = boolPointer(*out.CacheEnabled)
	}
	if out.UseColumnLabel == nil {
		out.UseColumnLabel = boolPointer(boolValue(defaults.UseColumnLabel, true))
	} else {
		out.UseColumnLabel = boolPointer(*out.UseColumnLabel)
	}
	if out.NullableOnForEach == nil {
		out.NullableOnForEach = boolPointer(boolValue(defaults.NullableOnForEach, true))
	} else {
		out.NullableOnForEach = boolPointer(*out.NullableOnForEach)
	}
	switch out.LocalCacheScope {
	case "":
		out.LocalCacheScope = defaults.LocalCacheScope
	case LocalCacheScopeSession, LocalCacheScopeStatement:
	default:
		return Configuration{}, configurationErrorf("localCacheScope %q is invalid", out.LocalCacheScope)
	}
	switch out.DefaultExecutorType {
	case "":
		out.DefaultExecutorType = defaults.DefaultExecutorType
	case ExecutorTypeSimple, ExecutorTypeReuse, ExecutorTypeBatch:
	default:
		return Configuration{}, configurationErrorf("defaultExecutorType %q is invalid", out.DefaultExecutorType)
	}
	if out.DefaultFetchSize < 0 {
		return Configuration{}, configurationErrorf("defaultFetchSize must be >= 0")
	}
	resultSetType, err := ParseResultSetType(string(out.DefaultResultSetType))
	if err != nil {
		return Configuration{}, err
	}
	out.DefaultResultSetType = resultSetType
	jdbcType, err := normalizeJDBCTypeName(out.JDBCTypeForNull)
	if err != nil {
		return Configuration{}, err
	}
	if jdbcType == "" {
		jdbcType = defaults.JDBCTypeForNull
	}
	out.JDBCTypeForNull = jdbcType
	autoMapping, err := ParseAutoMappingBehavior(string(out.AutoMappingBehavior))
	if err != nil {
		return Configuration{}, err
	}
	switch autoMapping {
	case "":
		out.AutoMappingBehavior = defaults.AutoMappingBehavior
	default:
		out.AutoMappingBehavior = autoMapping
	}
	unknownColumn, err := ParseAutoMappingUnknownColumnBehavior(string(out.AutoMappingUnknownColumnBehavior))
	if err != nil {
		return Configuration{}, err
	}
	switch unknownColumn {
	case "":
		out.AutoMappingUnknownColumnBehavior = defaults.AutoMappingUnknownColumnBehavior
	default:
		out.AutoMappingUnknownColumnBehavior = unknownColumn
	}
	switch {
	case out.PreparedStatementCacheSize == 0:
		out.PreparedStatementCacheSize = defaults.PreparedStatementCacheSize
	case out.PreparedStatementCacheSize < 0:
		return Configuration{}, configurationErrorf("preparedStatementCacheSize must be >= 0")
	}
	return out, nil
}

func boolPointer(value bool) *bool {
	return &value
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func cloneConfiguration(config Configuration) Configuration {
	if config.LocalCacheEnabled != nil {
		config.LocalCacheEnabled = boolPointer(*config.LocalCacheEnabled)
	}
	if config.CacheEnabled != nil {
		config.CacheEnabled = boolPointer(*config.CacheEnabled)
	}
	if config.UseColumnLabel != nil {
		config.UseColumnLabel = boolPointer(*config.UseColumnLabel)
	}
	if config.NullableOnForEach != nil {
		config.NullableOnForEach = boolPointer(*config.NullableOnForEach)
	}
	return config
}
