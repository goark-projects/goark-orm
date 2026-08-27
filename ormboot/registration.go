package ormboot

// BeanRegistration 描述可交给上层容器注册的运行时实例。
type BeanRegistration struct {
	Name     string
	Instance any
}

// BeanRegistrations 返回默认注册项，供 Goark 或其他容器适配层使用。
func (r *Runtime) BeanRegistrations() []BeanRegistration {
	if r == nil {
		return nil
	}
	registrations := []BeanRegistration{
		{Name: r.beanNames.Runtime, Instance: r},
		{Name: r.beanNames.Registry, Instance: r.Registry()},
		{Name: r.beanNames.Configuration, Instance: r.Configuration()},
		{Name: r.beanNames.SessionFactory, Instance: r.SessionFactory()},
	}
	out := registrations[:0]
	for _, registration := range registrations {
		if registration.Name != "" && registration.Instance != nil {
			out = append(out, registration)
		}
	}
	return out
}
