package runtime

import "strings"

const (
	mergeTargetAlias = "goark_orm_target"
	mergeSourceAlias = "goark_orm_source"
)

// buildMergeUpsertSQL 生成 SQL Server 和 Oracle 都能执行的单行 MERGE。
func buildMergeUpsertSQL(dialect Dialect, spec UpsertSpec) (SQLSource, error) {
	if len(spec.ConflictColumns) == 0 {
		return SQLSource{}, configurationErrorf("merge upsert requires conflict columns")
	}
	if len(spec.ReturningColumns) > 0 {
		return SQLSource{}, configurationErrorf("merge upsert does not support returning columns")
	}
	state := newSQLBuilderState()
	table, err := state.identifier(spec.Table)
	if err != nil {
		return SQLSource{}, err
	}
	source, err := mergeSourceSelect(dialect, state, spec.InsertColumns, spec.Values)
	if err != nil {
		return SQLSource{}, err
	}
	on, err := mergeConflictPredicate(state, spec.ConflictColumns)
	if err != nil {
		return SQLSource{}, err
	}
	insertColumns, insertValues, err := mergeInsertClause(state, spec.InsertColumns)
	if err != nil {
		return SQLSource{}, err
	}
	parts := []string{"MERGE INTO " + table + " " + mergeTargetAlias, "USING " + source + " " + mergeSourceAlias, "ON (" + strings.Join(on, " AND ") + ")"}
	if len(spec.UpdateColumns) > 0 {
		sets, err := mergeUpdateAssignments(state, spec.UpdateColumns)
		if err != nil {
			return SQLSource{}, err
		}
		parts = append(parts, "WHEN MATCHED THEN UPDATE SET "+strings.Join(sets, ", "))
	}
	parts = append(parts, "WHEN NOT MATCHED THEN INSERT ("+strings.Join(insertColumns, ", ")+") VALUES ("+strings.Join(insertValues, ", ")+")")
	sqlText := strings.Join(parts, " ")
	if DialectCapabilitiesOf(dialect).DBType == DbTypeSQLServer {
		sqlText += ";"
	}
	return state.source(sqlText, spec.CacheKey), nil
}

func mergeSourceSelect(dialect Dialect, state *sqlBuilderState, columns []string, values NamedArgs) (string, error) {
	if len(columns) == 0 {
		return "", configurationErrorf("merge upsert requires insert columns")
	}
	projections := make([]string, 0, len(columns))
	for _, columnName := range columns {
		key := strings.TrimSpace(columnName)
		value, ok := values[key]
		if !ok {
			return "", &BindingError{
				Operation: "build merge upsert",
				Parameter: key,
				Message:   "merge upsert value is missing",
			}
		}
		alias, err := state.identifier(key)
		if err != nil {
			return "", err
		}
		projections = append(projections, state.value(value)+" AS "+alias)
	}
	sqlText := "(SELECT " + strings.Join(projections, ", ")
	if DialectCapabilitiesOf(dialect).DBType == DbTypeOracle {
		sqlText += " FROM dual"
	}
	return sqlText + ")", nil
}

func mergeConflictPredicate(state *sqlBuilderState, columns []string) ([]string, error) {
	predicates := make([]string, 0, len(columns))
	for _, columnName := range columns {
		column, err := state.identifier(columnName)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, mergeTargetAlias+"."+column+" = "+mergeSourceAlias+"."+column)
	}
	return predicates, nil
}

func mergeInsertClause(state *sqlBuilderState, columns []string) ([]string, []string, error) {
	insertColumns := make([]string, 0, len(columns))
	insertValues := make([]string, 0, len(columns))
	for _, columnName := range columns {
		column, err := state.identifier(columnName)
		if err != nil {
			return nil, nil, err
		}
		insertColumns = append(insertColumns, column)
		insertValues = append(insertValues, mergeSourceAlias+"."+column)
	}
	return insertColumns, insertValues, nil
}

func mergeUpdateAssignments(state *sqlBuilderState, columns []string) ([]string, error) {
	sets := make([]string, 0, len(columns))
	for _, columnName := range columns {
		column, err := state.identifier(columnName)
		if err != nil {
			return nil, err
		}
		sets = append(sets, mergeTargetAlias+"."+column+" = "+mergeSourceAlias+"."+column)
	}
	return sets, nil
}
