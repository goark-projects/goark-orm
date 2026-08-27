package orm

import (
	"fmt"
	"reflect"
	"strings"
)

type resultSetMappingKind uint8

const (
	resultSetAssociation resultSetMappingKind = iota + 1
	resultSetCollection
)

type resultSetMappingPlan struct {
	kind           resultSetMappingKind
	property       string
	resultSet      string
	parentColumns  []string
	foreignColumns []string
	fieldIndex     []int
	targetType     reflect.Type
	pointerElem    bool
	bindings       map[string]columnBinding
	identity       []ResultFieldMeta
	presence       []ResultFieldMeta
}

func resultSetRootValues(target reflect.Value) ([]reflect.Value, bool) {
	target, ok := nestedSelectRootValue(target)
	if !ok {
		return nil, false
	}
	if target.Kind() == reflect.Struct {
		return []reflect.Value{target}, true
	}
	if target.Kind() != reflect.Slice {
		return nil, false
	}
	roots := make([]reflect.Value, 0, target.Len())
	for index := 0; index < target.Len(); index++ {
		root := rootValueAt(target, index)
		if root.IsValid() && root.Kind() == reflect.Struct {
			roots = append(roots, root)
		}
	}
	return roots, true
}

func resultSetMappingPlans(rootType reflect.Type, resultMap ResultMapMeta) ([]resultSetMappingPlan, error) {
	plans := make([]resultSetMappingPlan, 0, len(resultMap.Associations)+len(resultMap.Collections))
	for _, association := range resultMap.Associations {
		plan, ok, err := resultSetAssociationPlan(rootType, association)
		if err != nil {
			return nil, err
		}
		if ok {
			plans = append(plans, plan)
		}
	}
	for _, collection := range resultMap.Collections {
		plan, ok, err := resultSetCollectionPlan(rootType, collection)
		if err != nil {
			return nil, err
		}
		if ok {
			plans = append(plans, plan)
		}
	}
	return plans, nil
}

func resultSetAssociationPlan(rootType reflect.Type, association ResultAssociationMeta) (resultSetMappingPlan, bool, error) {
	resultSet := strings.TrimSpace(association.ResultSet)
	if resultSet == "" {
		return resultSetMappingPlan{}, false, nil
	}
	field, ok := exportedStructField(rootType, association.Property)
	if !ok {
		return resultSetMappingPlan{}, false, nil
	}
	targetType := dereferenceType(field.Type)
	if targetType.Kind() != reflect.Struct {
		return resultSetMappingPlan{}, false, fmt.Errorf("association %s resultSet target must be struct", association.Property)
	}
	parentColumns, foreignColumns, err := resultSetJoinColumns(association.Column, association.ForeignColumn)
	if err != nil {
		return resultSetMappingPlan{}, false, err
	}
	return resultSetMappingPlan{
		kind:           resultSetAssociation,
		property:       association.Property,
		resultSet:      resultSet,
		parentColumns:  parentColumns,
		foreignColumns: foreignColumns,
		fieldIndex:     field.Index,
		targetType:     targetType,
		bindings:       resultSetObjectBindings(targetType, association.Fields, association.Associations),
		identity:       resultIdentityFields(association.Fields),
		presence:       resultPresenceFields(association.NotNullColumns, "", resultIdentityFields(association.Fields)),
	}, true, nil
}

func resultSetCollectionPlan(rootType reflect.Type, collection ResultCollectionMeta) (resultSetMappingPlan, bool, error) {
	resultSet := strings.TrimSpace(collection.ResultSet)
	if resultSet == "" {
		return resultSetMappingPlan{}, false, nil
	}
	field, ok := exportedStructField(rootType, collection.Property)
	if !ok {
		return resultSetMappingPlan{}, false, nil
	}
	if field.Type.Kind() != reflect.Slice {
		return resultSetMappingPlan{}, false, fmt.Errorf("collection %s resultSet target must be slice", collection.Property)
	}
	targetType := field.Type.Elem()
	pointerElem := targetType.Kind() == reflect.Pointer
	if pointerElem {
		targetType = targetType.Elem()
	}
	if targetType.Kind() != reflect.Struct {
		return resultSetMappingPlan{}, false, fmt.Errorf("collection %s resultSet element must be struct", collection.Property)
	}
	parentColumns, foreignColumns, err := resultSetJoinColumns(collection.Column, collection.ForeignColumn)
	if err != nil {
		return resultSetMappingPlan{}, false, err
	}
	return resultSetMappingPlan{
		kind:           resultSetCollection,
		property:       collection.Property,
		resultSet:      resultSet,
		parentColumns:  parentColumns,
		foreignColumns: foreignColumns,
		fieldIndex:     field.Index,
		targetType:     targetType,
		pointerElem:    pointerElem,
		bindings:       resultSetObjectBindings(targetType, collection.Fields, collection.Associations),
		identity:       resultIdentityFields(collection.Fields),
		presence:       resultPresenceFields(collection.NotNullColumns, "", resultIdentityFields(collection.Fields)),
	}, true, nil
}

func resultSetObjectBindings(typ reflect.Type, fields []ResultFieldMeta, associations []ResultAssociationMeta) map[string]columnBinding {
	bindings := exportedFieldBindings(typ)
	for _, item := range fields {
		addDirectFieldBinding(bindings, typ, item, nil)
	}
	for _, association := range associations {
		addAssociationBindings(bindings, typ, nil, association, "")
	}
	return bindings
}

func resultSetJoinColumns(parent string, foreign string) ([]string, []string, error) {
	parentColumns := splitResultSetColumnList(parent)
	foreignColumns := splitResultSetColumnList(foreign)
	if len(parentColumns) == 0 || len(foreignColumns) == 0 {
		return nil, nil, fmt.Errorf("resultSet mapping requires column and foreignColumn")
	}
	if len(parentColumns) != len(foreignColumns) {
		return nil, nil, fmt.Errorf("resultSet mapping column count %d does not match foreignColumn count %d", len(parentColumns), len(foreignColumns))
	}
	return parentColumns, foreignColumns, nil
}

func splitResultSetColumnList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func resultSetMappingOrder(statement StatementMeta, plans []resultSetMappingPlan) ([]string, error) {
	needed := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		needed[plan.resultSet] = struct{}{}
	}
	if len(statement.ResultSets) == 0 {
		return nil, fmt.Errorf("statement %s resultSet mappings require StatementMeta.ResultSets order", statement.FullName)
	}
	start := 0
	if _, ok := needed[strings.TrimSpace(statement.ResultSets[0].Name)]; !ok {
		start = 1
	}
	order := make([]string, 0, len(statement.ResultSets)-start)
	for index := start; index < len(statement.ResultSets); index++ {
		name := strings.TrimSpace(statement.ResultSets[index].Name)
		if name == "" {
			continue
		}
		order = append(order, name)
		delete(needed, name)
	}
	if len(needed) > 0 {
		missing := make([]string, 0, len(needed))
		for name := range needed {
			missing = append(missing, name)
		}
		return nil, fmt.Errorf("statement %s does not declare resultSets %s", statement.FullName, strings.Join(missing, ","))
	}
	return order, nil
}

func plansByResultSet(plans []resultSetMappingPlan, name string) []resultSetMappingPlan {
	out := make([]resultSetMappingPlan, 0, len(plans))
	for _, plan := range plans {
		if plan.resultSet == name {
			out = append(out, plan)
		}
	}
	return out
}
