// Package ormboot 提供 Goark Boot 风格的 ORM 装配边界。
//
// 该包只依赖 goark.dev/orm 和 database/sql，不反向依赖 Goark 核心容器。
// 上层 boot 或业务模块可以将 BeanRegistrations 返回的实例注册到自己的容器中。
package ormboot
