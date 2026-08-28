package runtime

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// RenderedSQL 表示动态 SQL 渲染后的 SQL 和参数。
type RenderedSQL struct {
	SQL  string
	Args NamedArgs
}

// DynamicSQLRenderOptions 描述动态 SQL 渲染期选项。
type DynamicSQLRenderOptions struct {
	NullableOnForEach      bool
	ShrinkWhitespacesInSQL bool
}

type dynamicRenderContext struct {
	args                   NamedArgs
	aliases                map[string]string
	values                 map[string]any
	nullableOnForEach      bool
	shrinkWhitespacesInSQL bool
	seq                    int
}

// RenderDynamicSQL 将动态 SQL 节点树渲染为可继续参数编译的 SQL。
func RenderDynamicSQL(nodes []DynamicSQLNode, args NamedArgs) (RenderedSQL, error) {
	return RenderDynamicSQLWithOptions(nodes, args, DynamicSQLRenderOptions{NullableOnForEach: true})
}

// RenderDynamicSQLWithOptions 按指定选项渲染动态 SQL 节点树。
func RenderDynamicSQLWithOptions(nodes []DynamicSQLNode, args NamedArgs, options DynamicSQLRenderOptions) (RenderedSQL, error) {
	ctx := &dynamicRenderContext{
		args:                   copyNamedArgs(args),
		aliases:                make(map[string]string),
		values:                 make(map[string]any),
		nullableOnForEach:      options.NullableOnForEach,
		shrinkWhitespacesInSQL: options.ShrinkWhitespacesInSQL,
	}
	fragments, err := ctx.renderNodes(nodes)
	if err != nil {
		return RenderedSQL{}, err
	}
	sqlText := joinSQLFragments(fragments)
	if ctx.shrinkWhitespacesInSQL {
		sqlText = ShrinkSQLWhitespaces(sqlText)
	}
	return RenderedSQL{
		SQL:  sqlText,
		Args: ctx.args,
	}, nil
}

func copyNamedArgs(args NamedArgs) NamedArgs {
	out := make(NamedArgs, len(args))
	for key, value := range args {
		out[key] = value
	}
	return out
}

func (c *dynamicRenderContext) renderNodes(nodes []DynamicSQLNode) ([]string, error) {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		rendered, err := c.renderNode(node)
		if err != nil {
			return nil, err
		}
		if rendered != "" {
			out = append(out, rendered)
		}
	}
	return out, nil
}

func (c *dynamicRenderContext) renderNode(node DynamicSQLNode) (string, error) {
	switch node.Kind {
	case DynamicSQLNodeText:
		return c.renderText(node.Text), nil
	case DynamicSQLNodeIf:
		ok, err := c.eval(node.Test)
		if err != nil || !ok {
			return "", err
		}
		return c.renderChildren(node.Children)
	case DynamicSQLNodeWhere:
		content, err := c.renderChildren(node.Children)
		if err != nil || content == "" {
			return "", err
		}
		content = removePrefixOverride(content, []string{"AND", "OR"})
		if content == "" {
			return "", nil
		}
		return "WHERE " + content, nil
	case DynamicSQLNodeSet:
		content, err := c.renderChildren(node.Children)
		if err != nil || content == "" {
			return "", err
		}
		content = removeSuffixOverride(content, []string{","})
		if content == "" {
			return "", nil
		}
		return "SET " + content, nil
	case DynamicSQLNodeTrim:
		return c.renderTrim(node)
	case DynamicSQLNodeForeach:
		return c.renderForeach(node)
	case DynamicSQLNodeChoose:
		return c.renderChoose(node)
	case DynamicSQLNodeWhen, DynamicSQLNodeOtherwise:
		return c.renderChildren(node.Children)
	case DynamicSQLNodeInclude:
		return "", fmt.Errorf("goark-orm: dynamic SQL include %q was not expanded", node.RefID)
	case DynamicSQLNodeBind:
		return "", c.renderBind(node)
	default:
		return "", fmt.Errorf("goark-orm: unsupported dynamic SQL node %q", node.Kind)
	}
}

