package ormboot

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	orm "goark.dev/orm"
)

const (
	// DefaultName 是默认 ORM 装配单元名称。
	DefaultName = "goarkORM"
	// BeanNameRuntime 是默认运行时 Bean 名称。
	BeanNameRuntime = "goarkORMRuntime"
	// BeanNameRegistry 是默认元数据注册表 Bean 名称。
	BeanNameRegistry = "goarkORMRegistry"
	// BeanNameConfiguration 是默认运行期配置 Bean 名称。
	BeanNameConfiguration = "goarkORMConfiguration"
	// BeanNameSessionFactory 是默认 SessionFactory Bean 名称。
	BeanNameSessionFactory = "goarkORMSessionFactory"
)

// MetadataRegistrar 注册生成期产生的 ORM 元数据。
type MetadataRegistrar func(*orm.Registry) error

// BeanNames 描述适配器输出给上层容器的 Bean 名称。
type BeanNames struct {
	Runtime        string
	Registry       string
	Configuration  string
	SessionFactory string
}

// Config 描述一次 ORM boot 风格装配。
type Config struct {
	Name               string
	Order              int
	BeanNames          BeanNames
	Registry           *orm.Registry
	DB                 *sql.DB
	RuntimeConfig      orm.RuntimeConfig
	MyBatisConfig      orm.MyBatisConfig
	TypeHandlers       map[string]orm.TypeHandler
	Plugins            orm.PluginRegistry
	SessionOptions     []orm.SQLSessionOption
	MetadataRegistrars []MetadataRegistrar
}

func normalizeConfig(config Config) (Config, error) {
	config.Name = strings.TrimSpace(config.Name)
	if config.Name == "" {
		config.Name = DefaultName
	}
	if config.DB == nil {
		return Config{}, fmt.Errorf("goark-orm/ormboot: database is nil")
	}
	if config.Registry == nil {
		config.Registry = orm.NewRegistry()
	}
	if !runtimeConfigConfigured(config.RuntimeConfig) {
		config.RuntimeConfig = config.MyBatisConfig
	}
	beanNames, err := normalizeBeanNames(config.BeanNames)
	if err != nil {
		return Config{}, err
	}
	config.BeanNames = beanNames
	config.TypeHandlers = cloneTypeHandlers(config.TypeHandlers)
	config.SessionOptions = append([]orm.SQLSessionOption(nil), config.SessionOptions...)
	config.MetadataRegistrars = append([]MetadataRegistrar(nil), config.MetadataRegistrars...)
	return config, nil
}

func runtimeConfigConfigured(config orm.RuntimeConfig) bool {
	if len(config.Properties) > 0 ||
		len(config.TypeAliases) > 0 ||
		len(config.TypeHandlers) > 0 ||
		len(config.Mappers) > 0 ||
		len(config.Plugins) > 0 {
		return true
	}
	return !reflect.DeepEqual(config.Settings, orm.RuntimeSettings{}) ||
		!reflect.DeepEqual(config.Environment, orm.RuntimeEnvironment{}) ||
		config.DatabaseIDProvider.Type != "" ||
		config.DatabaseIDProvider.DefaultID != "" ||
		len(config.DatabaseIDProvider.Properties) > 0 ||
		!reflect.DeepEqual(config.Global, orm.GlobalConfig{})
}

func normalizeBeanNames(names BeanNames) (BeanNames, error) {
	out := BeanNames{
		Runtime:        firstName(names.Runtime, BeanNameRuntime),
		Registry:       firstName(names.Registry, BeanNameRegistry),
		Configuration:  firstName(names.Configuration, BeanNameConfiguration),
		SessionFactory: firstName(names.SessionFactory, BeanNameSessionFactory),
	}
	seen := make(map[string]struct{}, 4)
	for _, name := range []string{out.Runtime, out.Registry, out.Configuration, out.SessionFactory} {
		if _, exists := seen[name]; exists {
			return BeanNames{}, fmt.Errorf("goark-orm/ormboot: duplicate bean name %q", name)
		}
		seen[name] = struct{}{}
	}
	return out, nil
}

func firstName(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func cloneTypeHandlers(handlers map[string]orm.TypeHandler) map[string]orm.TypeHandler {
	if len(handlers) == 0 {
		return nil
	}
	out := make(map[string]orm.TypeHandler, len(handlers))
	for name, handler := range handlers {
		out[name] = handler
	}
	return out
}
