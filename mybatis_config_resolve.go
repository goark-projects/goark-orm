package orm

func (f MyBatisConfigFile) resolveProperties(resolver *configPropertyResolver) (MyBatisConfigFile, error) {
	var err error
	out := f
	out.Properties = resolver.Properties()
	if out.Settings, err = f.Settings.resolveProperties(resolver); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.Environment, err = f.Environment.resolveProperties(resolver); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.DatabaseIDProvider, err = f.DatabaseIDProvider.resolveProperties(resolver); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.TypeAliases, err = resolveTypeAliases(resolver, f.TypeAliases); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.TypeHandlers, err = resolveTypeHandlers(resolver, f.TypeHandlers); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.Mappers, err = resolveMapperRefs(resolver, f.Mappers); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.Plugins, err = resolvePluginRefs(resolver, f.Plugins); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.Global, err = resolveGlobalFile(resolver, f.Global); err != nil {
		return MyBatisConfigFile{}, err
	}
	if out.GlobalConfig, err = resolveGlobalFile(resolver, f.GlobalConfig); err != nil {
		return MyBatisConfigFile{}, err
	}
	return out, nil
}

func (f MyBatisSettingsFile) resolveProperties(resolver *configPropertyResolver) (MyBatisSettingsFile, error) {
	var err error
	out := f
	if out.LocalCacheScope, err = resolveConfigString(resolver, f.LocalCacheScope); err != nil {
		return MyBatisSettingsFile{}, err
	}
	if out.DefaultExecutorType, err = resolveConfigString(resolver, f.DefaultExecutorType); err != nil {
		return MyBatisSettingsFile{}, err
	}
	if out.DefaultStatementTimeout, err = resolveConfigString(resolver, f.DefaultStatementTimeout); err != nil {
		return MyBatisSettingsFile{}, err
	}
	if out.DefaultResultSetType, err = resolveConfigString(resolver, f.DefaultResultSetType); err != nil {
		return MyBatisSettingsFile{}, err
	}
	if out.JDBCTypeForNull, err = resolveConfigString(resolver, f.JDBCTypeForNull); err != nil {
		return MyBatisSettingsFile{}, err
	}
	if out.AutoMappingBehavior, err = resolveConfigString(resolver, f.AutoMappingBehavior); err != nil {
		return MyBatisSettingsFile{}, err
	}
	if out.AutoMappingUnknownColumnBehavior, err = resolveConfigString(resolver, f.AutoMappingUnknownColumnBehavior); err != nil {
		return MyBatisSettingsFile{}, err
	}
	if out.DatabaseID, err = resolveConfigString(resolver, f.DatabaseID); err != nil {
		return MyBatisSettingsFile{}, err
	}
	return out, nil
}

func (f MyBatisEnvironmentFile) resolveProperties(resolver *configPropertyResolver) (MyBatisEnvironmentFile, error) {
	var err error
	out := f
	if out.ID, err = resolveConfigString(resolver, f.ID); err != nil {
		return MyBatisEnvironmentFile{}, err
	}
	if out.DbType, err = resolveConfigString(resolver, f.DbType); err != nil {
		return MyBatisEnvironmentFile{}, err
	}
	if out.DatabaseID, err = resolveConfigString(resolver, f.DatabaseID); err != nil {
		return MyBatisEnvironmentFile{}, err
	}
	return out, nil
}

func (f DatabaseIDProviderFile) resolveProperties(resolver *configPropertyResolver) (DatabaseIDProviderFile, error) {
	var err error
	out := f
	if out.Type, err = resolveConfigString(resolver, f.Type); err != nil {
		return DatabaseIDProviderFile{}, err
	}
	if out.DefaultID, err = resolveConfigString(resolver, f.DefaultID); err != nil {
		return DatabaseIDProviderFile{}, err
	}
	out.Properties, err = resolveConfigProperties(resolver, f.Properties)
	if err != nil {
		return DatabaseIDProviderFile{}, err
	}
	return out, nil
}

