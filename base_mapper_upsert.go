package orm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// Upsert 执行方言原生插入或更新。
func (m *BaseMapper[T, ID]) Upsert(ctx context.Context, entity *T, conflictFields []Field[T], updateFields []Field[T]) (Result, error) {
	value, err := entityStructValue(entity)
	if err != nil {
		return Result{}, err
	}
	conflicts, err := m.columnsByFields(conflictFields, "upsert conflict")
	if err != nil {
		return Result{}, err
	}
	updates, err := m.upsertUpdateColumns(updateFields)
	if err != nil {
		return Result{}, err
	}
	if err := m.assignInsertID(ctx, value); err != nil {
		return Result{}, err
	}
	if err := m.fillInsertTimeFields(value); err != nil {
		return Result{}, err
	}
	if err := m.fillUpdateTimeFields(value); err != nil {
		return Result{}, err
	}
	if err := applyMetaObjectHandler(ctx, m.metaObjectHandler, StatementCommandInsert, m.entity, value, nil); err != nil {
		return Result{}, err
	}
	if err := applyMetaObjectHandler(ctx, m.metaObjectHandler, StatementCommandUpdate, m.entity, value, nil); err != nil {
		return Result{}, err
	}
	insertFields, err := m.upsertInsertColumns(value, columnSet(conflicts))
	if err != nil {
		return Result{}, err
	}
	args, err := m.upsertArgs(value, insertFields, updates)
	if err != nil {
		return Result{}, err
	}
	sqlText, err := m.upsertSQL(insertFields, conflicts, updates)
	if err != nil {
		return Result{}, err
	}
	statement := m.statement("Upsert", StatementCommandInsert, sqlText)
	statement.ParameterType = m.entity.TypeName
	statement.UseGeneratedKeys = m.upsertUsesLastInsertID()
	statement.KeyProperty = m.primary.FieldName
	return m.session.ExecStatement(ctx, statement, args)
}

// UpsertBatch 按默认批量大小执行原生 upsert。
func (m *BaseMapper[T, ID]) UpsertBatch(ctx context.Context, entities []T, conflictFields []Field[T], updateFields []Field[T]) (int64, error) {
	return m.UpsertBatchSize(ctx, entities, conflictFields, updateFields, DefaultBatchSize)
}

// UpsertBatchSize 按指定批量大小执行原生 upsert。
func (m *BaseMapper[T, ID]) UpsertBatchSize(ctx context.Context, entities []T, conflictFields []Field[T], updateFields []Field[T], batchSize int) (int64, error) {
	if len(entities) == 0 {
		return 0, nil
	}
	batchSize = normalizeBatchSize(batchSize)
	if session, ok := m.session.(Session); ok {
		return m.runBatch(ctx, session, len(entities), batchSize, func(mapper *BaseMapper[T, ID], index int) error {
			_, err := mapper.Upsert(ctx, &entities[index], conflictFields, updateFields)
			return err
		})
	}
	var rows int64
	for index := range entities {
		result, err := m.Upsert(ctx, &entities[index], conflictFields, updateFields)
		if err != nil {
			return rows, err
		}
		rows += result.RowsAffected
	}
	return rows, nil
}

func (m *BaseMapper[T, ID]) upsertSQL(insertFields []ColumnMeta, conflictFields []ColumnMeta, updateFields []ColumnMeta) (string, error) {
	table, err := m.quotedTable()
	if err != nil {
		return "", err
	}
	insertSQL, err := m.upsertInsertSQL(table, insertFields)
	if err != nil {
		return "", err
	}
	switch DialectCapabilitiesOf(m.dialect).Upsert {
	case DialectUpsertOnConflict:
		return m.onConflictUpsertSQL(insertSQL, conflictFields, updateFields)
	case DialectUpsertOnDuplicateKey:
		return m.onDuplicateKeyUpsertSQL(insertSQL, updateFields)
	default:
		return "", configurationErrorf("upsert is not supported by dialect %q", m.dialect.Name())
	}
}

func (m *BaseMapper[T, ID]) upsertInsertSQL(table string, fields []ColumnMeta) (string, error) {
	if len(fields) == 0 {
		return "", fmt.Errorf("goark-orm: entity %s has no insertable columns", m.entity.TypeName)
	}
	columns := make([]string, 0, len(fields))
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := quoteIdentifierPath(m.dialect, field.ColumnName)
		if err != nil {
			return "", err
		}
		columns = append(columns, column)
		values = append(values, "#{"+field.FieldName+"}")
	}
	return "INSERT INTO " + table + " (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(values, ", ") + ")", nil
}

func (m *BaseMapper[T, ID]) onConflictUpsertSQL(insertSQL string, conflictFields []ColumnMeta, updateFields []ColumnMeta) (string, error) {
	if len(conflictFields) == 0 {
		return "", configurationErrorf("on conflict upsert requires conflict columns")
	}
	conflicts, err := m.upsertQuotedColumns(conflictFields)
	if err != nil {
		return "", err
	}
	if len(updateFields) == 0 {
		return insertSQL + " ON CONFLICT (" + strings.Join(conflicts, ", ") + ") DO NOTHING", nil
	}
	sets, err := m.upsertAssignments(updateFields)
	if err != nil {
		return "", err
	}
	return insertSQL + " ON CONFLICT (" + strings.Join(conflicts, ", ") + ") DO UPDATE SET " + strings.Join(sets, ", "), nil
}

