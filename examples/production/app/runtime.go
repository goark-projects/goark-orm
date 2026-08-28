package app

import (
	"context"
	"fmt"
	"strings"

	orm "goark.dev/orm"
	"goark.dev/orm/audit"
	"goark.dev/orm/examples/production/account"
)

// Runtime 聚合生产示例中由 ORM 装配出的运行期对象。
type Runtime struct {
	assembly orm.RuntimeAssemblyResult
	Users    *UserApplication
}

// Assemble 使用显式元数据、运行期配置和调用方数据库连接池装配示例应用。
func Assemble(ctx context.Context, options RuntimeOptions) (*Runtime, error) {
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm example: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options.DB == nil {
		return nil, fmt.Errorf("goark-orm example: database is nil")
	}
	registry, err := newRegistry()
	if err != nil {
		return nil, err
	}
	configPath := strings.TrimSpace(options.ConfigPath)
	if configPath == "" {
		configPath = DefaultRuntimeConfigPath
	}
	config, err := orm.LoadRuntimeConfig(configPath)
	if err != nil {
		return nil, err
	}
	assembled, err := orm.AssembleRuntimeConfig(orm.RuntimeAssembly{
		Config:         config,
		Registry:       registry,
		DB:             options.DB,
		TypeHandlers:   map[string]orm.TypeHandler{"json": orm.NewJSONTypeHandler()},
		SessionOptions: runtimeSessionOptions(options),
	})
	if err != nil {
		return nil, err
	}
	users, err := NewUserApplication(account.NewUserMapper(assembled.Session), UserApplicationOptions{})
	if err != nil {
		_ = assembled.Session.Close()
		return nil, err
	}
	return &Runtime{assembly: assembled, Users: users}, nil
}

// Close 关闭示例运行期创建的 ORM Session。
func (r *Runtime) Close() error {
	if r == nil || r.assembly.Session == nil {
		return nil
	}
	return r.assembly.Session.Close()
}

// Registry 返回已校验的元数据注册表。
func (r *Runtime) Registry() *orm.Registry {
	if r == nil {
		return nil
	}
	return r.assembly.Registry
}

// SessionFactory 返回 ORM SessionFactory。
func (r *Runtime) SessionFactory() *orm.SQLSessionFactory {
	if r == nil {
		return nil
	}
	return r.assembly.SessionFactory
}

func newRegistry() (*orm.Registry, error) {
	registry := orm.NewRegistry()
	if err := account.RegisterGoarkORMMetadata(registry); err != nil {
		return nil, err
	}
	if err := account.RegisterSQLProviders(registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func runtimeSessionOptions(options RuntimeOptions) []orm.SQLSessionOption {
	out := []orm.SQLSessionOption{
		orm.WithMetaObjectHandler(account.NewAuditFillHandler(options.Clock)),
	}
	if options.AuditRecorder != nil {
		out = append(out, orm.WithStatementExecutorMiddleware(
			audit.NewMiddleware(options.AuditRecorder, audit.WithErrorEvents(true)),
		))
	}
	if options.SQLObserver != nil {
		out = append(out, orm.WithInterceptors(orm.NewSQLObserverInterceptor(options.SQLObserver)))
	}
	out = append(out, options.SessionOptions...)
	return out
}
