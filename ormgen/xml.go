package ormgen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"goark.dev/orm"
)

type xmlMapperModel struct {
	Namespace  string
	Cache      orm.CacheMeta
	ResultMaps []xmlResultMapModel
	Selects    []xmlStatementModel
	Inserts    []xmlStatementModel
	Updates    []xmlStatementModel
	Deletes    []xmlStatementModel
	Calls      []xmlStatementModel
	Fragments  map[string][]orm.DynamicSQLNode
}

type xmlResultMapModel struct {
	ID            string
	TypeName      string
	Extends       string
	AutoMapping   *bool
	Constructor   xmlResultConstructorModel
	IDs           []xmlResultFieldModel
	Results       []xmlResultFieldModel
	Associations  []xmlResultObjectModel
	Collections   []xmlResultObjectModel
	Discriminator xmlDiscriminatorModel
}

type xmlResultConstructorModel struct {
	Args []xmlResultArgModel
}

type xmlResultArgModel struct {
	Name        string
	Property    string
	Column      string
	TypeHandler string
	ID          bool
}

type xmlResultFieldModel struct {
	Property    string `xml:"property,attr"`
	Column      string `xml:"column,attr"`
	TypeHandler string `xml:"typeHandler,attr"`
}

type xmlResultObjectModel struct {
	Property       string
	TypeName       string
	Column         string
	ColumnPrefix   string
	NotNullColumns []string
	Select         string
	FetchType      string
	IDs            []xmlResultFieldModel
	Results        []xmlResultFieldModel
	Associations   []xmlResultObjectModel
	Collections    []xmlResultObjectModel
}

type xmlDiscriminatorModel struct {
	Column      string
	TypeName    string
	TypeHandler string
	Cases       []xmlDiscriminatorCaseModel
}

type xmlDiscriminatorCaseModel struct {
	Value        string
	ResultMap    string
	ResultType   string
	IDs          []xmlResultFieldModel
	Results      []xmlResultFieldModel
	Associations []xmlResultObjectModel
	Collections  []xmlResultObjectModel
}

type xmlStatementModel struct {
	ID                 string
	ResultMap          string
	ResultType         string
	ParameterType      string
	DatabaseID         string
	AffectData         bool
	UseGeneratedKeys   bool
	KeyProperty        string
	Options            orm.StatementOptions
	SelectKey          orm.SelectKeyMeta
	UseCache           orm.StatementCachePolicy
	FlushCache         orm.StatementCachePolicy
	StatementType      orm.StatementType
	Parameters         []orm.ParameterMeta
	ResultSets         []orm.ResultSetMeta
	SQL                string
	DynamicSQL         []orm.DynamicSQLNode
	InterceptorIgnores []string
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
			case "cache":
				if mapper.Cache.Enabled {
					return xmlMapperModel{}, fmt.Errorf("goark-orm: XML mapper %s declares multiple cache elements", mapper.Namespace)
				}
				cache, err := parseXMLCache(item)
				if err != nil {
					return xmlMapperModel{}, err
				}
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlMapperModel{}, err
				}
				mapper.Cache = cache
			case "cache-ref":
				if mapper.Cache.Enabled {
					return xmlMapperModel{}, fmt.Errorf("goark-orm: XML mapper %s declares multiple cache elements", mapper.Namespace)
				}
				cache, err := parseXMLCacheRef(item)
				if err != nil {
					return xmlMapperModel{}, err
				}
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlMapperModel{}, err
				}
				mapper.Cache = cache
			case "resultMap":
				resultMap, err := parseXMLResultMap(decoder, item)
				if err != nil {
					return xmlMapperModel{}, err
				}
				mapper.ResultMaps = append(mapper.ResultMaps, resultMap)
			case "select", "insert", "update", "delete", "call":
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
				case "call":
					mapper.Calls = append(mapper.Calls, statement)
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

func parseXMLCache(start xml.StartElement) (orm.CacheMeta, error) {
	size, err := parseOptionalXMLInt(start, "size")
	if err != nil {
		return orm.CacheMeta{}, err
	}
	flushInterval, err := parseOptionalXMLInt64(start, "flushInterval")
	if err != nil {
		return orm.CacheMeta{}, err
	}
	readOnly, _, err := parseOptionalXMLBool(start, "readOnly")
	if err != nil {
		return orm.CacheMeta{}, err
	}
	blocking, _, err := parseOptionalXMLBool(start, "blocking")
	if err != nil {
		return orm.CacheMeta{}, err
	}
	return orm.CacheMeta{
		Enabled:             true,
		Eviction:            attrValue(start, "eviction"),
		Size:                size,
		FlushIntervalMillis: flushInterval,
		ReadOnly:            readOnly,
		Blocking:            blocking,
	}, nil
}

