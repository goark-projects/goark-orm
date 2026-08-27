package ormboot

import (
	"sync"

	orm "goark.dev/orm"
)

// Runtime 保存 ORM boot 装配后的运行时对象。
type Runtime struct {
	name      string
	beanNames BeanNames
	result    orm.MyBatisAssemblyResult
	closeOnce sync.Once
	closeErr  error
}

// Name 返回运行时名称。
func (r *Runtime) Name() string {
	if r == nil {
		return ""
	}
	return r.name
}

// Configuration 返回 ORM 运行期配置快照。
func (r *Runtime) Configuration() orm.Configuration {
	if r == nil {
		return orm.DefaultConfiguration()
	}
	return r.result.Configuration
}

// Registry 返回 ORM 元数据注册表。
func (r *Runtime) Registry() *orm.Registry {
	if r == nil {
		return nil
	}
	return r.result.Registry
}

// SessionFactory 返回可创建普通、批量和事务 Session 的工厂。
func (r *Runtime) SessionFactory() *orm.SQLSessionFactory {
	if r == nil {
		return nil
	}
	return r.result.SessionFactory
}

// DefaultSession 返回装配阶段创建的默认 Session。
func (r *Runtime) DefaultSession() *orm.SQLSession {
	if r == nil {
		return nil
	}
	return r.result.Session
}

// Close 释放默认 Session；外部传入的 *sql.DB 仍由调用方管理。
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.result.Session != nil {
			r.closeErr = r.result.Session.Close()
		}
	})
	return r.closeErr
}