func (m *BaseMapper[T, ID]) onDuplicateKeyUpsertSQL(insertSQL string, updateFields []ColumnMeta) (string, error) {
	if len(updateFields) == 0 {
		return "", configurationErrorf("on duplicate key upsert requires update columns")
	}
	sets, err := m.upsertAssignments(updateFields)
	if err != nil {
		return "", err
	}
	return insertSQL + " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", "), nil
}

func (m *BaseMapper[T, ID]) upsertQuotedColumns(fields []ColumnMeta) ([]string, error) {
	columns := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := quoteIdentifierPath(m.dialect, field.ColumnName)
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, nil
}

func (m *BaseMapper[T, ID]) upsertAssignments(fields []ColumnMeta) ([]string, error) {
	sets := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := quoteIdentifierPath(m.dialect, field.ColumnName)
		if err != nil {
			return nil, err
		}
		sets = append(sets, column+" = #{"+field.FieldName+"}")
	}
	return sets, nil
}

func (m *BaseMapper[T, ID]) upsertInsertColumns(entity reflect.Value, forced map[string]struct{}) ([]ColumnMeta, error) {
	fields := make([]ColumnMeta, 0, len(m.entity.Columns))
	for _, column := range m.entity.Columns {
		value, err := fieldValue(entity, column)
		if err != nil {
			return nil, err
		}
		force := upsertForceInsertColumn(column, value, forced)
		if column.PrimaryKey && m.effectiveColumnIDType(column) == IDTypeAuto && !force {
			continue
		}
		strategy := effectiveFieldStrategy(column.InsertStrategy, m.dbConfig.InsertStrategy)
		if !force && !fieldIncludedByStrategy(value, strategy) {
			continue
		}
		fields = append(fields, column)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("goark-orm: entity %s has no insertable columns", m.entity.TypeName)
	}
	return fields, nil
}

func upsertForceInsertColumn(column ColumnMeta, value any, forced map[string]struct{}) bool {
	if len(forced) == 0 || !column.PrimaryKey {
		return false
	}
	if _, ok := forced[column.ColumnName]; !ok {
		return false
	}
	return !isZeroValue(value)
}

func (m *BaseMapper[T, ID]) upsertUpdateColumns(fields []Field[T]) ([]ColumnMeta, error) {
	columns, err := m.columnsByFields(fields, "upsert update")
	if err != nil {
		return nil, err
	}
	for _, column := range columns {
		if err := validateUpsertUpdateColumn(column); err != nil {
			return nil, err
		}
	}
	return columns, nil
}

func (m *BaseMapper[T, ID]) columnsByFields(fields []Field[T], role string) ([]ColumnMeta, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	out := make([]ColumnMeta, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		columnName := strings.TrimSpace(field.Column)
		if columnName == "" {
			return nil, fmt.Errorf("goark-orm: %s field column is empty", role)
		}
		column, ok := m.columnByName(columnName)
		if !ok {
			return nil, fmt.Errorf("goark-orm: %s field column %q is not mapped by entity %s", role, columnName, m.entity.TypeName)
		}
		if _, exists := seen[column.ColumnName]; exists {
			return nil, fmt.Errorf("goark-orm: duplicate %s field column %q", role, column.ColumnName)
		}
		seen[column.ColumnName] = struct{}{}
		out = append(out, column)
	}
	return out, nil
}

func (m *BaseMapper[T, ID]) columnByName(columnName string) (ColumnMeta, bool) {
	for _, column := range m.entity.Columns {
		if column.ColumnName == columnName {
			return column, true
		}
	}
	return ColumnMeta{}, false
}

func validateUpsertUpdateColumn(column ColumnMeta) error {
	switch {
	case column.PrimaryKey:
		return fmt.Errorf("goark-orm: upsert update field %s must not be primary key", column.FieldName)
	case column.SoftDelete:
		return fmt.Errorf("goark-orm: upsert update field %s must not be soft-delete field", column.FieldName)
	case column.Version:
		return fmt.Errorf("goark-orm: upsert update field %s must not be version field", column.FieldName)
	case column.CreatedAt:
		return fmt.Errorf("goark-orm: upsert update field %s must not be created-at field", column.FieldName)
	default:
		return nil
	}
}

func (m *BaseMapper[T, ID]) upsertArgs(entity reflect.Value, insertFields []ColumnMeta, updateFields []ColumnMeta) (NamedArgs, error) {
	args := make(NamedArgs, len(insertFields)+len(updateFields))
	if err := addColumnArgs(args, entity, insertFields); err != nil {
		return nil, err
	}
	if err := addColumnArgs(args, entity, updateFields); err != nil {
		return nil, err
	}
	return args, nil
}

func addColumnArgs(args NamedArgs, entity reflect.Value, fields []ColumnMeta) error {
	for _, field := range fields {
		value, err := fieldValue(entity, field)
		if err != nil {
			return err
		}
		args[field.FieldName] = value
	}
	return nil
}

func columnSet(columns []ColumnMeta) map[string]struct{} {
	if len(columns) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		out[column.ColumnName] = struct{}{}
	}
	return out
}

func (m *BaseMapper[T, ID]) upsertUsesLastInsertID() bool {
	if m.effectiveColumnIDType(m.primary) != IDTypeAuto {
		return false
	}
	return DialectCapabilitiesOf(m.dialect).GeneratedKey == DialectGeneratedKeyLastInsertID
}