func parseXMLCacheRef(start xml.StartElement) (orm.CacheMeta, error) {
	namespace := strings.TrimSpace(attrValue(start, "namespace"))
	if namespace == "" {
		return orm.CacheMeta{}, fmt.Errorf("goark-orm: cache-ref missing namespace")
	}
	return orm.CacheMeta{
		Enabled:      true,
		RefNamespace: namespace,
	}, nil
}

func parseXMLResultMap(decoder *xml.Decoder, start xml.StartElement) (xmlResultMapModel, error) {
	autoMapping, hasAutoMapping, err := parseOptionalXMLBool(start, "autoMapping")
	if err != nil {
		return xmlResultMapModel{}, err
	}
	resultMap := xmlResultMapModel{
		ID:       attrValue(start, "id"),
		TypeName: attrValue(start, "type"),
		Extends:  attrValue(start, "extends"),
	}
	if hasAutoMapping {
		resultMap.AutoMapping = &autoMapping
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlResultMapModel{}, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			switch item.Name.Local {
			case "constructor":
				constructor, err := parseXMLResultConstructor(decoder, item)
				if err != nil {
					return xmlResultMapModel{}, err
				}
				resultMap.Constructor = constructor
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
			case "association":
				association, err := parseXMLResultObject(decoder, item, false)
				if err != nil {
					return xmlResultMapModel{}, err
				}
				resultMap.Associations = append(resultMap.Associations, association)
			case "collection":
				collection, err := parseXMLResultObject(decoder, item, true)
				if err != nil {
					return xmlResultMapModel{}, err
				}
				resultMap.Collections = append(resultMap.Collections, collection)
			case "discriminator":
				discriminator, err := parseXMLDiscriminator(decoder, item)
				if err != nil {
					return xmlResultMapModel{}, err
				}
				resultMap.Discriminator = discriminator
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

func parseXMLResultConstructor(decoder *xml.Decoder, start xml.StartElement) (xmlResultConstructorModel, error) {
	constructor := xmlResultConstructorModel{}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlResultConstructorModel{}, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			switch item.Name.Local {
			case "idArg":
				constructor.Args = append(constructor.Args, parseXMLResultArg(item, true))
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlResultConstructorModel{}, err
				}
			case "arg":
				constructor.Args = append(constructor.Args, parseXMLResultArg(item, false))
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlResultConstructorModel{}, err
				}
			default:
				return xmlResultConstructorModel{}, fmt.Errorf("unsupported constructor element <%s>", item.Name.Local)
			}
		case xml.EndElement:
			if item.Name.Local == start.Name.Local {
				return constructor, nil
			}
		}
	}
}

func parseXMLDiscriminator(decoder *xml.Decoder, start xml.StartElement) (xmlDiscriminatorModel, error) {
	discriminator := xmlDiscriminatorModel{
		Column:      attrValue(start, "column"),
		TypeName:    firstNonEmpty(attrValue(start, "javaType"), attrValue(start, "type")),
		TypeHandler: attrValue(start, "typeHandler"),
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlDiscriminatorModel{}, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			switch item.Name.Local {
			case "case":
				item, err := parseXMLDiscriminatorCase(decoder, item)
				if err != nil {
					return xmlDiscriminatorModel{}, err
				}
				discriminator.Cases = append(discriminator.Cases, item)
			default:
				return xmlDiscriminatorModel{}, fmt.Errorf("unsupported discriminator element <%s>", item.Name.Local)
			}
		case xml.EndElement:
			if item.Name.Local == start.Name.Local {
				return discriminator, nil
			}
		}
	}
}

func parseXMLDiscriminatorCase(decoder *xml.Decoder, start xml.StartElement) (xmlDiscriminatorCaseModel, error) {
	item := xmlDiscriminatorCaseModel{
		Value:      attrValue(start, "value"),
		ResultMap:  attrValue(start, "resultMap"),
		ResultType: firstNonEmpty(attrValue(start, "resultType"), attrValue(start, "type")),
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlDiscriminatorCaseModel{}, err
		}
		switch child := token.(type) {
		case xml.StartElement:
			switch child.Name.Local {
			case "id":
				item.IDs = append(item.IDs, parseXMLResultField(child))
				if err := skipElement(decoder, child.Name.Local); err != nil {
					return xmlDiscriminatorCaseModel{}, err
				}
			case "result":
				item.Results = append(item.Results, parseXMLResultField(child))
				if err := skipElement(decoder, child.Name.Local); err != nil {
					return xmlDiscriminatorCaseModel{}, err
				}
			case "association":
				association, err := parseXMLResultObject(decoder, child, false)
				if err != nil {
					return xmlDiscriminatorCaseModel{}, err
				}
				item.Associations = append(item.Associations, association)
			case "collection":
				collection, err := parseXMLResultObject(decoder, child, true)
				if err != nil {
					return xmlDiscriminatorCaseModel{}, err
				}
				item.Collections = append(item.Collections, collection)
			default:
				return xmlDiscriminatorCaseModel{}, fmt.Errorf("unsupported discriminator case element <%s>", child.Name.Local)
			}
		case xml.EndElement:
			if child.Name.Local == start.Name.Local {
				return item, nil
			}
		}
	}
}

