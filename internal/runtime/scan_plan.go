package runtime

import (
	"reflect"
	"strconv"
	"strings"
)

type columnBindingPlanKey struct {
	Statement    string
	Namespace    string
	Type         reflect.Type
	HasResultMap bool
	ResultMap    string
}

// cachedColumnBindingsWithResultMap 复用列到字段的绑定计划，避免每行扫描重复反射建图。
func (s *SQLSession) cachedColumnBindingsWithResultMap(statement StatementMeta, typ reflect.Type, resultMap ResultMapMeta, hasResultMap bool) map[string]columnBinding {
	if s == nil {
		return buildColumnBindingsWithResultMap(nil, statement, typ, resultMap, hasResultMap)
	}
	key := columnBindingPlanKey{
		Statement:    strings.TrimSpace(statement.FullName),
		Namespace:    strings.TrimSpace(statement.Namespace),
		Type:         typ,
		HasResultMap: hasResultMap,
		ResultMap:    resultMapPlanKey(resultMap, hasResultMap),
	}
	if cached, ok := s.columnBindingPlans.Load(key); ok {
		return cached.(map[string]columnBinding)
	}
	bindings := buildColumnBindingsWithResultMap(s, statement, typ, resultMap, hasResultMap)
	actual, _ := s.columnBindingPlans.LoadOrStore(key, bindings)
	return actual.(map[string]columnBinding)
}

func resultMapPlanKey(resultMap ResultMapMeta, hasResultMap bool) string {
	if !hasResultMap {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(resultMap.ID))
	builder.WriteByte('|')
	builder.WriteString(strings.TrimSpace(resultMap.TypeName))
	builder.WriteByte('|')
	if resultMap.AutoMapping != nil {
		builder.WriteString(strconv.FormatBool(*resultMap.AutoMapping))
	}
	builder.WriteByte('|')
	writeResultFieldPlanKey(&builder, resultMap.Constructor.Args)
	writeResultFieldsPlanKey(&builder, resultMap.Fields)
	writeResultAssociationsPlanKey(&builder, resultMap.Associations)
	writeResultCollectionsPlanKey(&builder, resultMap.Collections)
	return builder.String()
}

func writeResultFieldPlanKey(builder *strings.Builder, args []ResultArgMeta) {
	for _, item := range args {
		builder.WriteString("arg:")
		builder.WriteString(strings.TrimSpace(item.Name))
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(item.Property))
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(item.Column))
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(item.TypeHandler))
		builder.WriteByte(';')
	}
}

func writeResultFieldsPlanKey(builder *strings.Builder, fields []ResultFieldMeta) {
	for _, item := range fields {
		builder.WriteString("field:")
		builder.WriteString(strings.TrimSpace(item.Property))
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(item.Column))
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(item.TypeHandler))
		builder.WriteByte(';')
	}
}

func writeResultAssociationsPlanKey(builder *strings.Builder, associations []ResultAssociationMeta) {
	for _, item := range associations {
		builder.WriteString("assoc:")
		builder.WriteString(strings.TrimSpace(item.Property))
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(item.ColumnPrefix))
		builder.WriteByte(':')
		builder.WriteString(strings.Join(item.NotNullColumns, ","))
		builder.WriteByte('{')
		writeResultFieldsPlanKey(builder, item.Fields)
		writeResultAssociationsPlanKey(builder, item.Associations)
		builder.WriteByte('}')
	}
}

func writeResultCollectionsPlanKey(builder *strings.Builder, collections []ResultCollectionMeta) {
	for _, item := range collections {
		builder.WriteString("coll:")
		builder.WriteString(strings.TrimSpace(item.Property))
		builder.WriteByte(':')
		builder.WriteString(strings.TrimSpace(item.ColumnPrefix))
		builder.WriteByte(':')
		builder.WriteString(strings.Join(item.NotNullColumns, ","))
		builder.WriteByte('{')
		writeResultFieldsPlanKey(builder, item.Fields)
		writeResultAssociationsPlanKey(builder, item.Associations)
		writeResultCollectionsPlanKey(builder, item.Collections)
		builder.WriteByte('}')
	}
}
