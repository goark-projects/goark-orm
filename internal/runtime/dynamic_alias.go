package runtime

import "strings"

// rewriteDynamicParameterAliases 在 foreach 热路径替换参数根别名，避免正则子匹配分配。
func rewriteDynamicParameterAliases(text string, aliases map[string]string) string {
	var builder strings.Builder
	offset := 0
	changed := false
	for index := 0; index < len(text); {
		if index+1 >= len(text) || text[index] != '#' || text[index+1] != '{' {
			index++
			continue
		}
		end, name, err := readSQLPlaceholderName(text, index)
		if err != nil {
			index++
			continue
		}
		rewritten, ok := rewriteDynamicParameterName(name, aliases)
		if !ok {
			index = end
			continue
		}
		if !changed {
			builder.Grow(len(text) + len(rewritten) - len(name))
			changed = true
		}
		builder.WriteString(text[offset:index])
		builder.WriteString("#{")
		builder.WriteString(rewritten)
		builder.WriteByte('}')
		offset = end
		index = end
	}
	if !changed {
		return text
	}
	builder.WriteString(text[offset:])
	return builder.String()
}

func rewriteDynamicParameterName(name string, aliases map[string]string) (string, bool) {
	name = strings.TrimSpace(name)
	root, next := scanPathIdentifier(name, 0)
	if root == "" {
		return "", false
	}
	alias, ok := aliases[root]
	if !ok {
		return "", false
	}
	if next == len(name) {
		return alias, true
	}
	switch name[next] {
	case '.', '[':
	default:
		return "", false
	}
	if _, err := parseParameterPath(name); err != nil {
		return "", false
	}
	return alias + name[next:], true
}
