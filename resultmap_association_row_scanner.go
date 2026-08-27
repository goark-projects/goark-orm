package orm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

func resultMapAssociationRowScannerSupported(resultMap ResultMapMeta) bool {
	if resultMapHasDiscriminator(resultMap) || resultMapHasNestedSelects(resultMap) {
		return false
	}
	if len(resultMap.Associations) == 0 || len(resultMap.Collections) > 0 {
		return false
	}
	if resultMap.AutoMapping == nil || *resultMap.AutoMapping {
		return false
	}
	if resultFieldsHaveTypeHandler(resultMapFieldMetas(resultMap)) {
		return false
	}
	return resultAssociationsRowScannerSupported(resultMap.Associations)
}

func resultAssociationsRowScannerSupported(associations []ResultAssociationMeta) bool {
	for _, association := range associations {
		if strings.TrimSpace(association.TypeName) == "" || strings.TrimSpace(association.Select) != "" ||
			strings.TrimSpace(association.ResultSet) != "" || len(association.Collections) > 0 {
			return false
		}
		if resultFieldsHaveTypeHandler(association.Fields) {
			return false
		}
		if !resultAssociationsRowScannerSupported(association.Associations) {
			return false
		}
	}
	return true
}

func resultFieldsHaveTypeHandler(fields []ResultFieldMeta) bool {
	for _, field := range fields {
		if strings.TrimSpace(field.TypeHandler) != "" {
			return true
		}
	}
	return false
}

func (s *SQLSession) scanResultMapAssociationsWithRegisteredRowScanners(ctx context.Context, row RowScannerRow, columns []string, statement StatementMeta, resultMap ResultMapMeta, target reflect.Value) (error, bool) {
	if s == nil || s.registry == nil || row == nil || !target.IsValid() || !target.CanAddr() {
		return nil, false
	}
	rootType := target.Type()
	rootScanner, ok := s.rowScannerForResultMapType(rootType, resultMap.TypeName)
	if !ok || !s.associationRowScannersAvailable(rootType, resultMap.Associations) {
		return nil, false
	}
	values, err := scanRowValues(row, len(columns))
	if err != nil {
		return mappingFailure(statement, &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}), true
	}
	valueRow := resultMapValueRow{values: values}
	rootColumns := resultMapDirectRowScannerColumns(resultMapFieldMetas(resultMap), "", columns)
	if err := rootScanner.ScanRow(ctx, rootColumns, valueRow, target.Addr().Interface()); err != nil {
		return mappingFailure(statement, err), true
	}
	columnIndexes := resultColumnIndexes(columns)
	if err := s.scanResultMapAssociations(ctx, statement, resultMap.ID, target, resultMap.Associations, "", columns, columnIndexes, values, valueRow); err != nil {
		return mappingFailure(statement, err), true
	}
	return nil, true
}

