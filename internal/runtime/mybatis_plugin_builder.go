package runtime

import "context"

// PluginRegistryBuilder 以显式 builder 装配需要 Go 函数的插件实例。
type PluginRegistryBuilder struct {
	plugins PluginRegistry
	err     error
}

// NewPluginRegistryBuilder 创建插件注册表 builder。
func NewPluginRegistryBuilder() *PluginRegistryBuilder {
	return &PluginRegistryBuilder{plugins: make(PluginRegistry)}
}

// With 注册自定义 StatementInterceptor。
func (b *PluginRegistryBuilder) With(name string, interceptor StatementInterceptor) *PluginRegistryBuilder {
	return b.add(name, interceptor)
}

// WithTenant 注册租户插件。
func (b *PluginRegistryBuilder) WithTenant(column string, value any) *PluginRegistryBuilder {
	return b.add(InterceptorNameTenant, NewTenantInterceptor(column, value))
}

// WithDynamicTable 注册动态表名插件。
func (b *PluginRegistryBuilder) WithDynamicTable(tables map[string]string) *PluginRegistryBuilder {
	return b.add(InterceptorNameDynamicTable, NewDynamicTableInterceptor(tables))
}

// WithDataPermission 注册数据权限插件。
func (b *PluginRegistryBuilder) WithDataPermission(provider DataPermissionProvider) *PluginRegistryBuilder {
	return b.add(InterceptorNameDataPermission, NewDataPermissionInterceptor(provider))
}

// WithSQLObserver 注册 SQL 观察插件。
func (b *PluginRegistryBuilder) WithSQLObserver(observe func(context.Context, SQLObservation) error) *PluginRegistryBuilder {
	return b.add(InterceptorNameSQLObserver, NewSQLObserverInterceptor(observe))
}

// Build 返回不可变注册表副本。
func (b *PluginRegistryBuilder) Build() (PluginRegistry, error) {
	if b == nil {
		return nil, configurationErrorf("plugin registry builder is nil")
	}
	if b.err != nil {
		return nil, b.err
	}
	out := make(PluginRegistry, len(b.plugins))
	for name, interceptor := range b.plugins {
		out[name] = interceptor
	}
	return out, nil
}

func (b *PluginRegistryBuilder) add(name string, interceptor StatementInterceptor) *PluginRegistryBuilder {
	if b == nil {
		return b
	}
	if b.err != nil {
		return b
	}
	normalized := normalizePluginName(name)
	if normalized == "" {
		b.err = configurationErrorf("plugin name is required")
		return b
	}
	if interceptor == nil {
		b.err = configurationErrorf("plugin %q interceptor is nil", name)
		return b
	}
	if _, exists := b.plugins[normalized]; exists {
		b.err = configurationErrorf("duplicate plugin %q", name)
		return b
	}
	b.plugins[normalized] = interceptor
	return b
}
