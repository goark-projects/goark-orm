package orm

import (
	"fmt"
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
	// ExecutorTypeSimple 表示普通即时执行器。
	ExecutorTypeSimple ExecutorType = "SIMPLE"
	// ExecutorTypeReuse 表示可复用预编译语句执行器。
	ExecutorTypeReuse ExecutorType = "REUSE"
	// ExecutorTypeBatch 表示批量执行器。
	ExecutorTypeBatch ExecutorType = "BATCH"
)

// Configuration 描述 MyBatis 风格的 ORM 运行期配置。
type Configuration struct {
	Dialect                  Dialect
	DatabaseID               string
	GlobalConfig             GlobalConfig
	LocalCacheEnabled        *bool
	LocalCacheScope          LocalCacheScope
	CacheEnabled             *bool
	MapUnderscoreToCamelCase bool
	UseGeneratedKeys         bool
	LazyLoadingEnabled       bool
	DefaultExecutorType      ExecutorType
	DefaultStatementTimeout  time.Duration
	DefaultFetchSize         int
	MetaObjectHandler        MetaObjectHandler
}

// DefaultConfiguration 返回独立 ORM 的默认运行期配置。
func DefaultConfiguration() Configuration {
	localCacheEnabled := true
	cacheEnabled := true
	return Configuration{
		LocalCacheEnabled:   &localCacheEnabled,
		LocalCacheScope:     LocalCacheScopeSession,
		CacheEnabled:        &cacheEnabled,
		DefaultExecutorType: ExecutorTypeSimple,
		GlobalConfig:        DefaultGlobalConfig(),
	}
}

// WithGlobalConfig 返回设置 MyBatis-Plus 风格全局配置后的配置副本。
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

// WithConfiguration 应用独立 ORM 配置。
func WithConfiguration(config Configuration) SQLSessionOption {
	return func(session *SQLSession) error {
		if session == nil {
			return fmt.Errorf("goark-orm: session is nil")
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
	switch out.LocalCacheScope {
	case "":
		out.LocalCacheScope = defaults.LocalCacheScope
	case LocalCacheScopeSession, LocalCacheScopeStatement:
	default:
		return Configuration{}, fmt.Errorf("goark-orm: localCacheScope %q is invalid", out.LocalCacheScope)
	}
	switch out.DefaultExecutorType {
	case "":
		out.DefaultExecutorType = defaults.DefaultExecutorType
	case ExecutorTypeSimple, ExecutorTypeReuse, ExecutorTypeBatch:
	default:
		return Configuration{}, fmt.Errorf("goark-orm: defaultExecutorType %q is invalid", out.DefaultExecutorType)
	}
	if out.DefaultFetchSize < 0 {
		return Configuration{}, fmt.Errorf("goark-orm: defaultFetchSize must be >= 0")
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
	return config
}