func parseXMLResultObject(decoder *xml.Decoder, start xml.StartElement, collection bool) (xmlResultObjectModel, error) {
	typeName := firstNonEmpty(attrValue(start, "type"), attrValue(start, "javaType"))
	if collection {
		typeName = firstNonEmpty(attrValue(start, "ofType"), typeName)
	}
	object := xmlResultObjectModel{
		Property:       attrValue(start, "property"),
		TypeName:       typeName,
		Column:         attrValue(start, "column"),
		ColumnPrefix:   attrValue(start, "columnPrefix"),
		NotNullColumns: splitXMLColumnList(attrValue(start, "notNullColumn")),
		Select:         attrValue(start, "select"),
		FetchType:      attrValue(start, "fetchType"),
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlResultObjectModel{}, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			switch item.Name.Local {
			case "id":
				object.IDs = append(object.IDs, parseXMLResultField(item))
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlResultObjectModel{}, err
				}
			case "result":
				object.Results = append(object.Results, parseXMLResultField(item))
				if err := skipElement(decoder, item.Name.Local); err != nil {
					return xmlResultObjectModel{}, err
				}
			case "association":
				association, err := parseXMLResultObject(decoder, item, false)
				if err != nil {
					return xmlResultObjectModel{}, err
				}
				object.Associations = append(object.Associations, association)
			case "collection":
				child, err := parseXMLResultObject(decoder, item, true)
				if err != nil {
					return xmlResultObjectModel{}, err
				}
				object.Collections = append(object.Collections, child)
			default:
				return xmlResultObjectModel{}, fmt.Errorf("unsupported resultMap nested element <%s>", item.Name.Local)
			}
		case xml.EndElement:
			if item.Name.Local == start.Name.Local {
				return object, nil
			}
		}
	}
}

func parseXMLResultArg(start xml.StartElement, id bool) xmlResultArgModel {
	return xmlResultArgModel{
		Name:        attrValue(start, "name"),
		Property:    attrValue(start, "property"),
		Column:      attrValue(start, "column"),
		TypeHandler: attrValue(start, "typeHandler"),
		ID:          id,
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
	body, err := parseXMLStatementBody(decoder, start.Name.Local)
	if err != nil {
		return xmlStatementModel{}, err
	}
	useCache, err := parseXMLStatementCachePolicy(start, "useCache")
	if err != nil {
		return xmlStatementModel{}, err
	}
	flushCache, err := parseXMLStatementCachePolicy(start, "flushCache")
	if err != nil {
		return xmlStatementModel{}, err
	}
	affectData, _, err := parseOptionalXMLBool(start, "affectData")
	if err != nil {
		return xmlStatementModel{}, err
	}
	options, err := parseXMLStatementOptions(start)
	if err != nil {
		return xmlStatementModel{}, err
	}
	statement := xmlStatementModel{
		ID:                 attrValue(start, "id"),
		ResultMap:          attrValue(start, "resultMap"),
		ResultType:         attrValue(start, "resultType"),
		ParameterType:      attrValue(start, "parameterType"),
		DatabaseID:         attrValue(start, "databaseId"),
		AffectData:         affectData,
		UseGeneratedKeys:   attrValue(start, "useGeneratedKeys") == "true",
		KeyProperty:        attrValue(start, "keyProperty"),
		Options:            options,
		SelectKey:          body.selectKey,
		UseCache:           useCache,
		FlushCache:         flushCache,
		StatementType:      parseXMLStatementType(start, start.Name.Local),
		Parameters:         body.parameters,
		ResultSets:         body.resultSets,
		DynamicSQL:         body.nodes,
		InterceptorIgnores: parseInterceptorIgnores(attrValue(start, "interceptorIgnore")),
	}
	if isStaticDynamicSQL(body.nodes) {
		statement.SQL = normalizeXMLSQL(body.nodes[0].Text)
		statement.DynamicSQL = nil
	}
	return statement, nil
}

func parseAnnotationSQL(raw string) (string, []orm.DynamicSQLNode, error) {
	sql := strings.TrimSpace(raw)
	if sql == "" || !strings.HasPrefix(strings.ToLower(sql), "<script") {
		return sql, nil, nil
	}
	decoder := xml.NewDecoder(strings.NewReader(sql))
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return "", nil, fmt.Errorf("goark-orm: annotation SQL <script> root is missing")
			}
			return "", nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "script" {
			return "", nil, fmt.Errorf("goark-orm: annotation SQL root must be <script>")
		}
		nodes, err := parseDynamicSQLNodes(decoder, start.Name.Local)
		if err != nil {
			return "", nil, err
		}
		if isStaticDynamicSQL(nodes) {
			return normalizeXMLSQL(nodes[0].Text), nil, nil
		}
		return "", nodes, nil
	}
}

type xmlStatementBody struct {
	nodes      []orm.DynamicSQLNode
	selectKey  orm.SelectKeyMeta
	parameters []orm.ParameterMeta
	resultSets []orm.ResultSetMeta
}

