package orm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

type resultCollectionPlan struct {
	property    string
	fieldIndex  []int
	elementType reflect.Type
	pointerElem bool
	bindings    map[string]columnBinding
	identity    []ResultFieldMeta
	presence    []ResultFieldMeta
}

func (s *SQLSession) scanRowsWithCollections(ctx context.Context, rows Rows, columns []string, statement StatementMeta, resultMap ResultMapMeta, target reflect.Value) error {
	if target.Kind() != reflect.Slice {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: "Query destination must be pointer to slice"}
	}
	elementType := target.Type().Elem()
	rootPointer := elementType.Kind() == reflect.Pointer
	rootType := elementType
	if rootPointer {
		rootType = elementType.Elem()
	}
	if rootType.Kind() != reflect.Struct {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: "collection resultMap requires struct root type"}
	}
	rootBindings := s.columnBindings(statement, rootType)
	plans := resultCollectionPlans(rootType, resultMap.Collections, "")
	columnIndexes := resultColumnIndexes(columns)
	roots := make(map[string]int)
	seenCollections := make(map[string]map[string]struct{})
	for rows.Next() {
		values, err := scanRowValues(rows, len(columns))
		if err != nil {
			return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
		}
		rootKey := resultObjectKey(resultMapFieldMetas(resultMap), columnIndexes, values)
		rootIndex, exists := roots[rootKey]
		if !exists {
			root := reflect.New(rootType).Elem()
			if err := s.applyBindings(ctx, root, rootBindings, columns, values); err != nil {
				return mappingFailure(statement, err)
			}
			if rootPointer {
				pointer := reflect.New(rootType)
				pointer.Elem().Set(root)
				target.Set(reflect.Append(target, pointer))
			} else {
				target.Set(reflect.Append(target, root))
			}
			rootIndex = target.Len() - 1
			roots[rootKey] = rootIndex
		}
		rootValue := rootValueAt(target, rootIndex)
		for _, plan := range plans {
			if !resultObjectPresent(plan.presence, columnIndexes, values) {
				continue
			}
			childKey := resultObjectKey(plan.identity, columnIndexes, values)
			seenKey := rootKey + "\x00" + plan.property
			seen := seenCollections[seenKey]
			if seen == nil {
				seen = make(map[string]struct{})
				seenCollections[seenKey] = seen
			}
			if _, ok := seen[childKey]; ok {
				continue
			}
			child := reflect.New(plan.elementType).Elem()
			if err := s.applyBindings(ctx, child, plan.bindings, columns, values); err != nil {
				return mappingFailure(statement, err)
			}
			if err := appendCollectionElement(rootValue, plan, child); err != nil {
				return mappingFailure(statement, err)
			}
			seen[childKey] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
	}
	if resultMapHasNestedSelects(resultMap) {
		if err := rows.Close(); err != nil {
			return &ExecutorError{Statement: statement.FullName, Operation: "close rows", Err: err}
		}
		return mappingFailure(statement, s.applyNestedSelects(ctx, statement, resultMap, target))
	}
	return nil
}

func resultCollectionPlans(rootType reflect.Type, collections []ResultCollectionMeta, inheritedPrefix string) []resultCollectionPlan {
	plans := make([]resultCollectionPlan, 0, len(collections))
	for _, collection := range collections {
		field, ok := rootType.FieldByName(collection.Property)
		if !ok || field.PkgPath != "" || field.Type.Kind() != reflect.Slice {
			continue
		}
		elementType := field.Type.Elem()
		pointerElem := elementType.Kind() == reflect.Pointer
		if pointerElem {
			elementType = elementType.Elem()
		}
		if elementType.Kind() != reflect.Struct {
			continue
		}
		prefix := inheritedPrefix + collection.ColumnPrefix
		bindings := exportedFieldBindings(elementType)
		for _, item := range collection.Fields {
			nestedField, ok := elementType.FieldByName(item.Property)
			if !ok || nestedField.PkgPath != "" {
				continue
			}
			column := prefixColumn(prefix, item.Column)
			if column == "" {
				continue
			}
			bindings[normalizeColumnKey(column)] = columnBinding{
				index:       nestedField.Index,
				typeHandler: item.TypeHandler,
				fieldName:   nestedField.Name,
			}
		}
		for _, association := range collection.Associations {
			addAssociationBindings(bindings, elementType, nil, association, prefix)
		}
		identity := resultIdentityFields(prefixedResultFields(collection.Fields, prefix))
		plans = append(plans, resultCollectionPlan{
			property:    collection.Property,
			fieldIndex:  field.Index,
			elementType: elementType,
			pointerElem: pointerElem,
			bindings:    bindings,
			identity:    identity,
			presence:    resultPresenceFields(collection.NotNullColumns, prefix, identity),
		})
	}
	return plans
}

