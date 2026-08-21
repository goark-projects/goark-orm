package ormgen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"goark.dev/orm"
)

type xmlMapperModel struct {
	Namespace  string
	ResultMaps []xmlResultMapModel
	Selects    []xmlStatementModel
	Inserts    []xmlStatementModel
	Updates    []xmlStatementModel
	Deletes    []xmlStatementModel
	Fragments  map[string][]orm.DynamicSQLNode
}

type xmlResultMapModel struct {
	ID       string                `xml:"id,attr"`
	TypeName string                `xml:"type,attr"`
	IDs      []xmlResultFieldModel `xml:"id"`
	Results  []xmlResultFieldModel `xml:"result"`
}

type xmlResultFieldModel struct {
	Property    string `xml:"property,attr"`
	Column      string `xml:"column,attr"`
	TypeHandler string `xml:"typeHandler,attr"`
}

type xmlStatementModel struct {
	ID               string
	ResultMap        string
	ResultType       string
	ParameterType    string
	UseGeneratedKeys bool
	KeyProperty      string
	SQL              string
	DynamicSQL       []orm.DynamicSQLNode
}

func parseXMLMapper(path string) (xmlMapperModel, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return xmlMapperModel{}, fmt.Errorf("goark-orm: read XML mapper %s failed: %w", path, err)
	}
	mapper, err := decodeXMLMapper(data)
	if err != nil {
		return xmlMapperModel{}, fmt.Errorf("goark-orm: parse XML mapper %s failed: %w", path, err)
	}
	mapper.Namespace = strings.TrimSpace(mapper.Namespace)
	if mapper.Namespace == "" {
		return xmlMapperModel{}, fmt.Errorf("goark-orm: XML mapper %s missing namespace", path)
	}
	if err := expandXMLIncludes(&mapper); err != nil {
		return xmlMapperModel{}, err
	}
	for _, statement := range mapper.allStatements() {
		for _, text := range append([]string{statement.SQL}, dynamicSQLTexts(statement.DynamicSQL)...) {
			if strings.Contains(text, "${") {
				return xmlMapperModel{}, fmt.Errorf("goark-orm: XML mapper %s statement %s uses forbidden ${}", path, statement.ID)
			}
		}
	}
	return mapper, nil
}

func decodeXMLMapper(data []byte) (xmlMapperModel, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return xmlMapperModel{}, fmt.Errorf("missing mapper root")
			}
			return xmlMapperModel{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "mapper" {
			return xmlMapperModel{}, fmt.Errorf("root element must be mapper")
		}
		return parseXMLMapperElement(decoder, start)
	}
}

func parseXMLMapperElement(decoder *xml.Decoder, start xml.StartElement) (xmlMapperModel, error) {
	mapper := xmlMapperModel{
		Namespace: attrValue(start, "namespace"),
		Fragments: make(map[string][]orm.DynamicSQLNode),
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlMapperModel{}, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			switch item.Name.Local {
			case "resultMap":
				resultMap, err := parseXMLResultMap(decoder, item)
				if err != nil {
					return xmlMapperModel{}, err
				}
				mapper.ResultMaps = append(mapper.ResultMaps, resultMap)
			case "select", "insert", "update", "delete":
				statement, err := parseXMLStatement(decoder, item)
				if err != nil {
					return xmlMapperModel{}, err
				}
				switch item.Name.Local {
				case "select":
					mapper.Selects = append(mapper.Selects, statement)
				case "insert":
					mapper.Inserts = append(mapper.Inserts, statement)
				case "update":
					mapper.Updates = append(mapper.Updates, statement)
				case "delete":
					mapper.Deletes = append(mapper.Deletes, statement)
				}
			case "sql":
				id := strings.TrimSpace(attrValue(item, "id"))
				if id == "" {
					return xmlMapperModel{}, fmt.Errorf("sql fragment missing id")
				}
				nodes, err := parseDynamicSQLNodes(decoder, item.Name.Local)
				if err != nil {
					return xmlMapperModel{}, err
				}
				mapper.Fragments[id] = nodes
			default:
				return xmlMapperModel{}, fmt.Errorf("unsupported mapper element <%s>", item.Name.Local)
			}
		case xml.EndElement:
			if item.Name.Local == start.Name.Local {
				return mapper, nil
			}
		}
	}
}