func parseXMLStatementBody(decoder *xml.Decoder, end string) (xmlStatementBody, error) {
	body := xmlStatementBody{nodes: make([]orm.DynamicSQLNode, 0)}
	for {
		token, err := decoder.Token()
		if err != nil {
			return xmlStatementBody{}, err
		}
		switch item := token.(type) {
		case xml.CharData:
			text := string(item)
			if strings.TrimSpace(text) != "" {
				body.nodes = append(body.nodes, orm.DynamicSQLNode{Kind: orm.DynamicSQLNodeText, Text: text})
			}
		case xml.StartElement:
			if item.Name.Local == "selectKey" {
				if body.selectKey.Enabled {
					return xmlStatementBody{}, fmt.Errorf("goark-orm: XML <%s> declares multiple selectKey elements", end)
				}
				parsed, err := parseXMLSelectKey(decoder, item)
				if err != nil {
					return xmlStatementBody{}, err
				}
				body.selectKey = parsed
				continue
			}
			if item.Name.Local == "parameter" {
				parameter, err := parseXMLCallableParameter(decoder, item)
				if err != nil {
					return xmlStatementBody{}, err
				}
				body.parameters = append(body.parameters, parameter)
				continue
			}
			if item.Name.Local == "resultSet" {
				resultSet, err := parseXMLCallableResultSet(decoder, item)
				if err != nil {
					return xmlStatementBody{}, err
				}
				body.resultSets = append(body.resultSets, resultSet)
				continue
			}
			node, err := parseDynamicSQLNode(decoder, item)
			if err != nil {
				return xmlStatementBody{}, err
			}
			body.nodes = append(body.nodes, node)
		case xml.EndElement:
			if item.Name.Local == end {
				body.nodes = mergeAdjacentTextNodes(body.nodes)
				return body, nil
			}
		}
	}
}

func parseXMLStatementType(start xml.StartElement, element string) orm.StatementType {
	value := strings.ToUpper(strings.TrimSpace(attrValue(start, "statementType")))
	switch value {
	case string(orm.StatementTypePrepared):
		return orm.StatementTypePrepared
	case string(orm.StatementTypeCallable):
		return orm.StatementTypeCallable
	default:
		if element == "call" {
			return orm.StatementTypeCallable
		}
		return ""
	}
}

func parseXMLCallableParameter(decoder *xml.Decoder, start xml.StartElement) (orm.ParameterMeta, error) {
	if err := skipElement(decoder, start.Name.Local); err != nil {
		return orm.ParameterMeta{}, err
	}
	name := firstNonEmpty(attrValue(start, "property"), attrValue(start, "name"))
	if strings.TrimSpace(name) == "" {
		return orm.ParameterMeta{}, fmt.Errorf("goark-orm: XML <parameter> missing property")
	}
	mode, err := orm.ParseParameterMode(attrValue(start, "mode"))
	if err != nil {
		return orm.ParameterMeta{}, err
	}
	return orm.ParameterMeta{
		Name:        strings.TrimSpace(name),
		Mode:        mode,
		JDBCType:    firstNonEmpty(attrValue(start, "jdbcType"), attrValue(start, "type")),
		TypeHandler: attrValue(start, "typeHandler"),
	}, nil
}

func parseXMLCallableResultSet(decoder *xml.Decoder, start xml.StartElement) (orm.ResultSetMeta, error) {
	if err := skipElement(decoder, start.Name.Local); err != nil {
		return orm.ResultSetMeta{}, err
	}
	name := firstNonEmpty(attrValue(start, "name"), attrValue(start, "property"))
	if strings.TrimSpace(name) == "" {
		return orm.ResultSetMeta{}, fmt.Errorf("goark-orm: XML <resultSet> missing name")
	}
	return orm.ResultSetMeta{
		Name:       strings.TrimSpace(name),
		ResultMap:  attrValue(start, "resultMap"),
		ResultType: attrValue(start, "resultType"),
	}, nil
}

func parseXMLSelectKey(decoder *xml.Decoder, start xml.StartElement) (orm.SelectKeyMeta, error) {
	nodes, err := parseDynamicSQLNodes(decoder, start.Name.Local)
	if err != nil {
		return orm.SelectKeyMeta{}, err
	}
	order, err := parseXMLSelectKeyOrder(start)
	if err != nil {
		return orm.SelectKeyMeta{}, err
	}
	selectKey := orm.SelectKeyMeta{
		Enabled:     true,
		KeyProperty: attrValue(start, "keyProperty"),
		ResultType:  attrValue(start, "resultType"),
		Order:       order,
		DynamicSQL:  nodes,
	}
	if isStaticDynamicSQL(nodes) {
		selectKey.SQL = normalizeXMLSQL(nodes[0].Text)
		selectKey.DynamicSQL = nil
	}
	return selectKey, nil
}

