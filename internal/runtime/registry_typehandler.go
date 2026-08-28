package runtime

import "strings"

// RegisterTypeHandlers 批量注册或替换全局 TypeHandler。
func (r *Registry) RegisterTypeHandlers(handlers map[string]TypeHandler) error {
	if r == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	normalized, err := normalizeTypeHandlerMap(handlers)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, name := range sortedTypeHandlerNames(normalized) {
		r.handlers[name] = normalized[name]
	}
	return nil
}

func normalizeTypeHandlerMap(handlers map[string]TypeHandler) (map[string]TypeHandler, error) {
	normalized := make(map[string]TypeHandler, len(handlers))
	for name, handler := range handlers {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, registryErrorf("type-handler", "", "type-handler name is required")
		}
		if handler == nil {
			return nil, registryErrorf("type-handler", name, "type-handler %q is nil", name)
		}
		normalized[name] = handler
	}
	return normalized, nil
}
