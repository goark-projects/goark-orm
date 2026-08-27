package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
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

func resultMapHasResultSetMappings(resultMap ResultMapMeta) bool {
	if resultObjectsHaveResultSetMappings(resultMap.Associations, resultMap.Collections) {
		return true
	}
	for _, item := range resultMap.Discriminator.Cases {
		if resultObjectsHaveResultSetMappings(item.Associations, item.Collections) {
			return true
		}
	}
	return false
}

func resultObjectsHaveResultSetMappings(associations []ResultAssociationMeta, collections []ResultCollectionMeta) bool {
	for _, association := range associations {
		if strings.TrimSpace(association.ResultSet) != "" ||
			resultObjectsHaveResultSetMappings(association.Associations, association.Collections) {
			return true
		}
	}
	for _, collection := range collections {
		if strings.TrimSpace(collection.ResultSet) != "" ||
			resultObjectsHaveResultSetMappings(collection.Associations, collection.Collections) {
			return true
		}
	}
	return false
}

func (s *SQLSession) scanRowsWithResultSetMappings(ctx context.Context, rows Rows, columns []string, statement StatementMeta, resultMap ResultMapMeta, target reflect.Value) error {
	if target.Kind() != reflect.Slice {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: "resultSet mapping requires slice destination"}
	}
	rootMap := resultMapWithoutResultSetMappings(resultMap)
	if len(rootMap.Collections) > 0 {
		if err := s.scanRowsWithCollections(ctx, rows, columns, statement, rootMap, target); err != nil {
			return err
		}
	} else {
		if target.IsNil() {
			target.Set(reflect.MakeSlice(target.Type(), 0, 0))
		}
		elementType := target.Type().Elem()
		for rows.Next() {
			element, err := s.scanSliceElementWithResultMap(ctx, rows, columns, statement, rootMap, elementType)
			if err != nil {
				return err
			}
			target.Set(reflect.Append(target, element))
		}
		if err := rows.Err(); err != nil {
			return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
		}
	}
	return s.applyResultSetMappings(ctx, rows, statement, resultMap, target)
}

func (s *SQLSession) scanOneWithResultSetMappings(ctx context.Context, rows Rows, columns []string, statement StatementMeta, resultMap ResultMapMeta, target reflect.Value) error {
	if target.Kind() == reflect.Slice {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: "QueryOne destination must not be slice"}
	}
	if len(resultMapWithoutResultSetMappings(resultMap).Collections) > 0 {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: "QueryOne resultSet mapping does not support inline collection joins"}
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
		}
		return sql.ErrNoRows
	}
	rootMap := resultMapWithoutResultSetMappings(resultMap)
	if err := s.scanValueWithResultMap(ctx, rows, columns, statement, rootMap, target); err != nil {
		return err
	}
	if rows.Next() {
		return &TooManyResultsError{Statement: statement.FullName}
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
	}
	return s.applyResultSetMappings(ctx, rows, statement, resultMap, target)
}

func (s *SQLSession) applyResultSetMappings(ctx context.Context, rows Rows, statement StatementMeta, resultMap ResultMapMeta, target reflect.Value) error {
	roots, ok := resultSetRootValues(target)
	if !ok || len(roots) == 0 {
		return nil
	}
	rootType := roots[0].Type()
	plans, err := resultSetMappingPlans(rootType, resultMap)
	if err != nil {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
	}
	if len(plans) == 0 {
		return nil
	}
	nextRows, ok := rows.(ResultSetRows)
	if !ok {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: "resultSet mapping requires rows with NextResultSet support"}
	}
	order, err := resultSetMappingOrder(statement, plans)
	if err != nil {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
	}
	for _, name := range order {
		if !nextRows.NextResultSet() {
			return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: fmt.Sprintf("resultSet %q is missing", name)}
		}
		columns, err := rows.Columns()
		if err != nil {
			return &ExecutorError{Statement: statement.FullName, Operation: "read resultSet columns", Err: err}
		}
		if err := s.scanMappedResultSet(ctx, rows, columns, statement, resultMap, roots, plansByResultSet(plans, name)); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "iterate result sets", Err: err}
	}
	return nil
}

