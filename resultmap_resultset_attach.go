package orm

import (
	"fmt"
	"reflect"
)

func attachResultSetChild(root reflect.Value, plan resultSetMappingPlan, child reflect.Value, rootIndex int, columnIndexes map[string]int, values []any, seenCollections map[string]struct{}) error {
	switch plan.kind {
	case resultSetAssociation:
		return assignResultSetAssociation(root, plan, child)
	case resultSetCollection:
		identityKey := resultObjectKey(plan.identity, columnIndexes, values)
		seenKey := fmt.Sprintf("%d\x00%s\x00%s", rootIndex, plan.property, identityKey)
		if _, exists := seenCollections[seenKey]; exists {
			return nil
		}
		if err := appendResultSetCollection(root, plan, child); err != nil {
			return err
		}
		seenCollections[seenKey] = struct{}{}
	}
	return nil
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
