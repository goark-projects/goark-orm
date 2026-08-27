package orm

import (
	"database/sql"
	"strings"
)

// MyBatisAssembly 描述一次显式运行期装配输入。
type MyBatisAssembly struct {
	Config         MyBatisConfig
	Registry       *Registry
	DB             *sql.DB
	TypeHandlers   map[string]TypeHandler
	SessionOptions []SQLSessionOption
}

// MyBatisAssemblyResult 返回配置装配后的稳定运行期对象。
type MyBatisAssemblyResult struct {
	Configuration  Configuration
	Registry       *Registry
	SessionFactory *SQLSessionFactory
	TypeAliases    map[string]string
	TypeHandlers   []string
	Mappers        []MapperRef
}

// AssembleMyBatisConfig 显式装配配置、注册表和可选 SessionFactory。
func AssembleMyBatisConfig(assembly MyBatisAssembly) (MyBatisAssemblyResult, error) {
	if assembly.Registry == nil {
		return MyBatisAssemblyResult{}, configurationErrorf("registry is nil")
	}
	if err := assembly.Config.Validate(); err != nil {
		return MyBatisAssemblyResult{}, err
	}
	if len(assembly.TypeHandlers) > 0 {
		if err := assembly.Registry.RegisterTypeHandlers(assembly.TypeHandlers); err != nil {
			return MyBatisAssemblyResult{}, err
		}
	}
	typeHandlerNames, err := assembly.Config.TypeHandlerNames()
	if err != nil {
		return MyBatisAssemblyResult{}, err
	}
	if err := validateConfiguredTypeHandlers(assembly.Registry, typeHandlerNames); err != nil {
		return MyBatisAssemblyResult{}, err
	}
	if err := validateConfiguredMappers(assembly.Registry, assembly.Config.Mappers); err != nil {
		return MyBatisAssemblyResult{}, err
	}
	if err := assembly.Registry.Validate(); err != nil {
		return MyBatisAssemblyResult{}, err
	}
	configuration, err := assembly.Config.BuildConfiguration()
	if err != nil {
		return MyBatisAssemblyResult{}, err
	}
	aliases, err := assembly.Config.TypeAliasMap()
	if err != nil {
		return MyBatisAssemblyResult{}, err
	}
	options := append([]SQLSessionOption(nil), assembly.SessionOptions...)
	options = append(options, WithConfiguration(configuration))
	var factory *SQLSessionFactory
	if assembly.DB != nil {
		factory, err = NewSQLSessionFactory(assembly.Registry, assembly.DB, configuration.Dialect, options...)
		if err != nil {
			return MyBatisAssemblyResult{}, err
		}
	}
	return MyBatisAssemblyResult{
		Configuration:  configuration,
		Registry:       assembly.Registry,
		SessionFactory: factory,
		TypeAliases:    aliases,
		TypeHandlers:   append([]string(nil), typeHandlerNames...),
		Mappers:        copyMapperRefs(assembly.Config.Mappers),
	}, nil
}

func validateConfiguredTypeHandlers(registry *Registry, names []string) error {
	for _, name := range names {
		if _, ok := registry.TypeHandler(name); !ok {
			return configurationErrorf("typeHandler %q is not registered", name)
		}
	}
	return nil
}

func validateConfiguredMappers(registry *Registry, mappers []MapperRef) error {
	for _, mapper := range mappers {
		namespace := strings.TrimSpace(mapper.Namespace)
		if namespace == "" {
			continue
		}
		if _, ok := registry.Mapper(namespace); !ok {
			return configurationErrorf("mapper namespace %q is not registered", namespace)
		}
	}
	return nil
}

func copyMapperRefs(mappers []MapperRef) []MapperRef {
	if len(mappers) == 0 {
		return nil
	}
	out := make([]MapperRef, len(mappers))
	copy(out, mappers)
	return out
}