func (c *dynamicRenderContext) renderBind(node DynamicSQLNode) error {
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return fmt.Errorf("goark-orm: bind name is required")
	}
	if !validIdentifierPart(name) {
		return fmt.Errorf("goark-orm: bind name %q is invalid", name)
	}
	if strings.Contains(node.Value, "${") {
		return fmt.Errorf("goark-orm: bind %s uses forbidden ${}", name)
	}
	value, err := evalValueExpression(node.Value, c.lookup)
	if err != nil {
		return fmt.Errorf("goark-orm: bind %s failed: %w", name, err)
	}
	c.values[name] = value
	c.args[name] = value
	return nil
}

func (c *dynamicRenderContext) renderChildren(nodes []DynamicSQLNode) (string, error) {
	fragments, err := c.renderNodes(nodes)
	if err != nil {
		return "", err
	}
	return joinSQLFragments(fragments), nil
}

func (c *dynamicRenderContext) renderText(text string) string {
	text = normalizeSQLFragment(text)
	if text == "" || len(c.aliases) == 0 || !strings.Contains(text, "#{") {
		return text
	}
	return rewriteDynamicParameterAliases(text, c.aliases)
}

func (c *dynamicRenderContext) renderTrim(node DynamicSQLNode) (string, error) {
	content, err := c.renderChildren(node.Children)
	if err != nil || content == "" {
		return "", err
	}
	content = removePrefixOverride(content, splitOverrides(node.PrefixOverrides))
	content = removeSuffixOverride(content, splitOverrides(node.SuffixOverrides))
	if content == "" {
		return "", nil
	}
	fragments := make([]string, 0, 3)
	if strings.TrimSpace(node.Prefix) != "" {
		fragments = append(fragments, strings.TrimSpace(node.Prefix))
	}
	fragments = append(fragments, content)
	if strings.TrimSpace(node.Suffix) != "" {
		fragments = append(fragments, strings.TrimSpace(node.Suffix))
	}
	return joinSQLFragments(fragments), nil
}

func (c *dynamicRenderContext) renderForeach(node DynamicSQLNode) (string, error) {
	collectionName := strings.TrimSpace(node.Collection)
	if collectionName == "" {
		return "", fmt.Errorf("goark-orm: foreach collection is required")
	}
	collection, ok := c.lookup(collectionName)
	if !ok || isNilValue(collection) {
		if c.foreachNullable(node) {
			return "", nil
		}
		return "", fmt.Errorf("goark-orm: foreach collection %q is nil", collectionName)
	}
	value := reflect.ValueOf(collection)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			if c.foreachNullable(node) {
				return "", nil
			}
			return "", fmt.Errorf("goark-orm: foreach collection %q is nil", collectionName)
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return "", fmt.Errorf("goark-orm: foreach collection %q must be slice or array", collectionName)
	}
	itemName := strings.TrimSpace(node.Item)
	if itemName == "" {
		itemName = "item"
	}
	indexName := strings.TrimSpace(node.Index)
	items := make([]string, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		itemValue := value.Index(i).Interface()
		alias := c.nextAlias(itemName)
		c.args[alias] = itemValue
		previousAlias, hadAlias := c.aliases[itemName]
		previousValue, hadValue := c.values[itemName]
		c.aliases[itemName] = alias
		c.values[itemName] = itemValue
		var indexAlias string
		var hadIndexAlias bool
		var previousIndexAlias string
		if indexName != "" {
			indexAlias = c.nextAlias(indexName)
			c.args[indexAlias] = i
			previousIndexAlias, hadIndexAlias = c.aliases[indexName]
			c.aliases[indexName] = indexAlias
			c.values[indexName] = i
		}
		content, err := c.renderChildren(node.Children)
		if hadAlias {
			c.aliases[itemName] = previousAlias
		} else {
			delete(c.aliases, itemName)
		}
		if hadValue {
			c.values[itemName] = previousValue
		} else {
			delete(c.values, itemName)
		}
		if indexName != "" {
			if hadIndexAlias {
				c.aliases[indexName] = previousIndexAlias
			} else {
				delete(c.aliases, indexName)
			}
			delete(c.values, indexName)
		}
		if err != nil {
			return "", err
		}
		if content != "" {
			items = append(items, content)
		}
	}
	if len(items) == 0 {
		return "", nil
	}
	separator := strings.TrimSpace(node.Separator)
	if separator == "" {
		separator = " "
	} else if !strings.HasSuffix(separator, " ") {
		separator += " "
	}
	content := strings.Join(items, separator)
	return strings.TrimSpace(node.Open + content + node.Close), nil
}