func (s *SQLSession) scanMappedResultSet(ctx context.Context, rows Rows, columns []string, statement StatementMeta, resultMap ResultMapMeta, roots []reflect.Value, plans []resultSetMappingPlan) error {
	if len(plans) == 0 {
		var targetStack [rowScanStackTargetCount]any
		targets := rowScanTargets(len(columns), &targetStack)
		for rows.Next() {
			for index := range targets {
				var discard any
				targets[index] = &discard
			}
			if err := rows.Scan(targets...); err != nil {
				return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
			}
		}
		return rows.Err()
	}
	columnIndexes := resultColumnIndexes(columns)
	parentIndexes := make([]map[string][]int, len(plans))
	for index, plan := range plans {
		indexes, err := indexResultSetParents(roots, resultMap, plan)
		if err != nil {
			return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
		}
		parentIndexes[index] = indexes
	}
	seenCollections := make(map[string]struct{})
	for rows.Next() {
		values, err := scanRowValues(rows, len(columns))
		if err != nil {
			return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
		}
		for index, plan := range plans {
			childKey, ok, err := resultSetRowKey(columnIndexes, values, plan.foreignColumns)
			if err != nil {
				return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
			}
			if !ok {
				continue
			}
			rootIndexes := parentIndexes[index][childKey]
			if len(rootIndexes) == 0 || !resultObjectPresent(plan.presence, columnIndexes, values) {
				continue
			}
			for _, rootIndex := range rootIndexes {
				child := reflect.New(plan.targetType).Elem()
				if err := s.applyBindings(ctx, child, plan.bindings, columns, values); err != nil {
					return mappingFailure(statement, err)
				}
				root := roots[rootIndex]
				switch plan.kind {
				case resultSetAssociation:
					if err := assignResultSetAssociation(root, plan, child); err != nil {
						return err
					}
				case resultSetCollection:
					identityKey := resultObjectKey(plan.identity, columnIndexes, values)
					seenKey := fmt.Sprintf("%d\x00%s\x00%s", rootIndex, plan.property, identityKey)
					if _, exists := seenCollections[seenKey]; exists {
						continue
					}
					if err := appendResultSetCollection(root, plan, child); err != nil {
						return err
					}
					seenCollections[seenKey] = struct{}{}
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "iterate mapped resultSet", Err: err}
	}
	return nil
}

func resultMapWithoutResultSetMappings(resultMap ResultMapMeta) ResultMapMeta {
	resultMap.Associations = associationsWithoutResultSetMappings(resultMap.Associations)
	resultMap.Collections = collectionsWithoutResultSetMappings(resultMap.Collections)
	return resultMap
}

func associationsWithoutResultSetMappings(items []ResultAssociationMeta) []ResultAssociationMeta {
	if len(items) == 0 {
		return nil
	}
	out := make([]ResultAssociationMeta, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ResultSet) != "" {
			continue
		}
		item.Associations = associationsWithoutResultSetMappings(item.Associations)
		item.Collections = collectionsWithoutResultSetMappings(item.Collections)
		out = append(out, item)
	}
	return out
}

func collectionsWithoutResultSetMappings(items []ResultCollectionMeta) []ResultCollectionMeta {
	if len(items) == 0 {
		return nil
	}
	out := make([]ResultCollectionMeta, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ResultSet) != "" {
			continue
		}
		item.Associations = associationsWithoutResultSetMappings(item.Associations)
		item.Collections = collectionsWithoutResultSetMappings(item.Collections)
		out = append(out, item)
	}
	return out
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

func indexResultSetParents(roots []reflect.Value, resultMap ResultMapMeta, plan resultSetMappingPlan) (map[string][]int, error) {
	indexes := make(map[string][]int, len(roots))
	for index, root := range roots {
		key, ok, err := resultSetRootKey(root, resultMap, plan.parentColumns)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		indexes[key] = append(indexes[key], index)
	}
	return indexes, nil
}

func resultSetRootKey(root reflect.Value, resultMap ResultMapMeta, columns []string) (string, bool, error) {
	values := make([]any, 0, len(columns))
	for _, column := range columns {
		value, ok, err := resultSetRootColumnValue(root, resultMap, column)
		if err != nil || !ok {
			return "", false, err
		}
		if value == nil {
			return "", false, nil
		}
		values = append(values, value)
	}
	return resultSetKey(values), true, nil
}

func resultSetRootColumnValue(root reflect.Value, resultMap ResultMapMeta, column string) (any, bool, error) {
	if property := resultMapPropertyForColumn(resultMap, column); property != "" {
		return resultSetRootFieldValue(root, property)
	}
	return resultSetRootFieldValue(root, column)
}

func resultMapPropertyForColumn(resultMap ResultMapMeta, column string) string {
	key := normalizeColumnKey(column)
	for _, field := range resultMapFieldMetas(resultMap) {
		if normalizeColumnKey(field.Column) == key {
			return strings.TrimSpace(field.Property)
		}
	}
	return ""
}

func resultSetRootFieldValue(root reflect.Value, property string) (any, bool, error) {
	field, ok := exportedFieldByProperty(root, property)
	if !ok {
		return nil, false, nil
	}
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return nil, true, nil
		}
		field = field.Elem()
	}
	if !field.CanInterface() {
		return nil, false, fmt.Errorf("field %s cannot be read", property)
	}
	return field.Interface(), true, nil
}

func resultSetRowKey(columnIndexes map[string]int, values []any, columns []string) (string, bool, error) {
	keyValues := make([]any, 0, len(columns))
	for _, column := range columns {
		index, ok := columnIndexes[normalizeColumnKey(column)]
		if !ok || index < 0 || index >= len(values) {
			return "", false, fmt.Errorf("foreignColumn %s is not present", column)
		}
		if values[index] == nil {
			return "", false, nil
		}
		keyValues = append(keyValues, values[index])
	}
	return resultSetKey(keyValues), true, nil
}

func resultSetKey(values []any) string {
	var builder strings.Builder
	for index, value := range values {
		builder.WriteString(strconv.Itoa(index))
		builder.WriteByte('=')
		writeResultKeyValue(&builder, value)
		builder.WriteByte(';')
	}
	return builder.String()
}

func assignResultSetAssociation(root reflect.Value, plan resultSetMappingPlan, child reflect.Value) error {
	field, ok := fieldByIndexAlloc(root, plan.fieldIndex)
	if !ok || !field.IsValid() || !field.CanSet() {
		return nil
	}
	if field.Kind() == reflect.Pointer {
		pointer := reflect.New(plan.targetType)
		pointer.Elem().Set(child)
		field.Set(pointer)
		return nil
	}
	if child.Type().AssignableTo(field.Type()) {
		field.Set(child)
	}
	return nil
}

func appendResultSetCollection(root reflect.Value, plan resultSetMappingPlan, child reflect.Value) error {
	return appendCollectionElement(root, resultCollectionPlan{
		property:    plan.property,
		fieldIndex:  plan.fieldIndex,
		elementType: plan.targetType,
		pointerElem: plan.pointerElem,
	}, child)
}