func resolveTypeAliases(resolver *configPropertyResolver, items []TypeAlias) ([]TypeAlias, error) {
	out := make([]TypeAlias, 0, len(items))
	for _, item := range items {
		alias, err := resolveConfigString(resolver, item.Alias)
		if err != nil {
			return nil, err
		}
		typeName, err := resolveConfigString(resolver, item.TypeName)
		if err != nil {
			return nil, err
		}
		out = append(out, TypeAlias{Alias: alias, TypeName: typeName})
	}
	return out, nil
}

func resolveTypeHandlers(resolver *configPropertyResolver, items []TypeHandlerRef) ([]TypeHandlerRef, error) {
	out := make([]TypeHandlerRef, 0, len(items))
	for _, item := range items {
		name, err := resolveConfigString(resolver, item.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, TypeHandlerRef{Name: name})
	}
	return out, nil
}

func resolveMapperRefs(resolver *configPropertyResolver, items []MapperRef) ([]MapperRef, error) {
	out := make([]MapperRef, 0, len(items))
	for _, item := range items {
		resource, err := resolveConfigString(resolver, item.Resource)
		if err != nil {
			return nil, err
		}
		namespace, err := resolveConfigString(resolver, item.Namespace)
		if err != nil {
			return nil, err
		}
		out = append(out, MapperRef{Resource: resource, Namespace: namespace})
	}
	return out, nil
}

func resolvePluginRefs(resolver *configPropertyResolver, items []PluginRef) ([]PluginRef, error) {
	out := make([]PluginRef, 0, len(items))
	for _, item := range items {
		name, err := resolveConfigString(resolver, item.Name)
		if err != nil {
			return nil, err
		}
		options, err := resolveStringMap(resolver, item.Options)
		if err != nil {
			return nil, err
		}
		out = append(out, PluginRef{
			Name:    name,
			Enabled: cloneBoolPointer(item.Enabled),
			Options: options,
		})
	}
	return out, nil
}

func resolveGlobalFile(resolver *configPropertyResolver, item *MyBatisGlobalConfigFile) (*MyBatisGlobalConfigFile, error) {
	if item == nil {
		return nil, nil
	}
	dbConfig, err := item.DbConfig.resolveProperties(resolver)
	if err != nil {
		return nil, err
	}
	return &MyBatisGlobalConfigFile{DbConfig: dbConfig}, nil
}

func (f MyBatisDbConfigFile) resolveProperties(resolver *configPropertyResolver) (MyBatisDbConfigFile, error) {
	var err error
	out := f
	if out.IDType, err = resolveConfigString(resolver, f.IDType); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.TablePrefix, err = resolveConfigString(resolver, f.TablePrefix); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.Schema, err = resolveConfigString(resolver, f.Schema); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.LogicDeleteField, err = resolveConfigString(resolver, f.LogicDeleteField); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.LogicDeleteValue, err = resolveConfigValue(resolver, f.LogicDeleteValue); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.LogicNotDeleteValue, err = resolveConfigValue(resolver, f.LogicNotDeleteValue); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.InsertStrategy, err = resolveConfigString(resolver, f.InsertStrategy); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.UpdateStrategy, err = resolveConfigString(resolver, f.UpdateStrategy); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	if out.WhereStrategy, err = resolveConfigString(resolver, f.WhereStrategy); err != nil {
		return MyBatisDbConfigFile{}, err
	}
	return out, nil
}

func resolveConfigValue(resolver *configPropertyResolver, value any) (any, error) {
	text, ok := value.(string)
	if !ok {
		return value, nil
	}
	return resolveConfigString(resolver, text)
}

func resolveConfigProperties(resolver *configPropertyResolver, properties ConfigProperties) (ConfigProperties, error) {
	resolved, err := resolveStringMap(resolver, properties)
	if err != nil {
		return nil, err
	}
	return ConfigProperties(resolved), nil
}

func resolveStringMap(resolver *configPropertyResolver, values map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		resolvedKey, err := resolveConfigString(resolver, key)
		if err != nil {
			return nil, err
		}
		resolvedValue, err := resolveConfigString(resolver, value)
		if err != nil {
			return nil, err
		}
		out[resolvedKey] = resolvedValue
	}
	return out, nil
}

func copyConfigProperties(properties ConfigProperties) ConfigProperties {
	if len(properties) == 0 {
		return nil
	}
	out := make(ConfigProperties, len(properties))
	for key, value := range properties {
		out[key] = value
	}
	return out
}