func (s *SQLSession) applyBindings(ctx context.Context, target reflect.Value, bindings map[string]columnBinding, columns []string, values []any) error {
	var columnIndexes map[string]int
	for index, column := range columns {
		if index < 0 || index >= len(values) {
			continue
		}
		binding, ok := s.lookupColumnBinding(bindings, column)
		if !ok {
			continue
		}
		if len(binding.presenceColumns) > 0 {
			if columnIndexes == nil {
				columnIndexes = resultColumnIndexes(columns)
			}
			if !resultColumnsPresent(binding.presenceColumns, columnIndexes, values) {
				continue
			}
		}
		field, ok := fieldByBindingIndexAlloc(target, binding.index)
		if !ok || !field.IsValid() || !field.CanSet() {
			continue
		}
		if err := s.setFieldFromDB(ctx, field, values[index], binding.typeHandler); err != nil {
			return &MappingError{
				Column: column,
				Field:  binding.fieldName,
				Err:    err,
			}
		}
	}
	return nil
}

func (s *SQLSession) setFieldFromDB(ctx context.Context, field reflect.Value, value any, typeHandler string) error {
	if typeHandler != "" {
		handler, ok := s.typeHandlers[typeHandler]
		if !ok {
			return mappingErrorf("type-handler %q is not registered", typeHandler)
		}
		if err := handler.FromDB(ctx, value, field.Addr().Interface()); err != nil {
			return &MappingError{
				Message: fmt.Sprintf("type-handler %q failed", typeHandler),
				Err:     err,
			}
		}
		return nil
	}
	return setReflectField(field, value)
}

func setReflectField(field reflect.Value, value any) error {
	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}
	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setReflectField(field.Elem(), value)
	}
	if bytes, ok := value.([]byte); ok && field.Kind() == reflect.String {
		field.SetString(string(bytes))
		return nil
	}
	source := reflect.ValueOf(value)
	if source.Type().AssignableTo(field.Type()) {
		field.Set(source)
		return nil
	}
	if source.Type().ConvertibleTo(field.Type()) {
		field.Set(source.Convert(field.Type()))
		return nil
	}
	return fmt.Errorf("goark-orm: database value %T cannot assign to field %s", value, field.Type())
}

func fieldByIndexAlloc(root reflect.Value, index []int) (reflect.Value, bool) {
	value := root
	for _, item := range index {
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				value.Set(reflect.New(value.Type().Elem()))
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct || item < 0 || item >= value.NumField() {
			return reflect.Value{}, false
		}
		value = value.Field(item)
	}
	return value, true
}

func fieldByBindingIndexAlloc(root reflect.Value, index []int) (reflect.Value, bool) {
	if len(index) == 1 && root.Kind() == reflect.Struct {
		item := index[0]
		if item < 0 || item >= root.NumField() {
			return reflect.Value{}, false
		}
		return root.Field(item), true
	}
	return fieldByIndexAlloc(root, index)
}

func rootValueAt(target reflect.Value, index int) reflect.Value {
	value := target.Index(index)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return value.Elem()
	}
	return value
}

func appendCollectionElement(root reflect.Value, plan resultCollectionPlan, child reflect.Value) error {
	field, ok := fieldByIndexAlloc(root, plan.fieldIndex)
	if !ok || !field.IsValid() || !field.CanSet() {
		return nil
	}
	if field.Kind() != reflect.Slice {
		return mappingErrorf("collection property %s is not slice", plan.property)
	}
	if field.IsNil() {
		field.Set(reflect.MakeSlice(field.Type(), 0, 1))
	}
	if plan.pointerElem {
		pointer := reflect.New(plan.elementType)
		pointer.Elem().Set(child)
		field.Set(reflect.Append(field, pointer))
		return nil
	}
	field.Set(reflect.Append(field, child))
	return nil
}

