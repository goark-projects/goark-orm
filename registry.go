package orm

import (
	"fmt"
	"sync"
)

// Registry 保存编译期生成的 ORM 元数据。
type Registry struct {
	mu         sync.RWMutex
	entities   map[string]EntityMeta
	mappers    map[string]MapperMeta
	statements map[string]StatementMeta
}

// NewRegistry 创建空的 ORM 元数据注册表。
func NewRegistry() *Registry {
	return &Registry{
		entities:   make(map[string]EntityMeta),
		mappers:    make(map[string]MapperMeta),
		statements: make(map[string]StatementMeta),
	}
}

// RegisterEntity 注册实体元数据。
func (r *Registry) RegisterEntity(meta EntityMeta) error {
	if r == nil {
		return fmt.Errorf("goark-orm: registry is nil")
	}
	if meta.TypeName == "" {
		return fmt.Errorf("goark-orm: entity type name is required")
	}
	if meta.Table == "" {
		return fmt.Errorf("goark-orm: entity %s table is required", meta.TypeName)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entities[meta.TypeName]; exists {
		return fmt.Errorf("goark-orm: duplicate entity %q", meta.TypeName)
	}
	r.entities[meta.TypeName] = copyEntityMeta(meta)
	return nil
}

// RegisterMapper 注册 Mapper 元数据，并同步注册内部 Statement。
func (r *Registry) RegisterMapper(meta MapperMeta) error {
	if r == nil {
		return fmt.Errorf("goark-orm: registry is nil")
	}
	if meta.TypeName == "" {
		return fmt.Errorf("goark-orm: mapper type name is required")
	}
	if meta.Namespace == "" {
		return fmt.Errorf("goark-orm: mapper %s namespace is required", meta.TypeName)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.mappers[meta.Namespace]; exists {
		return fmt.Errorf("goark-orm: duplicate mapper namespace %q", meta.Namespace)
	}
	for _, statement := range meta.Statements {
		if statement.FullName == "" {
			return fmt.Errorf("goark-orm: mapper %s has statement without full name", meta.TypeName)
		}
		if _, exists := r.statements[statement.FullName]; exists {
			return fmt.Errorf("goark-orm: duplicate statement %q", statement.FullName)
		}
	}
	copied := copyMapperMeta(meta)
	r.mappers[copied.Namespace] = copied
	for _, statement := range copied.Statements {
		r.statements[statement.FullName] = statement
	}
	return nil
}

// Entity 按实体类型名读取元数据。
func (r *Registry) Entity(typeName string) (EntityMeta, bool) {
	if r == nil {
		return EntityMeta{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.entities[typeName]
	if !ok {
		return EntityMeta{}, false
	}
	return copyEntityMeta(meta), true
}

// Mapper 按 namespace 读取 Mapper 元数据。
func (r *Registry) Mapper(namespace string) (MapperMeta, bool) {
	if r == nil {
		return MapperMeta{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.mappers[namespace]
	if !ok {
		return MapperMeta{}, false
	}
	return copyMapperMeta(meta), true
}

// Statement 按完整 Statement 名称读取语句元数据。
func (r *Registry) Statement(fullName string) (StatementMeta, bool) {
	if r == nil {
		return StatementMeta{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.statements[fullName]
	return meta, ok
}

// Entities 返回所有实体元数据副本。
func (r *Registry) Entities() []EntityMeta {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EntityMeta, 0, len(r.entities))
	for _, meta := range r.entities {
		out = append(out, copyEntityMeta(meta))
	}
	return out
}

// Mappers 返回所有 Mapper 元数据副本。
func (r *Registry) Mappers() []MapperMeta {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MapperMeta, 0, len(r.mappers))
	for _, meta := range r.mappers {
		out = append(out, copyMapperMeta(meta))
	}
	return out
}

func copyEntityMeta(meta EntityMeta) EntityMeta {
	meta.Columns = append([]ColumnMeta(nil), meta.Columns...)
	return meta
}

func copyMapperMeta(meta MapperMeta) MapperMeta {
	meta.ResultMaps = append([]ResultMapMeta(nil), meta.ResultMaps...)
	for index := range meta.ResultMaps {
		meta.ResultMaps[index].Fields = append([]ResultFieldMeta(nil), meta.ResultMaps[index].Fields...)
	}
	meta.Statements = append([]StatementMeta(nil), meta.Statements...)
	for index := range meta.Statements {
		meta.Statements[index].DynamicSQL = copyDynamicSQLNodes(meta.Statements[index].DynamicSQL)
	}
	return meta
}

func copyDynamicSQLNodes(nodes []DynamicSQLNode) []DynamicSQLNode {
	if len(nodes) == 0 {
		return nil
	}
	out := append([]DynamicSQLNode(nil), nodes...)
	for index := range out {
		out[index].Children = copyDynamicSQLNodes(out[index].Children)
	}
	return out
}
