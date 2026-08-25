package orm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

func resultMapHasDiscriminator(resultMap ResultMapMeta) bool {
	return strings.TrimSpace(resultMap.Discriminator.Column) != "" && len(resultMap.Discriminator.Cases) > 0
}

func (s *SQLSession) effectiveResultMapForRow(ctx context.Context, statement StatementMeta, resultMap ResultMapMeta, columnIndexes map[string]int, values []any, targetType reflect.Type) (ResultMapMeta, error) {
	return s.effectiveResultMapForRowWithStack(ctx, statement, resultMap, columnIndexes, values, targetType, nil)
}

func (s *SQLSession) effectiveResultMapForRowWithStack(ctx context.Context, statement StatementMeta, resultMap ResultMapMeta, columnIndexes map[string]int, values []any, targetType reflect.Type, stack []string) (ResultMapMeta, error) {
	if !resultMapHasDiscriminator(resultMap) {
		return resultMap, nil
	}
	discriminator := resultMap.Discriminator
	index, ok := columnIndexes[normalizeColumnKey(discriminator.Column)]
	if !ok || index < 0 || index >= len(values) {
		return resultMap, nil
	}
	value, err := s.discriminatorString(ctx, discriminator, values[index])
	if err != nil {
		return ResultMapMeta{}, err
	}
	for _, item := range discriminator.Cases {
		if item.Value != value {
			continue
		}
		if item.ResultMap != "" {
			referenced, ok := s.lookupResultMap(statement.Namespace, item.ResultMap)
			if !ok {
				return ResultMapMeta{}, fmt.Errorf("goark-orm: discriminator resultMap %q on statement %s is not registered", item.ResultMap, statement.FullName)
			}
			if containsResultMapID(stack, referenced.ID) {
				return ResultMapMeta{}, fmt.Errorf("goark-orm: discriminator resultMap cycle detected: %s", strings.Join(append(stack, referenced.ID), " -> "))
			}
			effective, err := s.effectiveResultMapForRowWithStack(ctx, statement, referenced, columnIndexes, values, targetType, append(stack, resultMap.ID))
			if err != nil {
				return ResultMapMeta{}, err
			}
			if err := validateDiscriminatorResultType(statement, effective, targetType); err != nil {
				return ResultMapMeta{}, err
			}
			return effective, nil
		}
		effective := mergeDiscriminatorCase(resultMap, item)
		if err := validateDiscriminatorResultType(statement, effective, targetType); err != nil {
			return ResultMapMeta{}, err
		}
		return effective, nil
	}
	return resultMap, nil
}

func (s *SQLSession) discriminatorString(ctx context.Context, discriminator ResultDiscriminatorMeta, value any) (string, error) {
	if discriminator.TypeHandler == "" {
		return databaseValueString(value), nil
	}
	handler, ok := s.typeHandlers[discriminator.TypeHandler]
	if !ok {
		return "", fmt.Errorf("goark-orm: type-handler %q is not registered", discriminator.TypeHandler)
	}
	target := discriminatorTypeHandlerTarget(discriminator.TypeName)
	if err := handler.FromDB(ctx, value, target); err != nil {
		return "", fmt.Errorf("goark-orm: discriminator type-handler %q failed: %w", discriminator.TypeHandler, err)
	}
	return databaseValueString(reflect.ValueOf(target).Elem().Interface()), nil
}

func discriminatorTypeHandlerTarget(typeName string) any {
	switch normalizeTypeIdentifier(typeName) {
	case "bool":
		var value bool
		return &value
	case "string":
		var value string
		return &value
	case "int":
		var value int
		return &value
	case "int8":
		var value int8
		return &value
	case "int16":
		var value int16
		return &value
	case "int32", "rune":
		var value int32
		return &value
	case "int64":
		var value int64
		return &value
	case "uint":
		var value uint
		return &value
	case "uint8", "byte":
		var value uint8
		return &value
	case "uint16":
		var value uint16
		return &value
	case "uint32":
		var value uint32
		return &value
	case "uint64":
		var value uint64
		return &value
	case "float32":
		var value float32
		return &value
	case "float64":
		var value float64
		return &value
	default:
		var value any
		return &value
	}
}

func databaseValueString(value any) string {
	switch item := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(item)
	default:
		return fmt.Sprint(item)
	}
}

func mergeDiscriminatorCase(base ResultMapMeta, item ResultDiscriminatorCaseMeta) ResultMapMeta {
	out := base
	if strings.TrimSpace(item.ResultType) != "" {
		out.TypeName = strings.TrimSpace(item.ResultType)
	}
	out.Discriminator = ResultDiscriminatorMeta{}
	out.Fields = append(append([]ResultFieldMeta(nil), base.Fields...), item.Fields...)
	out.Associations = append(append([]ResultAssociationMeta(nil), base.Associations...), item.Associations...)
	out.Collections = append(append([]ResultCollectionMeta(nil), base.Collections...), item.Collections...)
	return out
}

func validateDiscriminatorResultType(statement StatementMeta, resultMap ResultMapMeta, targetType reflect.Type) error {
	typeName := normalizeTypeIdentifier(resultMap.TypeName)
	if typeName == "" || targetType == nil {
		return nil
	}
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType.Kind() != reflect.Struct || targetType.Name() == typeName {
		return nil
	}
	return fmt.Errorf("goark-orm: discriminator resultType %q on statement %s cannot scan into %s", resultMap.TypeName, statement.FullName, targetType.Name())
}

func containsResultMapID(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