func parseXMLSelectKeyOrder(start xml.StartElement) (orm.SelectKeyOrder, error) {
	value := strings.ToUpper(strings.TrimSpace(attrValue(start, "order")))
	switch value {
	case "", string(orm.SelectKeyOrderAfter):
		return orm.SelectKeyOrderAfter, nil
	case string(orm.SelectKeyOrderBefore):
		return orm.SelectKeyOrderBefore, nil
	default:
		return "", fmt.Errorf("goark-orm: XML <selectKey> attribute order requires BEFORE or AFTER")
	}
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
	case "bind":
		if err := skipElement(decoder, start.Name.Local); err != nil {
			return orm.DynamicSQLNode{}, err
		}
		return orm.DynamicSQLNode{
			Kind:  orm.DynamicSQLNodeBind,
			Name:  attrValue(start, "name"),
			Value: attrValue(start, "value"),
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

func parseOptionalXMLInt(start xml.StartElement, name string) (int, error) {
	value := attrValue(start, name)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("goark-orm: XML <%s> attribute %s requires integer value", start.Name.Local, name)
	}
	return parsed, nil
}

func parseOptionalXMLInt64(start xml.StartElement, name string) (int64, error) {
	value := attrValue(start, name)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("goark-orm: XML <%s> attribute %s requires integer value", start.Name.Local, name)
	}
	return parsed, nil
}

func parseOptionalXMLBool(start xml.StartElement, name string) (bool, bool, error) {
	value := attrValue(start, name)
	if value == "" {
		return false, false, nil
	}
	switch value {
	case "true":
		return true, true, nil
	case "false":
		return false, true, nil
	default:
		return false, true, fmt.Errorf("goark-orm: XML <%s> attribute %s requires boolean value", start.Name.Local, name)
	}
}

func parseXMLStatementCachePolicy(start xml.StartElement, name string) (orm.StatementCachePolicy, error) {
	value, ok, err := parseOptionalXMLBool(start, name)
	if err != nil || !ok {
		return orm.StatementCacheDefault, err
	}
	if value {
		return orm.StatementCacheEnabled, nil
	}
	return orm.StatementCacheDisabled, nil
}

func parseXMLStatementOptions(start xml.StartElement) (orm.StatementOptions, error) {
	timeout, err := parseXMLStatementTimeout(start)
	if err != nil {
		return orm.StatementOptions{}, err
	}
	fetchSize, err := parseOptionalXMLInt(start, "fetchSize")
	if err != nil {
		return orm.StatementOptions{}, err
	}
	if fetchSize < 0 {
		return orm.StatementOptions{}, fmt.Errorf("goark-orm: XML <%s> attribute fetchSize must be >= 0", start.Name.Local)
	}
	resultSetType, err := orm.ParseResultSetType(attrValue(start, "resultSetType"))
	if err != nil {
		return orm.StatementOptions{}, err
	}
	resultOrdered, _, err := parseOptionalXMLBool(start, "resultOrdered")
	if err != nil {
		return orm.StatementOptions{}, err
	}
	return orm.StatementOptions{
		Timeout:       timeout,
		FetchSize:     fetchSize,
		ResultSetType: resultSetType,
		ResultOrdered: resultOrdered,
		KeyColumn:     attrValue(start, "keyColumn"),
	}, nil
}

func parseXMLStatementTimeout(start xml.StartElement) (time.Duration, error) {
	value := firstNonEmpty(attrValue(start, "timeoutDuration"), attrValue(start, "timeout"))
	if value == "" {
		return 0, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		if duration < 0 {
			return 0, fmt.Errorf("goark-orm: XML <%s> attribute timeout must be >= 0", start.Name.Local)
		}
		return duration, nil
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("goark-orm: XML <%s> attribute timeout requires duration or integer seconds", start.Name.Local)
	}
	if seconds < 0 {
		return 0, fmt.Errorf("goark-orm: XML <%s> attribute timeout must be >= 0", start.Name.Local)
	}
	return time.Duration(seconds) * time.Second, nil
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
		mapper.Inserts[index].SelectKey.DynamicSQL, err = expandDynamicIncludes(mapper.Inserts[index].SelectKey.DynamicSQL, mapper.Fragments, nil)
		if err != nil {
			return err
		}
	}
	for index := range mapper.Updates {
		mapper.Updates[index].DynamicSQL, err = expandDynamicIncludes(mapper.Updates[index].DynamicSQL, mapper.Fragments, nil)
		if err != nil {
			return err
		}
		mapper.Updates[index].SelectKey.DynamicSQL, err = expandDynamicIncludes(mapper.Updates[index].SelectKey.DynamicSQL, mapper.Fragments, nil)
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
	for index := range mapper.Calls {
		mapper.Calls[index].DynamicSQL, err = expandDynamicIncludes(mapper.Calls[index].DynamicSQL, mapper.Fragments, nil)
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
	out := make([]xmlStatementModel, 0, len(m.Selects)+len(m.Inserts)+len(m.Updates)+len(m.Deletes)+len(m.Calls))
	out = append(out, m.Selects...)
	out = append(out, m.Inserts...)
	out = append(out, m.Updates...)
	out = append(out, m.Deletes...)
	out = append(out, m.Calls...)
	return out
}

func xmlResultMaps(mapper xmlMapperModel) ([]orm.ResultMapMeta, error) {
	resultMaps := make([]orm.ResultMapMeta, 0, len(mapper.ResultMaps))
	models := make(map[string]xmlResultMapModel, len(mapper.ResultMaps))
	for _, item := range mapper.ResultMaps {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			return nil, fmt.Errorf("goark-orm: XML resultMap missing id")
		}
		if _, exists := models[id]; exists {
			return nil, fmt.Errorf("goark-orm: duplicate XML resultMap %q", id)
		}
		item.ID = id
		models[id] = item
	}
	resolved := make(map[string]orm.ResultMapMeta, len(models))
	for _, item := range mapper.ResultMaps {
		resultMap, err := resolveXMLResultMap(mapper.Namespace, item.ID, models, resolved, nil)
		if err != nil {
			return nil, err
		}
		resultMaps = append(resultMaps, resultMap)
	}
	return resultMaps, nil
}