func (s *SQLSession) scanResultMapAssociations(ctx context.Context, statement StatementMeta, resultMapID string, root reflect.Value, associations []ResultAssociationMeta, inheritedPrefix string, columns []string, columnIndexes map[string]int, values []any, row resultMapValueRow) error {
	for _, association := range associations {
		field, ok, err := associationField(root, association.Property)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		prefix := inheritedPrefix + strings.TrimSpace(association.ColumnPrefix)
		if !resultAssociationPresent(association, prefix, columnIndexes, values) {
			continue
		}
		target, scannerDest, ok, err := associationScanDestination(field)
		if err != nil || !ok {
			return err
		}
		scanner, ok := s.rowScannerForResultMapType(target.Type(), association.TypeName)
		if !ok {
			return &MappingError{Statement: statement.FullName, ResultMap: resultMapID, Field: association.Property, Message: fmt.Sprintf("row scanner for association %s is not registered", association.TypeName)}
		}
		scannerColumns := resultMapDirectRowScannerColumns(association.Fields, prefix, columns)
		if err := scanner.ScanRow(ctx, scannerColumns, row, scannerDest); err != nil {
			return err
		}
		if err := s.scanResultMapAssociations(ctx, statement, resultMapID, target, association.Associations, prefix, columns, columnIndexes, values, row); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLSession) associationRowScannersAvailable(rootType reflect.Type, associations []ResultAssociationMeta) bool {
	for _, association := range associations {
		field, ok := exportedStructField(rootType, association.Property)
		if !ok {
			return false
		}
		associationType := dereferenceType(field.Type)
		if associationType.Kind() != reflect.Struct {
			return false
		}
		if _, ok := s.rowScannerForResultMapType(associationType, association.TypeName); !ok {
			return false
		}
		if !s.associationRowScannersAvailable(associationType, association.Associations) {
			return false
		}
	}
	return true
}

func (s *SQLSession) rowScannerForResultMapType(typ reflect.Type, typeName string) (RowScanner, bool) {
	if s == nil || s.registry == nil || typ == nil {
		return nil, false
	}
	typ = dereferenceType(typ)
	if typ.Kind() != reflect.Struct {
		return nil, false
	}
	for _, name := range []string{strings.TrimSpace(typeName), typ.Name(), normalizeTypeIdentifier(typeName)} {
		if name == "" {
			continue
		}
		if scanner, ok := s.registry.RowScanner(name); ok {
			return scanner, true
		}
	}
	return nil, false
}

func resultMapDirectRowScannerColumns(fields []ResultFieldMeta, prefix string, columns []string) []string {
	out := make([]string, len(columns))
	properties := resultFieldsColumnProperties(fields, prefix)
	for index, column := range columns {
		if property := properties[normalizeColumnKey(column)]; property != "" {
			out[index] = property
			continue
		}
		out[index] = fmt.Sprintf("__goark_orm_discard_%d", index)
	}
	return out
}

func resultFieldsColumnProperties(fields []ResultFieldMeta, prefix string) map[string]string {
	properties := make(map[string]string, len(fields))
	for _, field := range fields {
		addResultMapColumnProperty(properties, prefixColumn(prefix, field.Column), field.Property)
	}
	return properties
}

func resultAssociationPresent(association ResultAssociationMeta, prefix string, columnIndexes map[string]int, values []any) bool {
	fields := prefixedResultFields(association.Fields, prefix)
	presence := resultPresenceFields(association.NotNullColumns, prefix, resultIdentityFields(fields))
	return resultObjectPresent(presence, columnIndexes, values)
}

func associationField(root reflect.Value, property string) (reflect.Value, bool, error) {
	field, ok := exportedFieldByProperty(root, property)
	if !ok {
		return reflect.Value{}, false, nil
	}
	if !field.CanSet() {
		return reflect.Value{}, false, &MappingError{Field: property, Message: "association field cannot be set"}
	}
	return field, true, nil
}

func associationScanDestination(field reflect.Value) (reflect.Value, any, bool, error) {
	for field.Kind() == reflect.Interface {
		if field.IsNil() {
			return reflect.Value{}, nil, false, nil
		}
		field = field.Elem()
	}
	if field.Kind() == reflect.Pointer {
		if field.Type().Elem().Kind() != reflect.Struct {
			return reflect.Value{}, nil, false, nil
		}
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return field.Elem(), field.Interface(), true, nil
	}
	if field.Kind() != reflect.Struct || !field.CanAddr() {
		return reflect.Value{}, nil, false, nil
	}
	return field, field.Addr().Interface(), true, nil
}

func exportedStructField(rootType reflect.Type, property string) (reflect.StructField, bool) {
	rootType = dereferenceType(rootType)
	if rootType.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	field, ok := rootType.FieldByName(strings.TrimSpace(property))
	if !ok || field.PkgPath != "" {
		return reflect.StructField{}, false
	}
	return field, true
}

func dereferenceType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

type resultMapValueRow struct {
	values []any
}

func (r resultMapValueRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return fmt.Errorf("goark-orm: row scanner destination count %d does not match value count %d", len(dest), len(r.values))
	}
	for index, target := range dest {
		if err := scanResultMapValue(target, r.values[index]); err != nil {
			return err
		}
	}
	return nil
}

func scanResultMapValue(target any, value any) error {
	if target == nil {
		return nil
	}
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return fmt.Errorf("goark-orm: row scanner target must be non-nil pointer")
	}
	return setReflectField(targetValue.Elem(), value)
}
