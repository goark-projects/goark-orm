package orm

import (
	"strconv"
	"strings"
)

// PluginRef 描述运行期插件声明。内置插件直接按名称装配，自定义插件由调用方显式提供。
type PluginRef struct {
	Name    string            `json:"name"`
	Enabled *bool             `json:"enabled,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// PluginRegistry 保存外部集成层显式提供的插件实例。
type PluginRegistry map[string]StatementInterceptor

// SessionOptions 将配置声明转换为 SQLSessionOption。
func (c MyBatisConfig) SessionOptions(plugins PluginRegistry) ([]SQLSessionOption, error) {
	interceptors := make([]StatementInterceptor, 0, len(c.Plugins))
	for _, plugin := range c.Plugins {
		if plugin.Enabled != nil && !*plugin.Enabled {
			continue
		}
		interceptor, err := buildStatementInterceptor(plugin, plugins)
		if err != nil {
			return nil, err
		}
		interceptors = append(interceptors, interceptor)
	}
	if len(interceptors) == 0 {
		return nil, nil
	}
	return []SQLSessionOption{WithInterceptors(interceptors...)}, nil
}

func validatePluginRefs(plugins []PluginRef) error {
	seen := make(map[string]struct{}, len(plugins))
	for _, plugin := range plugins {
		name := normalizePluginName(plugin.Name)
		if name == "" {
			return configurationErrorf("plugin name is required")
		}
		if _, exists := seen[name]; exists {
			return configurationErrorf("duplicate plugin %q", plugin.Name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func buildStatementInterceptor(plugin PluginRef, plugins PluginRegistry) (StatementInterceptor, error) {
	name := normalizePluginName(plugin.Name)
	if name == "" {
		return nil, configurationErrorf("plugin name is required")
	}
	switch name {
	case "pagination":
		if err := rejectPluginOptions(plugin, nil); err != nil {
			return nil, err
		}
		return NewPaginationInterceptor(), nil
	case "blockattack":
		if err := rejectPluginOptions(plugin, nil); err != nil {
			return nil, err
		}
		return NewBlockAttackInterceptor(), nil
	case "readonly":
		if err := rejectPluginOptions(plugin, nil); err != nil {
			return nil, err
		}
		return NewReadOnlyInterceptor(), nil
	case "illegalsql":
		options, err := illegalSQLOptions(plugin.Options)
		if err != nil {
			return nil, err
		}
		return NewIllegalSQLInterceptor(options...), nil
	default:
		interceptor, ok := lookupPlugin(plugins, plugin.Name)
		if !ok {
			return nil, configurationErrorf("plugin %q is not registered", plugin.Name)
		}
		if err := rejectPluginOptions(plugin, nil); err != nil {
			return nil, err
		}
		return interceptor, nil
	}
}

func illegalSQLOptions(values map[string]string) ([]IllegalSQLOption, error) {
	allowed := map[string]func(bool) IllegalSQLOption{
		"denyselectwildcard":     WithIllegalSQLDenySelectWildcard,
		"denymultiplestatements": WithIllegalSQLDenyMultipleStatements,
		"denywritewithoutwhere":  WithIllegalSQLDenyWriteWithoutWhere,
	}
	options := make([]IllegalSQLOption, 0, len(values))
	for key, value := range values {
		normalized := normalizePluginName(key)
		build, ok := allowed[normalized]
		if !ok {
			return nil, configurationErrorf("plugin illegalSQL option %q is invalid", key)
		}
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return nil, configurationErrorf("plugin illegalSQL option %q requires boolean value", key)
		}
		options = append(options, build(enabled))
	}
	return options, nil
}

func rejectPluginOptions(plugin PluginRef, allowed map[string]struct{}) error {
	for key := range plugin.Options {
		normalized := normalizePluginName(key)
		if _, ok := allowed[normalized]; !ok {
			return configurationErrorf("plugin %q option %q is invalid", plugin.Name, key)
		}
	}
	return nil
}

func lookupPlugin(plugins PluginRegistry, name string) (StatementInterceptor, bool) {
	normalized := normalizePluginName(name)
	for candidate, interceptor := range plugins {
		if normalizePluginName(candidate) == normalized && interceptor != nil {
			return interceptor, true
		}
	}
	return nil, false
}

func normalizePluginName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return strings.ReplaceAll(value, " ", "")
}
