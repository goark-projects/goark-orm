package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
)

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
	plans, err := resultSetMappingPlans(roots[0].Type(), resultMap)
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
		return drainResultSetRows(rows, len(columns), statement, resultMap)
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
			err := s.applyResultSetRow(ctx, statement, resultMap, roots, parentIndexes[index], columns, columnIndexes, values, plan, seenCollections)
			if err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "iterate mapped resultSet", Err: err}
	}
	return nil
}

func (s *SQLSession) applyResultSetRow(ctx context.Context, statement StatementMeta, resultMap ResultMapMeta, roots []reflect.Value, parentIndexes map[string][]int, columns []string, columnIndexes map[string]int, values []any, plan resultSetMappingPlan, seenCollections map[string]struct{}) error {
	childKey, ok, err := resultSetRowKey(columnIndexes, values, plan.foreignColumns)
	if err != nil {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
	}
	if !ok {
		return nil
	}
	rootIndexes := parentIndexes[childKey]
	if len(rootIndexes) == 0 || !resultObjectPresent(plan.presence, columnIndexes, values) {
		return nil
	}
	for _, rootIndex := range rootIndexes {
		child := reflect.New(plan.targetType).Elem()
		if err := s.applyBindings(ctx, child, plan.bindings, columns, values); err != nil {
			return mappingFailure(statement, err)
		}
		if err := attachResultSetChild(roots[rootIndex], plan, child, rootIndex, columnIndexes, values, seenCollections); err != nil {
			return err
		}
	}
	return nil
}

func drainResultSetRows(rows Rows, columnCount int, statement StatementMeta, resultMap ResultMapMeta) error {
	var targetStack [rowScanStackTargetCount]any
	targets := rowScanTargets(columnCount, &targetStack)
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