func parseXMLResultMap(decoder *xml.Decoder, start xml.StartElement) (xmlResultMapModel, error) {
	resultMap := xmlResultMapModel{
		ID:       attrValue(start, "id"),
		TypeName: attrValue(start, "type"),
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlResultMapModel{}, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			switch item.Name.Local {
			case "id":
				resultMap.IDs = append(resultMap.IDs, parseXMLResultField(item))
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlResultMapModel{}, err
				}
			case "result":
				resultMap.Results = append(resultMap.Results, parseXMLResultField(item))
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlResultMapModel{}, err
				}
			default:
				return xmlResultMapModel{}, fmt.Errorf("unsupported resultMap element <%s>", item.Name.Local)
			}
		case xml.EndElement:
			if item.Name.Local == start.Name.Local {
				return resultMap, nil
			}
		}
	}
}

func parseXMLResultField(start xml.StartElement) xmlResultFieldModel {
	return xmlResultFieldModel{
		Property:    attrValue(start, "property"),
		Column:      attrValue(start, "column"),
		TypeHandler: attrValue(start, "typeHandler"),
	}
}

func parseXMLStatement(decoder *xml.Decoder, start xml.StartElement) (xmlStatementModel, error) {
	nodes, err := parseDynamicSQLNodes(decoder, start.Name.Local)
	if err != nil {
		return xmlStatementModel{}, err
	}
	statement := xmlStatementModel{
		ID:               attrValue(start, "id"),
		ResultMap:        attrValue(start, "resultMap"),
		ResultType:       attrValue(start, "resultType"),
		ParameterType:    attrValue(start, "parameterType"),
		UseGeneratedKeys: attrValue(start, "useGeneratedKeys") == "true",
		KeyProperty:      attrValue(start, "keyProperty"),
		DynamicSQL:       nodes,
	}
	if isStaticDynamicSQL(nodes) {
		statement.SQL = normalizeXMLSQL(nodes[0].Text)
		statement.DynamicSQL = nil
	}
	return statement, nil
}

func parseDynamicSQLNodes(decoder *xml.Decoder, end string) ([]orm.DynamicSQLNode, error) {
	nodes := make([]orm.DynamicSQLNode, 0)
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch item := token.(type) {
		case xml.CharData:
			text := string(item)
			if strings.TrimSpace(text) != "" {
				nodes = append(nodes, orm.DynamicSQLNode{Kind: orm.DynamicSQLNodeText, Text: text})
			}
		case xml.StartElement:
			node, err := parseDynamicSQLNode(decoder, item)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		case xml.EndElement:
			if item.Name.Local == end {
				return mergeAdjacentTextNodes(nodes), nil
			}
		}
	}
}

func parseDynamicSQLNode(decoder *xml.Decoder, start xml.StartElement) (orm.DynamicSQLNode, error) {
	switch start.Name.Local {
	case "if":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeIf, func(node *orm.DynamicSQLNode) {
			node.Test = attrValue(start, "test")
		})
	case "where":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeWhere, nil)
	case "set":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeSet, nil)
	case "trim":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeTrim, func(node *orm.DynamicSQLNode) {
			node.Prefix = attrValue(start, "prefix")
			node.Suffix = attrValue(start, "suffix")
			node.PrefixOverrides = attrValue(start, "prefixOverrides")
			node.SuffixOverrides = attrValue(start, "suffixOverrides")
		})
	case "foreach":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeForeach, func(node *orm.DynamicSQLNode) {
			node.Collection = attrValue(start, "collection")
			node.Item = attrValue(start, "item")
			node.Index = attrValue(start, "index")
			node.Open = attrValue(start, "open")
			node.Close = attrValue(start, "close")
			node.Separator = attrValue(start, "separator")
		})
	case "choose":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeChoose, nil)
	case "when":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeWhen, func(node *orm.DynamicSQLNode) {
			node.Test = attrValue(start, "test")
		})
	case "otherwise":
		return parseContainerDynamicNode(decoder, start, orm.DynamicSQLNodeOtherwise, nil)
	case "include":
		if err := skipElement(decoder, start.Name.Local); err != nil {
			return orm.DynamicSQLNode{}, err
		}
		return orm.DynamicSQLNode{
			Kind:  orm.DynamicSQLNodeInclude,
			RefID: firstNonEmpty(attrValue(start, "refid"), attrValue(start, "refId")),
		}, nil
	default:
		return orm.DynamicSQLNode{}, fmt.Errorf("unsupported dynamic SQL element <%s>", start.Name.Local)
	}
}

func parseContainerDynamicNode(decoder *xml.Decoder, start xml.StartElement, kind orm.DynamicSQLNodeKind, apply func(*orm.DynamicSQLNode)) (orm.DynamicSQLNode, error) {
	children, err := parseDynamicSQLNodes(decoder, start.Name.Local)
	if err != nil {
		return orm.DynamicSQLNode{}, err
	}
	node := orm.DynamicSQLNode{Kind: kind, Children: children}
	if apply != nil {
		apply(&node)
	}
	return node, nil
}

