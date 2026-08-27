package dbkit

import (
	"fmt"
	"reflect"
	"strings"

	orm "goark.dev/orm"
)

// Kit 是围绕 BaseMapper 与 Service 的轻量门面。
type Kit[T any, ID any] struct {
	service *orm.Service[T, ID]
}

// New 基于已有 BaseMapper 创建 Kit。
func New[T any, ID any](mapper *orm.BaseMapper[T, ID]) (*Kit[T, ID], error) {
	service, err := orm.NewService(mapper)
	if err != nil {
		return nil, err
	}
	return &Kit[T, ID]{service: service}, nil
}

// NewWithEntity 基于显式实体元数据创建 Kit。
func NewWithEntity[T any, ID any](session orm.StatementSession, entity orm.EntityMeta, options ...orm.BaseMapperOption) (*Kit[T, ID], error) {
	mapper, err := orm.NewBaseMapper[T, ID](session, entity, options...)
	if err != nil {
		return nil, err
	}
	return New(mapper)
}

// NewFromRegistry 使用实体类型名从注册表创建 Kit。
func NewFromRegistry[T any, ID any](session orm.StatementSession, registry *orm.Registry, options ...orm.BaseMapperOption) (*Kit[T, ID], error) {
	typeName, err := entityTypeName[T]()
	if err != nil {
		return nil, err
	}
	return NewFromRegistryName[T, ID](session, registry, typeName, options...)
}

// NewFromRegistryName 使用显式实体类型名从注册表创建 Kit。
func NewFromRegistryName[T any, ID any](session orm.StatementSession, registry *orm.Registry, typeName string, options ...orm.BaseMapperOption) (*Kit[T, ID], error) {
	if registry == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: registry is nil")
	}
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return nil, fmt.Errorf("goark-orm/dbkit: entity type name is required")
	}
	entity, ok := registry.Entity(typeName)
	if !ok {
		return nil, fmt.Errorf("goark-orm/dbkit: entity %s is not registered", typeName)
	}
	return NewWithEntity[T, ID](session, entity, options...)
}

// Service 返回底层服务层实例。
func (k *Kit[T, ID]) Service() *orm.Service[T, ID] {
	if k == nil {
		return nil
	}
	return k.service
}

// BaseMapper 返回底层通用 Mapper。
func (k *Kit[T, ID]) BaseMapper() *orm.BaseMapper[T, ID] {
	if k == nil || k.service == nil {
		return nil
	}
	return k.service.BaseMapper()
}

func (k *Kit[T, ID]) requireService() (*orm.Service[T, ID], error) {
	if k == nil || k.service == nil {
		return nil, fmt.Errorf("goark-orm/dbkit: service is nil")
	}
	return k.service, nil
}

func entityTypeName[T any]() (string, error) {
	entityType := reflect.TypeFor[T]()
	if entityType.Kind() != reflect.Struct {
		return "", fmt.Errorf("goark-orm/dbkit: entity type must be a struct, got %s", entityType.Kind())
	}
	if entityType.Name() == "" {
		return "", fmt.Errorf("goark-orm/dbkit: entity type name is required")
	}
	return entityType.Name(), nil
}