func scanRowValues(rows interface{ Scan(dest ...any) error }, count int) ([]any, error) {
	values := make([]any, count)
	targets := make([]any, count)
	for index := range values {
		targets[index] = &values[index]
	}
	if err := rows.Scan(targets...); err != nil {
		return nil, err
	}
	return values, nil
}

func resultColumnIndexes(columns []string) map[string]int {
	indexes := make(map[string]int, len(columns))
	for index, column := range columns {
		indexes[normalizeColumnKey(column)] = index
	}
	return indexes
}

func resultIdentityFields(fields []ResultFieldMeta) []ResultFieldMeta {
	ids := make([]ResultFieldMeta, 0)
	for _, field := range fields {
		if field.ID {
			ids = append(ids, field)
		}
	}
	if len(ids) > 0 {
		return ids
	}
	return fields
}

func resultMapFieldMetas(resultMap ResultMapMeta) []ResultFieldMeta {
	out := make([]ResultFieldMeta, 0, len(resultMap.Constructor.Args)+len(resultMap.Fields))
	for _, arg := range resultMap.Constructor.Args {
		property := strings.TrimSpace(arg.Property)
		if property == "" {
			property = strings.TrimSpace(arg.Name)
		}
		out = append(out, ResultFieldMeta{
			Property:    property,
			Column:      strings.TrimSpace(arg.Column),
			ID:          arg.ID,
			TypeHandler: strings.TrimSpace(arg.TypeHandler),
		})
	}
	out = append(out, resultMap.Fields...)
	return out
}

func prefixedResultFields(fields []ResultFieldMeta, prefix string) []ResultFieldMeta {
	if len(fields) == 0 {
		return nil
	}
	out := make([]ResultFieldMeta, 0, len(fields))
	for _, field := range fields {
		field.Column = prefixColumn(prefix, field.Column)
		out = append(out, field)
	}
	return out
}

func resultPresenceFields(notNullColumns []string, prefix string, fallback []ResultFieldMeta) []ResultFieldMeta {
	columns := prefixedColumns(notNullColumns, prefix)
	if len(columns) == 0 {
		return fallback
	}
	out := make([]ResultFieldMeta, 0, len(columns))
	for _, column := range columns {
		out = append(out, ResultFieldMeta{Column: column})
	}
	return out
}

func resultObjectPresent(fields []ResultFieldMeta, columns map[string]int, values []any) bool {
	for _, field := range resultIdentityFields(fields) {
		index, ok := columns[normalizeColumnKey(field.Column)]
		if !ok || index < 0 || index >= len(values) {
			continue
		}
		if !isZeroDatabaseValue(values[index]) {
			return true
		}
	}
	return false
}

func resultColumnsPresent(fields []string, columns map[string]int, values []any) bool {
	for _, column := range fields {
		index, ok := columns[normalizeColumnKey(column)]
		if !ok || index < 0 || index >= len(values) {
			continue
		}
		if !isZeroDatabaseValue(values[index]) {
			return true
		}
	}
	return false
}

func resultObjectKey(fields []ResultFieldMeta, columns map[string]int, values []any) string {
	identity := resultIdentityFields(fields)
	var builder strings.Builder
	for _, field := range identity {
		index, ok := columns[normalizeColumnKey(field.Column)]
		if !ok || index < 0 || index >= len(values) {
			continue
		}
		value := values[index]
		builder.WriteString(field.Column)
		builder.WriteByte('=')
		_, _ = fmt.Fprintf(&builder, "%T:%#v", value, value)
		builder.WriteByte(';')
	}
	if builder.Len() == 0 {
		for index, value := range values {
			_, _ = fmt.Fprintf(&builder, "%d:%T:%#v;", index, value, value)
		}
	}
	return builder.String()
}

func isZeroDatabaseValue(value any) bool {
	if value == nil {
		return true
	}
	reflectValue := reflect.ValueOf(value)
	return reflectValue.IsZero()
}
