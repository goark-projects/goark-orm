package runtime

import "strings"

type compatibilitySettings struct {
	LazyLoadTriggerMethods             []string
	DefaultScriptingLanguage           string
	DefaultEnumTypeHandler             string
	LogPrefix                          string
	LogImpl                            string
	ProxyFactory                       string
	VFSImpl                            []string
	ConfigurationFactory               string
	DefaultSQLProviderType             string
	ArgNameBasedConstructorAutoMapping bool
}

func (s MyBatisSettings) compatibilitySettings() compatibilitySettings {
	return compatibilitySettings{
		LazyLoadTriggerMethods:             s.LazyLoadTriggerMethods,
		DefaultScriptingLanguage:           s.DefaultScriptingLanguage,
		DefaultEnumTypeHandler:             s.DefaultEnumTypeHandler,
		LogPrefix:                          s.LogPrefix,
		LogImpl:                            s.LogImpl,
		ProxyFactory:                       s.ProxyFactory,
		VFSImpl:                            s.VFSImpl,
		ConfigurationFactory:               s.ConfigurationFactory,
		DefaultSQLProviderType:             s.DefaultSQLProviderType,
		ArgNameBasedConstructorAutoMapping: s.ArgNameBasedConstructorAutoMapping,
	}
}

func (c Configuration) compatibilitySettings() compatibilitySettings {
	return compatibilitySettings{
		LazyLoadTriggerMethods:             c.LazyLoadTriggerMethods,
		DefaultScriptingLanguage:           c.DefaultScriptingLanguage,
		DefaultEnumTypeHandler:             c.DefaultEnumTypeHandler,
		LogPrefix:                          c.LogPrefix,
		LogImpl:                            c.LogImpl,
		ProxyFactory:                       c.ProxyFactory,
		VFSImpl:                            c.VFSImpl,
		ConfigurationFactory:               c.ConfigurationFactory,
		DefaultSQLProviderType:             c.DefaultSQLProviderType,
		ArgNameBasedConstructorAutoMapping: c.ArgNameBasedConstructorAutoMapping,
	}
}

func validateCompatibilitySettings(settings compatibilitySettings) error {
	if _, err := normalizeSettingList("lazyLoadTriggerMethods", settings.LazyLoadTriggerMethods, false); err != nil {
		return err
	}
	if err := validateSettingToken("defaultScriptingLanguage", settings.DefaultScriptingLanguage, true); err != nil {
		return err
	}
	if err := validateSettingToken("defaultEnumTypeHandler", settings.DefaultEnumTypeHandler, true); err != nil {
		return err
	}
	if err := validateSettingToken("logImpl", settings.LogImpl, false); err != nil {
		return err
	}
	if err := validateSettingToken("proxyFactory", settings.ProxyFactory, false); err != nil {
		return err
	}
	if _, err := normalizeSettingList("vfsImpl", settings.VFSImpl, true); err != nil {
		return err
	}
	if err := validateSettingToken("configurationFactory", settings.ConfigurationFactory, true); err != nil {
		return err
	}
	return validateSettingToken("defaultSqlProviderType", settings.DefaultSQLProviderType, true)
}

func defaultLazyLoadTriggerMethods() []string {
	return []string{"equals", "clone", "hashCode", "toString"}
}

func normalizeSettingList(label string, values []string, allowSlash bool) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if err := validateSettingToken(label, value, allowSlash); err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func validateSettingToken(label string, value string, allowSlash bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '_', '-', '.':
			continue
		case '/':
			if allowSlash {
				continue
			}
		}
		return configurationErrorf("%s %q contains invalid character %q", label, value, r)
	}
	return nil
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
