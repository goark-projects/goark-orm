package orm

import (
	"fmt"
	"strings"
)

// ConfigProperties 描述运行期配置文件中的占位符属性。
type ConfigProperties map[string]string

type configPropertyResolver struct {
	values map[string]string
}

func newConfigPropertyResolver(properties ConfigProperties) (*configPropertyResolver, error) {
	values := make(map[string]string, len(properties))
	for key, value := range properties {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, configurationErrorf("property name is required")
		}
		if _, exists := values[key]; exists {
			return nil, configurationErrorf("duplicate property %q", key)
		}
		values[key] = value
	}
	resolver := &configPropertyResolver{values: values}
	for key := range values {
		resolved, err := resolver.resolveProperty(key, map[string]struct{}{})
		if err != nil {
			return nil, err
		}
		values[key] = resolved
	}
	return resolver, nil
}

func (r *configPropertyResolver) Resolve(value string) (string, error) {
	if r == nil {
		r = &configPropertyResolver{}
	}
	return r.resolveString(value, map[string]struct{}{})
}

func (r *configPropertyResolver) Properties() ConfigProperties {
	if r == nil {
		return nil
	}
	return copyConfigProperties(r.values)
}

func (r *configPropertyResolver) resolveProperty(name string, stack map[string]struct{}) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", configurationErrorf("property name is required")
	}
	value, ok := r.values[name]
	if !ok {
		return "", configurationErrorf("property %q is not defined", name)
	}
	if _, exists := stack[name]; exists {
		return "", configurationErrorf("property %q contains circular reference", name)
	}
	stack[name] = struct{}{}
	resolved, err := r.resolveString(value, stack)
	delete(stack, name)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (r *configPropertyResolver) resolveString(value string, stack map[string]struct{}) (string, error) {
	start := strings.Index(value, "${")
	if start < 0 {
		return value, nil
	}
	var builder strings.Builder
	builder.Grow(len(value))
	cursor := 0
	for start >= 0 {
		start += cursor
		builder.WriteString(value[cursor:start])
		nameStart := start + 2
		endOffset := strings.IndexByte(value[nameStart:], '}')
		if endOffset < 0 {
			return "", configurationErrorf("property placeholder is not closed")
		}
		end := nameStart + endOffset
		name := strings.TrimSpace(value[nameStart:end])
		replacement, err := r.resolveProperty(name, stack)
		if err != nil {
			return "", err
		}
		builder.WriteString(replacement)
		cursor = end + 1
		next := strings.Index(value[cursor:], "${")
		start = next
	}
	builder.WriteString(value[cursor:])
	return builder.String(), nil
}

func resolveConfigString(resolver *configPropertyResolver, value string) (string, error) {
	resolved, err := resolver.Resolve(value)
	if err != nil {
		return "", fmt.Errorf("resolve config string %q failed: %w", value, err)
	}
	return resolved, nil
}