func (c *dynamicRenderContext) foreachNullable(node DynamicSQLNode) bool {
	if node.Nullable != nil {
		return *node.Nullable
	}
	return c.nullableOnForEach
}

func (c *dynamicRenderContext) renderChoose(node DynamicSQLNode) (string, error) {
	var otherwise *DynamicSQLNode
	for index := range node.Children {
		child := node.Children[index]
		switch child.Kind {
		case DynamicSQLNodeWhen:
			ok, err := c.eval(child.Test)
			if err != nil {
				return "", err
			}
			if ok {
				return c.renderChildren(child.Children)
			}
		case DynamicSQLNodeOtherwise:
			otherwise = &child
		default:
			return "", fmt.Errorf("goark-orm: choose only supports when and otherwise")
		}
	}
	if otherwise != nil {
		return c.renderChildren(otherwise.Children)
	}
	return "", nil
}

func (c *dynamicRenderContext) nextAlias(name string) string {
	name = sanitizeIdentifier(name)
	if name == "" {
		name = "item"
	}
	alias := "__goark_orm_" + name + "_" + strconv.Itoa(c.seq)
	c.seq++
	return alias
}

func sanitizeIdentifier(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func splitOverrides(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func removePrefixOverride(sql string, overrides []string) string {
	sql = strings.TrimSpace(sql)
	for _, item := range overrides {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		upperSQL := strings.ToUpper(sql)
		upperItem := strings.ToUpper(item)
		if upperSQL == upperItem {
			return ""
		}
		if strings.HasPrefix(upperSQL, upperItem+" ") {
			return strings.TrimSpace(sql[len(item):])
		}
	}
	return sql
}

func removeSuffixOverride(sql string, overrides []string) string {
	sql = strings.TrimSpace(sql)
	for _, item := range overrides {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		upperSQL := strings.ToUpper(sql)
		upperItem := strings.ToUpper(item)
		if upperSQL == upperItem {
			return ""
		}
		if strings.HasSuffix(upperSQL, " "+upperItem) || strings.HasSuffix(upperSQL, upperItem) {
			return strings.TrimSpace(sql[:len(sql)-len(item)])
		}
	}
	return sql
}

func normalizeSQLFragment(raw string) string {
	if !strings.Contains(raw, "\n") {
		return strings.TrimSpace(raw)
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.Join(normalized, " ")
}

func joinSQLFragments(fragments []string) string {
	count := 0
	size := 0
	only := ""
	for _, fragment := range fragments {
		fragment = strings.TrimSpace(fragment)
		if fragment != "" {
			count++
			size += len(fragment)
			only = fragment
		}
	}
	if count == 0 {
		return ""
	}
	if count == 1 {
		return only
	}
	var builder strings.Builder
	builder.Grow(size + count - 1)
	wrote := false
	for _, fragment := range fragments {
		fragment = strings.TrimSpace(fragment)
		if fragment == "" {
			continue
		}
		if wrote {
			builder.WriteByte(' ')
		}
		builder.WriteString(fragment)
		wrote = true
	}
	return builder.String()
}

func (c *dynamicRenderContext) eval(expression string) (bool, error) {
	return evalExpression(expression, c.lookup)
}

func (c *dynamicRenderContext) lookup(name string) (any, bool) {
	name = strings.TrimSpace(name)
	if value, ok := c.values[name]; ok {
		return value, true
	}
	if value, ok := c.args[name]; ok {
		return value, true
	}
	value, ok, err := resolveNamedArg(c.values, name)
	if err == nil && ok {
		return value, true
	}
	value, ok, err = resolveNamedArg(c.args, name)
	if err == nil && ok {
		return value, true
	}
	return nil, false
}
