package runtime

import (
	"context"
	"reflect"
	"strings"
)

// scanWithRegisteredResultMapRowScanner 对可安全拆解的 ResultMap 复用生成式 RowScanner。
func (s *SQLSession) scanWithRegisteredResultMapRowScanner(ctx context.Context, row RowScannerRow, columns []string, statement StatementMeta, resultMap ResultMapMeta, target reflect.Value) (error, bool) {
	if !resultMapRowScannerSupported(resultMap) {
		if resultMapAssociationRowScannerSupported(resultMap) {
			return s.scanResultMapAssociationsWithRegisteredRowScanners(ctx, row, columns, statement, resultMap, target)
		}
		return nil, false
	}
	scannerColumns := resultMapRowScannerColumns(resultMap, columns)
	return s.scanWithRegisteredRowScanner(ctx, row, scannerColumns, statement, target)
}

func resultMapRowScannerSupported(resultMap ResultMapMeta) bool {
	if resultMapHasDiscriminator(resultMap) || resultMapHasNestedSelects(resultMap) {
		return false
	}
	if len(resultMap.Associations) > 0 || len(resultMap.Collections) > 0 {
		return false
	}
	for _, field := range resultMap.Fields {
		if strings.TrimSpace(field.TypeHandler) != "" {
			return false
		}
	}
	for _, arg := range resultMap.Constructor.Args {
		if strings.TrimSpace(arg.TypeHandler) != "" {
			return false
		}
	}
	return true
}

func resultMapRowScannerColumns(resultMap ResultMapMeta, columns []string) []string {
	if len(columns) == 0 {
		return nil
	}
	properties := resultMapColumnProperties(resultMap)
	if len(properties) == 0 {
		return append([]string(nil), columns...)
	}
	out := make([]string, len(columns))
	for index, column := range columns {
		if property := properties[normalizeColumnKey(column)]; property != "" {
			out[index] = property
			continue
		}
		out[index] = column
	}
	return out
}

func resultMapColumnProperties(resultMap ResultMapMeta) map[string]string {
	properties := make(map[string]string, len(resultMap.Fields)+len(resultMap.Constructor.Args))
	for _, field := range resultMap.Fields {
		addResultMapColumnProperty(properties, field.Column, field.Property)
	}
	for _, arg := range resultMap.Constructor.Args {
		property := strings.TrimSpace(arg.Property)
		if property == "" {
			property = strings.TrimSpace(arg.Name)
		}
		addResultMapColumnProperty(properties, arg.Column, property)
	}
	return properties
}

func addResultMapColumnProperty(properties map[string]string, column string, property string) {
	column = strings.TrimSpace(column)
	property = strings.TrimSpace(property)
	if column == "" || property == "" {
		return
	}
	properties[normalizeColumnKey(column)] = property
}