func skipElement(decoder *xml.Decoder, name string) error {
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch item := token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			if item.Name.Local == name {
				depth--
			}
		}
	}
	return nil
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func isStaticDynamicSQL(nodes []orm.DynamicSQLNode) bool {
	return len(nodes) == 1 && nodes[0].Kind == orm.DynamicSQLNodeText
}

func mergeAdjacentTextNodes(nodes []orm.DynamicSQLNode) []orm.DynamicSQLNode {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]orm.DynamicSQLNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Kind == orm.DynamicSQLNodeText && len(out) > 0 && out[len(out)-1].Kind == orm.DynamicSQLNodeText {
			out[len(out)-1].Text += "\n" + node.Text
			continue
		}
		out = append(out, node)
	}
	return out
}

func expandXMLIncludes(mapper *xmlMapperModel) error {
	var err error
	for index := range mapper.Selects {
		mapper.Selects[index].DynamicSQL, err = expandDynamicIncludes(mapper.Selects[index].DynamicSQL, mapper.Fragments, nil)
		if err != nil {
			return err
		}
	}
	for index := range mapper.Inserts {
		mapper.Inserts[index].DynamicSQL, err = expandDynamicIncludes(mapper.Inserts[index].DynamicSQL, mapper.Fragments, nil)
		if err != nil {
			return err
		}
	}
	for index := range mapper.Updates {
		mapper.Updates[index].DynamicSQL, err = expandDynamicIncludes(mapper.Updates[index].DynamicSQL, mapper.Fragments, nil)
		if err != nil {
			return err
		}
	}
	for index := range mapper.Deletes {
		mapper.Deletes[index].DynamicSQL, err = expandDynamicIncludes(mapper.Deletes[index].DynamicSQL, mapper.Fragments, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func expandDynamicIncludes(nodes []orm.DynamicSQLNode, fragments map[string][]orm.DynamicSQLNode, stack []string) ([]orm.DynamicSQLNode, error) {
	out := make([]orm.DynamicSQLNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Kind == orm.DynamicSQLNodeInclude {
			refID := strings.TrimSpace(node.RefID)
			if refID == "" {
				return nil, fmt.Errorf("include missing refid")
			}
			if containsString(stack, refID) {
				return nil, fmt.Errorf("include cycle detected: %s", strings.Join(append(stack, refID), " -> "))
			}
			fragment, ok := fragments[refID]
			if !ok {
				keys := make([]string, 0, len(fragments))
				for key := range fragments {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				return nil, fmt.Errorf("include refid %q not found; available fragments: %s", refID, strings.Join(keys, ", "))
			}
			expanded, err := expandDynamicIncludes(copyDynamicNodesForGen(fragment), fragments, append(stack, refID))
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
			continue
		}
		if len(node.Children) > 0 {
			children, err := expandDynamicIncludes(node.Children, fragments, stack)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}
		out = append(out, node)
	}
	return out, nil
}

func copyDynamicNodesForGen(nodes []orm.DynamicSQLNode) []orm.DynamicSQLNode {
	out := append([]orm.DynamicSQLNode(nil), nodes...)
	for index := range out {
		out[index].Children = copyDynamicNodesForGen(out[index].Children)
	}
	return out
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func (m xmlMapperModel) allStatements() []xmlStatementModel {
	out := make([]xmlStatementModel, 0, len(m.Selects)+len(m.Inserts)+len(m.Updates)+len(m.Deletes))
	out = append(out, m.Selects...)
	out = append(out, m.Inserts...)
	out = append(out, m.Updates...)
	out = append(out, m.Deletes...)
	return out
}

func xmlResultMaps(mapper xmlMapperModel) ([]orm.ResultMapMeta, error) {
	resultMaps := make([]orm.ResultMapMeta, 0, len(mapper.ResultMaps))
	for _, item := range mapper.ResultMaps {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			return nil, fmt.Errorf("goark-orm: XML resultMap missing id")
		}
		resultMap := orm.ResultMapMeta{
			ID:       id,
			TypeName: strings.TrimSpace(item.TypeName),
		}
		for _, field := range item.IDs {
			resultMap.Fields = append(resultMap.Fields, orm.ResultFieldMeta{
				Property:    strings.TrimSpace(field.Property),
				Column:      strings.TrimSpace(field.Column),
				ID:          true,
				TypeHandler: strings.TrimSpace(field.TypeHandler),
			})
		}
		for _, field := range item.Results {
			resultMap.Fields = append(resultMap.Fields, orm.ResultFieldMeta{
				Property:    strings.TrimSpace(field.Property),
				Column:      strings.TrimSpace(field.Column),
				TypeHandler: strings.TrimSpace(field.TypeHandler),
			})
		}
		resultMaps = append(resultMaps, resultMap)
	}
	return resultMaps, nil
}

func xmlStatements(namespace string, mapper xmlMapperModel) ([]StatementModel, error) {
	statements := make([]StatementModel, 0)
	appendStatements := func(command orm.StatementCommand, source []xmlStatementModel) error {
		for _, item := range source {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				return fmt.Errorf("goark-orm: XML %s statement missing id", command)
			}
			statement := StatementModel{
				ID:               id,
				Namespace:        namespace,
				FullName:         namespace + "." + id,
				Command:          command,
				Source:           orm.StatementSourceXML,
				SQL:              normalizeXMLSQL(item.SQL),
				DynamicSQL:       item.DynamicSQL,
				ResultMap:        strings.TrimSpace(item.ResultMap),
				ResultType:       strings.TrimSpace(item.ResultType),
				ParameterType:    strings.TrimSpace(item.ParameterType),
				UseGeneratedKeys: item.UseGeneratedKeys,
				KeyProperty:      strings.TrimSpace(item.KeyProperty),
			}
			statement.Parameters = statementParameters(statement.SQL)
			statement.Parameters = append(statement.Parameters, dynamicStatementParameters(statement.DynamicSQL)...)
			statement.Parameters = uniqueSorted(statement.Parameters)
			statements = append(statements, statement)
		}
		return nil
	}
	if err := appendStatements(orm.StatementCommandSelect, mapper.Selects); err != nil {
		return nil, err
	}
	if err := appendStatements(orm.StatementCommandInsert, mapper.Inserts); err != nil {
		return nil, err
	}
	if err := appendStatements(orm.StatementCommandUpdate, mapper.Updates); err != nil {
		return nil, err
	}
	if err := appendStatements(orm.StatementCommandDelete, mapper.Deletes); err != nil {
		return nil, err
	}
	return statements, nil
}

func normalizeXMLSQL(raw string) string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.Join(normalized, "\n")
}

func dynamicSQLTexts(nodes []orm.DynamicSQLNode) []string {
	out := make([]string, 0)
	for _, node := range nodes {
		if node.Kind == orm.DynamicSQLNodeText && node.Text != "" {
			out = append(out, node.Text)
		}
		out = append(out, dynamicSQLTexts(node.Children)...)
	}
	return out
}

func dynamicStatementParameters(nodes []orm.DynamicSQLNode) []string {
	out := make([]string, 0)
	collectDynamicParameters(nodes, nil, &out)
	return out
}

func collectDynamicParameters(nodes []orm.DynamicSQLNode, scoped map[string]struct{}, out *[]string) {
	for _, node := range nodes {
		switch node.Kind {
		case orm.DynamicSQLNodeText:
			for _, param := range statementParameters(node.Text) {
				if _, ok := scoped[param]; !ok {
					*out = append(*out, param)
				}
			}
		case orm.DynamicSQLNodeIf, orm.DynamicSQLNodeWhen:
			for _, param := range expressionParameters(node.Test) {
				if _, ok := scoped[param]; !ok {
					*out = append(*out, param)
				}
			}
			collectDynamicParameters(node.Children, scoped, out)
		case orm.DynamicSQLNodeForeach:
			if node.Collection != "" {
				*out = append(*out, node.Collection)
			}
			childScope := make(map[string]struct{}, len(scoped)+2)
			for key := range scoped {
				childScope[key] = struct{}{}
			}
			if node.Item != "" {
				childScope[node.Item] = struct{}{}
			}
			if node.Index != "" {
				childScope[node.Index] = struct{}{}
			}
			collectDynamicParameters(node.Children, childScope, out)
		default:
			collectDynamicParameters(node.Children, scoped, out)
		}
	}
}

func expressionParameters(expression string) []string {
	identifiers := identifiersOutsideQuotes(expression)
	out := make([]string, 0, len(identifiers))
	keywords := map[string]struct{}{
		"and":   {},
		"or":    {},
		"nil":   {},
		"true":  {},
		"false": {},
	}
	for _, identifier := range identifiers {
		if _, ok := keywords[identifier]; ok {
			continue
		}
		out = append(out, identifier)
	}
	return out
}

func identifiersOutsideQuotes(value string) []string {
	out := make([]string, 0)
	inQuote := rune(0)
	for index := 0; index < len(value); {
		r := rune(value[index])
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			index++
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			index++
			continue
		}
		if isIdentifierStart(r) {
			start := index
			index++
			for index < len(value) && isIdentifierPart(rune(value[index])) {
				index++
			}
			out = append(out, value[start:index])
			continue
		}
		index++
	}
	return out
}

func isIdentifierStart(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_'
}

func isIdentifierPart(r rune) bool {
	return isIdentifierStart(r) || (r >= '0' && r <= '9')
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