func resolveXMLResultMap(namespace string, id string, models map[string]xmlResultMapModel, resolved map[string]orm.ResultMapMeta, stack []string) (orm.ResultMapMeta, error) {
	id = localXMLResultMapID(namespace, id)
	if resultMap, ok := resolved[id]; ok {
		return resultMap, nil
	}
	if containsString(stack, id) {
		return orm.ResultMapMeta{}, fmt.Errorf("goark-orm: resultMap extends cycle detected: %s", strings.Join(append(stack, id), " -> "))
	}
	item, ok := models[id]
	if !ok {
		return orm.ResultMapMeta{}, fmt.Errorf("goark-orm: resultMap %q not found", id)
	}
	resultMap := xmlResultMapMeta(item)
	if parentID := strings.TrimSpace(item.Extends); parentID != "" {
		parent, err := resolveXMLResultMap(namespace, parentID, models, resolved, append(stack, id))
		if err != nil {
			return orm.ResultMapMeta{}, err
		}
		resultMap = mergeResultMaps(parent, resultMap)
	}
	resolved[id] = resultMap
	return resultMap, nil
}

func xmlResultMapMeta(item xmlResultMapModel) orm.ResultMapMeta {
	resultMap := orm.ResultMapMeta{
		ID:          strings.TrimSpace(item.ID),
		TypeName:    strings.TrimSpace(item.TypeName),
		Extends:     strings.TrimSpace(item.Extends),
		AutoMapping: item.AutoMapping,
	}
	resultMap.Constructor = xmlResultConstructor(item.Constructor)
	resultMap.Fields = xmlResultFieldMetas(item.IDs, true)
	resultMap.Fields = append(resultMap.Fields, xmlResultFieldMetas(item.Results, false)...)
	for _, association := range item.Associations {
		resultMap.Associations = append(resultMap.Associations, xmlResultAssociation(association))
	}
	for _, collection := range item.Collections {
		resultMap.Collections = append(resultMap.Collections, xmlResultCollection(collection))
	}
	resultMap.Discriminator = xmlResultDiscriminator(item.Discriminator)
	return resultMap
}

func mergeResultMaps(parent orm.ResultMapMeta, child orm.ResultMapMeta) orm.ResultMapMeta {
	out := child
	if out.TypeName == "" {
		out.TypeName = parent.TypeName
	}
	if out.AutoMapping == nil {
		out.AutoMapping = parent.AutoMapping
	}
	out.Constructor.Args = append(append([]orm.ResultArgMeta(nil), parent.Constructor.Args...), child.Constructor.Args...)
	out.Fields = append(append([]orm.ResultFieldMeta(nil), parent.Fields...), child.Fields...)
	out.Associations = append(append([]orm.ResultAssociationMeta(nil), parent.Associations...), child.Associations...)
	out.Collections = append(append([]orm.ResultCollectionMeta(nil), parent.Collections...), child.Collections...)
	if len(out.Discriminator.Cases) == 0 && out.Discriminator.Column == "" {
		out.Discriminator = parent.Discriminator
	}
	return out
}

func localXMLResultMapID(namespace string, id string) string {
	id = strings.TrimSpace(id)
	namespace = strings.TrimSpace(namespace)
	prefix := namespace + "."
	if namespace != "" && strings.HasPrefix(id, prefix) {
		return strings.TrimPrefix(id, prefix)
	}
	return id
}

