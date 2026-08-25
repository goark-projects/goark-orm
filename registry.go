package orm

import (
	"strings"
	"sync"
)

// Registry 保存编译期生成的 ORM 元数据。
type Registry struct {
	mu         sync.RWMutex
	entities   map[string]EntityMeta
	mappers    map[string]MapperMeta
	statements map[string]StatementMeta
	caches     map[string]Cache
	cacheRefs  map[string]string
	handlers   map[string]TypeHandler
	providers  map[string]SQLProvider
}

// NewRegistry 创建空的 ORM 元数据注册表。
func NewRegistry() *Registry {
	return &Registry{
		entities:   make(map[string]EntityMeta),
		mappers:    make(map[string]MapperMeta),
		statements: make(map[string]StatementMeta),
		caches:     make(map[string]Cache),
		cacheRefs:  make(map[string]string),
		handlers:   defaultTypeHandlers(),
		providers:  make(map[string]SQLProvider),
	}
}

// RegisterEntity 注册实体元数据。
func (r *Registry) RegisterEntity(meta EntityMeta) error {
	if r == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	if meta.TypeName == "" {
		return registryErrorf("entity", "", "entity type name is required")
	}
	if meta.Table == "" {
		return registryErrorf("entity", meta.TypeName, "entity %s table is required", meta.TypeName)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entities[meta.TypeName]; exists {
		return registryErrorf("entity", meta.TypeName, "duplicate entity %q", meta.TypeName)
	}
	r.entities[meta.TypeName] = copyEntityMeta(meta)
	return nil
}

// RegisterMapper 注册 Mapper 元数据，并同步注册内部 Statement。
func (r *Registry) RegisterMapper(meta MapperMeta) error {
	if r == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	if meta.TypeName == "" {
		return registryErrorf("mapper", "", "mapper type name is required")
	}
	if meta.Namespace == "" {
		return registryErrorf("mapper", meta.TypeName, "mapper %s namespace is required", meta.TypeName)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.mappers[meta.Namespace]; exists {
		return registryErrorf("mapper", meta.Namespace, "duplicate mapper namespace %q", meta.Namespace)
	}
	for _, statement := range meta.Statements {
		if statement.FullName == "" {
			return registryErrorf("statement", meta.TypeName, "mapper %s has statement without full name", meta.TypeName)
		}
		if _, exists := r.statements[statement.FullName]; exists {
			return registryErrorf("statement", statement.FullName, "duplicate statement %q", statement.FullName)
		}
	}
	copied := copyMapperMeta(meta)
	r.mappers[copied.Namespace] = copied
	if copied.Cache.Enabled {
		if copied.Cache.RefNamespace != "" {
			r.cacheRefs[copied.Namespace] = copied.Cache.RefNamespace
		} else if _, exists := r.caches[copied.Namespace]; !exists {
			r.caches[copied.Namespace] = newMemoryCacheFromMeta(copied.Namespace, copied.Cache)
		}
	}
	for _, statement := range copied.Statements {
		r.statements[statement.FullName] = statement
	}
	return nil
}

// RegisterCache 注册或替换指定 namespace 的二级缓存实现。
func (r *Registry) RegisterCache(namespace string, cache Cache) error {
	if r == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return registryErrorf("cache", "", "cache namespace is required")
	}
	if cache == nil {
		return registryErrorf("cache", namespace, "cache for namespace %s is nil", namespace)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.caches[namespace] = cache
	return nil
}

// RegisterTypeHandler 注册或替换全局 TypeHandler。
func (r *Registry) RegisterTypeHandler(name string, handler TypeHandler) error {
	if r == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return registryErrorf("type-handler", "", "type-handler name is required")
	}
	if handler == nil {
		return registryErrorf("type-handler", name, "type-handler %q is nil", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
	return nil
}

// TypeHandler 按名称读取全局 TypeHandler。
func (r *Registry) TypeHandler(name string) (TypeHandler, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[strings.TrimSpace(name)]
	return handler, ok
}

// TypeHandlers 返回全局 TypeHandler 快照。
func (r *Registry) TypeHandlers() map[string]TypeHandler {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]TypeHandler, len(r.handlers))
	for name, handler := range r.handlers {
		out[name] = handler
	}
	return out
}

// RegisterSQLProvider 注册或替换全局 SQL Provider。
func (r *Registry) RegisterSQLProvider(name string, provider SQLProvider) error {
	if r == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return registryErrorf("SQL provider", "", "SQL provider name is required")
	}
	if provider == nil {
		return registryErrorf("SQL provider", name, "SQL provider %q is nil", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
	return nil
}

// SQLProvider 按名称读取全局 SQL Provider。
func (r *Registry) SQLProvider(name string) (SQLProvider, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[strings.TrimSpace(name)]
	return provider, ok
}

// Cache 按 mapper namespace 读取二级缓存，自动解析 cache-ref。
func (r *Registry) Cache(namespace string) (Cache, string, bool) {
	if r == nil {
		return nil, "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	resolved, ok := r.resolveCacheNamespaceLocked(namespace)
	if !ok {
		return nil, "", false
	}
	cache, ok := r.caches[resolved]
	return cache, resolved, ok
}

func (r *Registry) resolveCacheNamespaceLocked(namespace string) (string, bool) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "", false
	}
	seen := make(map[string]struct{})
	current := namespace
	for {
		if _, ok := seen[current]; ok {
			return "", false
		}
		seen[current] = struct{}{}
		ref := strings.TrimSpace(r.cacheRefs[current])
		if ref == "" {
			if _, ok := r.caches[current]; ok {
				return current, true
			}
			return "", false
		}
		current = ref
	}
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
		meta.ResultMaps[index] = copyResultMapMeta(meta.ResultMaps[index])
	}
	meta.Statements = append([]StatementMeta(nil), meta.Statements...)
	for index := range meta.Statements {
		meta.Statements[index].Parameters = append([]string(nil), meta.Statements[index].Parameters...)
		meta.Statements[index].DynamicSQL = copyDynamicSQLNodes(meta.Statements[index].DynamicSQL)
		meta.Statements[index].SelectKey.DynamicSQL = copyDynamicSQLNodes(meta.Statements[index].SelectKey.DynamicSQL)
	}
	return meta
}

