package orm

import "strings"

// mergeUpsertSQL 用实体字段名生成 BaseMapper 原生 MERGE。
func (m *BaseMapper[T, ID]) mergeUpsertSQL(table string, insertFields []ColumnMeta, conflictFields []ColumnMeta, updateFields []ColumnMeta) (string, error) {
	if len(conflictFields) == 0 {
		return "", configurationErrorf("merge upsert requires conflict columns")
	}
	source, err := m.mergeSourceSelect(insertFields)
	if err != nil {
		return "", err
	}
	on, err := m.mergeConflictPredicate(conflictFields)
	if err != nil {
		return "", err
	}
	insertColumns, insertValues, err := m.mergeInsertClause(insertFields)
	if err != nil {
		return "", err
	}
	parts := []string{"MERGE INTO " + table + " " + mergeTargetAlias, "USING " + source + " " + mergeSourceAlias, "ON (" + strings.Join(on, " AND ") + ")"}
	if len(updateFields) > 0 {
		sets, err := m.mergeUpdateAssignments(updateFields)
		if err != nil {
			return "", err
		}
		parts = append(parts, "WHEN MATCHED THEN UPDATE SET "+strings.Join(sets, ", "))
	}
	parts = append(parts, "WHEN NOT MATCHED THEN INSERT ("+strings.Join(insertColumns, ", ")+") VALUES ("+strings.Join(insertValues, ", ")+")")
	sqlText := strings.Join(parts, " ")
	if DialectCapabilitiesOf(m.dialect).DBType == DbTypeSQLServer {
		sqlText += ";"
	}
	return sqlText, nil
}

func (m *BaseMapper[T, ID]) mergeSourceSelect(fields []ColumnMeta) (string, error) {
	if len(fields) == 0 {
		return "", configurationErrorf("merge upsert requires insert columns")
	}
	projections := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := quoteIdentifierPath(m.dialect, field.ColumnName)
		if err != nil {
			return "", err
		}
		projections = append(projections, "#{"+field.FieldName+"} AS "+column)
	}
	sqlText := "(SELECT " + strings.Join(projections, ", ")
	if DialectCapabilitiesOf(m.dialect).DBType == DbTypeOracle {
		sqlText += " FROM dual"
	}
	return sqlText + ")", nil
}

func (m *BaseMapper[T, ID]) mergeConflictPredicate(fields []ColumnMeta) ([]string, error) {
	predicates := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := quoteIdentifierPath(m.dialect, field.ColumnName)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, mergeTargetAlias+"."+column+" = "+mergeSourceAlias+"."+column)
	}
	return predicates, nil
}

func (m *BaseMapper[T, ID]) mergeInsertClause(fields []ColumnMeta) ([]string, []string, error) {
	insertColumns := make([]string, 0, len(fields))
	insertValues := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := quoteIdentifierPath(m.dialect, field.ColumnName)
		if err != nil {
			return nil, nil, err
		}
		insertColumns = append(insertColumns, column)
		insertValues = append(insertValues, mergeSourceAlias+"."+column)
	}
	return insertColumns, insertValues, nil
}

func (m *BaseMapper[T, ID]) mergeUpdateAssignments(fields []ColumnMeta) ([]string, error) {
	sets := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := quoteIdentifierPath(m.dialect, field.ColumnName)
		if err != nil {
			return nil, err
		}
		sets = append(sets, mergeTargetAlias+"."+column+" = "+mergeSourceAlias+"."+column)
	}
	return sets, nil
}