func xmlResultAssociation(item xmlResultObjectModel) orm.ResultAssociationMeta {
	out := orm.ResultAssociationMeta{
		Property:       strings.TrimSpace(item.Property),
		TypeName:       strings.TrimSpace(item.TypeName),
		Column:         strings.TrimSpace(item.Column),
		ColumnPrefix:   strings.TrimSpace(item.ColumnPrefix),
		NotNullColumns: trimmedStrings(item.NotNullColumns),
		Select:         strings.TrimSpace(item.Select),
		FetchType:      strings.TrimSpace(item.FetchType),
	}
	out.Fields = xmlResultFields(item)
	for _, association := range item.Associations {
		out.Associations = append(out.Associations, xmlResultAssociation(association))
	}
	for _, collection := range item.Collections {
		out.Collections = append(out.Collections, xmlResultCollection(collection))
	}
	return out
}

func xmlResultCollection(item xmlResultObjectModel) orm.ResultCollectionMeta {
	out := orm.ResultCollectionMeta{
		Property:       strings.TrimSpace(item.Property),
		TypeName:       strings.TrimSpace(item.TypeName),
		Column:         strings.TrimSpace(item.Column),
		ColumnPrefix:   strings.TrimSpace(item.ColumnPrefix),
		NotNullColumns: trimmedStrings(item.NotNullColumns),
		Select:         strings.TrimSpace(item.Select),
		FetchType:      strings.TrimSpace(item.FetchType),
	}
	out.Fields = xmlResultFields(item)
	for _, association := range item.Associations {
		out.Associations = append(out.Associations, xmlResultAssociation(association))
	}
	for _, collection := range item.Collections {
		out.Collections = append(out.Collections, xmlResultCollection(collection))
	}
	return out
}

func xmlResultFields(item xmlResultObjectModel) []orm.ResultFieldMeta {
	fields := xmlResultFieldMetas(item.IDs, true)
	fields = append(fields, xmlResultFieldMetas(item.Results, false)...)
	return fields
}

func xmlResultConstructor(item xmlResultConstructorModel) orm.ResultConstructorMeta {
	return orm.ResultConstructorMeta{Args: xmlResultArgMetas(item.Args)}
}

func xmlResultArgMetas(items []xmlResultArgModel) []orm.ResultArgMeta {
	args := make([]orm.ResultArgMeta, 0, len(items))
	for _, arg := range items {
		args = append(args, orm.ResultArgMeta{
			Name:        strings.TrimSpace(arg.Name),
			Property:    strings.TrimSpace(arg.Property),
			Column:      strings.TrimSpace(arg.Column),
			ID:          arg.ID,
			TypeHandler: strings.TrimSpace(arg.TypeHandler),
		})
	}
	return args
}

func xmlResultFieldMetas(items []xmlResultFieldModel, id bool) []orm.ResultFieldMeta {
	fields := make([]orm.ResultFieldMeta, 0, len(items))
	for _, field := range items {
		fields = append(fields, orm.ResultFieldMeta{
			Property:    strings.TrimSpace(field.Property),
			Column:      strings.TrimSpace(field.Column),
			ID:          id,
			TypeHandler: strings.TrimSpace(field.TypeHandler),
		})
	}
	return fields
}

func splitXMLColumnList(value string) []string {
	parts := strings.Split(value, ",")
	return trimmedStrings(parts)
}

func trimmedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func callableParameterNames(parameters []orm.ParameterMeta) []string {
	out := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		name := strings.TrimSpace(parameter.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func xmlResultDiscriminator(item xmlDiscriminatorModel) orm.ResultDiscriminatorMeta {
	out := orm.ResultDiscriminatorMeta{
		Column:      strings.TrimSpace(item.Column),
		TypeName:    strings.TrimSpace(item.TypeName),
		TypeHandler: strings.TrimSpace(item.TypeHandler),
	}
	for _, item := range item.Cases {
		out.Cases = append(out.Cases, xmlResultDiscriminatorCase(item))
	}
	return out
}

func xmlResultDiscriminatorCase(item xmlDiscriminatorCaseModel) orm.ResultDiscriminatorCaseMeta {
	out := orm.ResultDiscriminatorCaseMeta{
		Value:      strings.TrimSpace(item.Value),
		ResultMap:  strings.TrimSpace(item.ResultMap),
		ResultType: strings.TrimSpace(item.ResultType),
	}
	out.Fields = xmlResultFieldMetas(item.IDs, true)
	out.Fields = append(out.Fields, xmlResultFieldMetas(item.Results, false)...)
	for _, association := range item.Associations {
		out.Associations = append(out.Associations, xmlResultAssociation(association))
	}
	for _, collection := range item.Collections {
		out.Collections = append(out.Collections, xmlResultCollection(collection))
	}
	return out
}

func xmlStatements(namespace string, mapper xmlMapperModel, databaseID string) ([]StatementModel, error) {
	databaseID = strings.TrimSpace(databaseID)
	statements := make([]StatementModel, 0)
	appendStatements := func(command orm.StatementCommand, source []xmlStatementModel) error {
		selected, err := selectXMLStatementsForDatabase(command, source, databaseID)
		if err != nil {
			return err
		}
		for _, item := range selected {
			id := strings.TrimSpace(item.ID)
			if strings.TrimSpace(item.ResultMap) != "" && strings.TrimSpace(item.ResultType) != "" {
				return fmt.Errorf("goark-orm: XML statement %s declares both resultMap and resultType", id)
			}
			statement := StatementModel{
				ID:                 id,
				Namespace:          namespace,
				FullName:           namespace + "." + id,
				Command:            command,
				StatementType:      item.StatementType,
				Source:             orm.StatementSourceXML,
				SQL:                normalizeXMLSQL(item.SQL),
				DynamicSQL:         item.DynamicSQL,
				ResultMap:          strings.TrimSpace(item.ResultMap),
				ResultType:         strings.TrimSpace(item.ResultType),
				ParameterType:      strings.TrimSpace(item.ParameterType),
				DatabaseID:         strings.TrimSpace(item.DatabaseID),
				AffectData:         item.AffectData,
				UseGeneratedKeys:   item.UseGeneratedKeys,
				KeyProperty:        strings.TrimSpace(item.KeyProperty),
				Options:            item.Options,
				SelectKey:          item.SelectKey,
				UseCache:           item.UseCache,
				FlushCache:         item.FlushCache,
				ParameterModes:     item.Parameters,
				ResultSets:         item.ResultSets,
				InterceptorIgnores: item.InterceptorIgnores,
			}
			statement.SelectKey.KeyProperty = strings.TrimSpace(statement.SelectKey.KeyProperty)
			statement.SelectKey.ResultType = strings.TrimSpace(statement.SelectKey.ResultType)
			statement.SelectKey.SQL = normalizeXMLSQL(statement.SelectKey.SQL)
			statement.Parameters = statementParameters(statement.SQL)
			statement.Parameters = append(statement.Parameters, dynamicStatementParameters(statement.DynamicSQL)...)
			statement.Parameters = append(statement.Parameters, statementParameters(statement.SelectKey.SQL)...)
			statement.Parameters = append(statement.Parameters, dynamicStatementParameters(statement.SelectKey.DynamicSQL)...)
			statement.Parameters = append(statement.Parameters, callableParameterNames(statement.ParameterModes)...)
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
	if err := appendStatements(orm.StatementCommandCall, mapper.Calls); err != nil {
		return nil, err
	}
	return statements, nil
}

type selectedXMLStatement struct {
	item        xmlStatementModel
	specificity int
}

func selectXMLStatementsForDatabase(command orm.StatementCommand, source []xmlStatementModel, databaseID string) ([]xmlStatementModel, error) {
	orderedIDs := make([]string, 0, len(source))
	selected := make(map[string]selectedXMLStatement, len(source))
	for _, item := range source {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			return nil, fmt.Errorf("goark-orm: XML %s statement missing id", command)
		}
		item.ID = id
		item.DatabaseID = strings.TrimSpace(item.DatabaseID)
		specificity := xmlStatementDatabaseSpecificity(item.DatabaseID, databaseID)
		if specificity < 0 {
			continue
		}
		current, exists := selected[id]
		if exists && current.specificity == specificity {
			return nil, fmt.Errorf("goark-orm: duplicate XML statement %s for databaseId %q", id, item.DatabaseID)
		}
		if !exists {
			orderedIDs = append(orderedIDs, id)
		}
		if !exists || specificity > current.specificity {
			selected[id] = selectedXMLStatement{item: item, specificity: specificity}
		}
	}
	out := make([]xmlStatementModel, 0, len(selected))
	for _, id := range orderedIDs {
		out = append(out, selected[id].item)
	}
	return out, nil
}

func xmlStatementDatabaseSpecificity(statementDatabaseID string, targetDatabaseID string) int {
	switch {
	case targetDatabaseID == "" && statementDatabaseID == "":
		return 0
	case targetDatabaseID == "":
		return -1
	case statementDatabaseID == targetDatabaseID:
		return 2
	case statementDatabaseID == "":
		return 1
	default:
		return -1
	}
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
		if node.Value != "" {
			out = append(out, node.Value)
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
		case orm.DynamicSQLNodeBind:
			if scoped == nil {
				scoped = make(map[string]struct{})
			}
			for _, param := range expressionParameters(node.Value) {
				if _, ok := scoped[param]; !ok {
					*out = append(*out, param)
				}
			}
			if node.Name != "" {
				scoped[node.Name] = struct{}{}
			}
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
			for index < len(value) {
				switch value[index] {
				case '.':
					next := index + 1
					if next >= len(value) || !isIdentifierStart(rune(value[next])) {
						out = append(out, value[start:index])
						goto nextToken
					}
					index = next + 1
					for index < len(value) && isIdentifierPart(rune(value[index])) {
						index++
					}
				case '[':
					end := strings.IndexByte(value[index:], ']')
					if end < 0 {
						out = append(out, value[start:index])
						goto nextToken
					}
					index += end + 1
				default:
					out = append(out, value[start:index])
					goto nextToken
				}
			}
			out = append(out, value[start:index])
		nextToken:
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