func copyResultMapMeta(meta ResultMapMeta) ResultMapMeta {
	if meta.AutoMapping != nil {
		value := *meta.AutoMapping
		meta.AutoMapping = &value
	}
	meta.Constructor.Args = append([]ResultArgMeta(nil), meta.Constructor.Args...)
	meta.Fields = append([]ResultFieldMeta(nil), meta.Fields...)
	meta.Associations = copyResultAssociationMetas(meta.Associations)
	meta.Collections = copyResultCollectionMetas(meta.Collections)
	meta.Discriminator.Cases = copyResultDiscriminatorCaseMetas(meta.Discriminator.Cases)
	return meta
}

func copyResultDiscriminatorCaseMetas(items []ResultDiscriminatorCaseMeta) []ResultDiscriminatorCaseMeta {
	if len(items) == 0 {
		return nil
	}
	out := append([]ResultDiscriminatorCaseMeta(nil), items...)
	for index := range out {
		out[index].Fields = append([]ResultFieldMeta(nil), out[index].Fields...)
		out[index].Associations = copyResultAssociationMetas(out[index].Associations)
		out[index].Collections = copyResultCollectionMetas(out[index].Collections)
	}
	return out
}

func copyResultAssociationMetas(items []ResultAssociationMeta) []ResultAssociationMeta {
	if len(items) == 0 {
		return nil
	}
	out := append([]ResultAssociationMeta(nil), items...)
	for index := range out {
		out[index].NotNullColumns = append([]string(nil), out[index].NotNullColumns...)
		out[index].Fields = append([]ResultFieldMeta(nil), out[index].Fields...)
		out[index].Associations = copyResultAssociationMetas(out[index].Associations)
		out[index].Collections = copyResultCollectionMetas(out[index].Collections)
	}
	return out
}

func copyResultCollectionMetas(items []ResultCollectionMeta) []ResultCollectionMeta {
	if len(items) == 0 {
		return nil
	}
	out := append([]ResultCollectionMeta(nil), items...)
	for index := range out {
		out[index].NotNullColumns = append([]string(nil), out[index].NotNullColumns...)
		out[index].Fields = append([]ResultFieldMeta(nil), out[index].Fields...)
		out[index].Associations = copyResultAssociationMetas(out[index].Associations)
		out[index].Collections = copyResultCollectionMetas(out[index].Collections)
	}
	return out
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
