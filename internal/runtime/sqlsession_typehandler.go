package runtime

import "strings"

// WithTypeHandlers 批量注册 Session 级 TypeHandler。
func WithTypeHandlers(handlers map[string]TypeHandler) SQLSessionOption {
	return func(session *SQLSession) error {
		normalized := make(map[string]TypeHandler, len(handlers))
		for name, handler := range handlers {
			name = strings.TrimSpace(name)
			if name == "" {
				return configurationErrorf("type-handler name is required")
			}
			if handler == nil {
				return configurationErrorf("type-handler %q is nil", name)
			}
			normalized[name] = handler
		}
		for _, name := range sortedTypeHandlerNames(normalized) {
			session.typeHandlers[name] = normalized[name]
		}
		return nil
	}
}
