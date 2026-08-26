package orm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	baseMapperPrimaryKeyArg       = "__goark_orm_pk"
	baseMapperVersionOldArg       = "__goark_orm_version_old"
	baseMapperSoftDeleteLiveArg   = "__goark_orm_soft_delete_live"
	baseMapperSoftDeleteDeleteArg = "__goark_orm_soft_delete_deleted"
)

// StatementSession 执行已经构造好的 StatementMeta。
type StatementSession interface {
	QueryStatement(ctx context.Context, statement StatementMeta, args NamedArgs, dest any) error
	QueryOneStatement(ctx context.Context, statement StatementMeta, args NamedArgs, dest any) error
	ExecStatement(ctx context.Context, statement StatementMeta, args NamedArgs) (Result, error)
}

type dialectProvider interface {
	Dialect() Dialect
}

type identifierGeneratorProvider interface {
	IdentifierGenerator() IdentifierGenerator
}

type metaObjectHandlerProvider interface {
	MetaObjectHandler() MetaObjectHandler
}

type globalConfigProvider interface {
	GlobalConfig() GlobalConfig
}

type baseMapperOptions struct {
	dialect             Dialect
	clock               func() time.Time
	identifierGenerator IdentifierGenerator
	metaObjectHandler   MetaObjectHandler
	globalConfig        GlobalConfig
	globalConfigSet     bool
}

// BaseMapperOption 配置 BaseMapper。
type BaseMapperOption func(*baseMapperOptions)

// WithBaseMapperDialect 指定 BaseMapper 生成 SQL 使用的数据库方言。
func WithBaseMapperDialect(dialect Dialect) BaseMapperOption {
	return func(options *baseMapperOptions) {
		options.dialect = dialect
	}
}

// WithBaseMapperClock 指定自动时间字段使用的时钟。
func WithBaseMapperClock(clock func() time.Time) BaseMapperOption {
	return func(options *baseMapperOptions) {
		if clock != nil {
			options.clock = clock
		}
	}
}

// WithBaseMapperIdentifierGenerator 指定 BaseMapper 插入前主键生成器。
func WithBaseMapperIdentifierGenerator(generator IdentifierGenerator) BaseMapperOption {
	return func(options *baseMapperOptions) {
		options.identifierGenerator = generator
	}
}

// WithBaseMapperMetaObjectHandler 指定 BaseMapper 自动填充处理器。
func WithBaseMapperMetaObjectHandler(handler MetaObjectHandler) BaseMapperOption {
	return func(options *baseMapperOptions) {
		options.metaObjectHandler = handler
	}
}

// WithBaseMapperGlobalConfig 指定 BaseMapper 使用的全局配置。
func WithBaseMapperGlobalConfig(global GlobalConfig) BaseMapperOption {
	return func(options *baseMapperOptions) {
		options.globalConfig = global
		options.globalConfigSet = true
	}
}

// BaseMapper 提供 MyBatis-Plus 风格的实体通用 CRUD。
type BaseMapper[T any, ID any] struct {
	session             StatementSession
	entity              EntityMeta
	dbConfig            DbConfig
	dialect             Dialect
	clock               func() time.Time
	identifierGenerator IdentifierGenerator
	metaObjectHandler   MetaObjectHandler
	primary             ColumnMeta
	softDeleteColumn    ColumnMeta
	hasSoftDelete       bool
	version             ColumnMeta
	hasVersion          bool
	createdAt           ColumnMeta
	hasCreatedAt        bool
	updatedAt           ColumnMeta
	hasUpdatedAt        bool
}

// NewBaseMapper 创建实体通用 Mapper。
func NewBaseMapper[T any, ID any](session StatementSession, entity EntityMeta, options ...BaseMapperOption) (*BaseMapper[T, ID], error) {
	if session == nil {
		return nil, fmt.Errorf("goark-orm: base mapper session is nil")
	}
	opts := baseMapperOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	dialect := opts.dialect
	if dialect == nil {
		if provider, ok := session.(dialectProvider); ok {
			dialect = provider.Dialect()
		}
	}
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	globalConfig := DefaultGlobalConfig()
	if provider, ok := session.(globalConfigProvider); ok {
		globalConfig = provider.GlobalConfig()
	}
	if opts.globalConfigSet {
		globalConfig = opts.globalConfig
	}
	normalizedGlobal, err := normalizeGlobalConfig(globalConfig)
	if err != nil {
		return nil, err
	}
	globalConfig = normalizedGlobal
	identifierGenerator := opts.identifierGenerator
	if identifierGenerator == nil {
		if provider, ok := session.(identifierGeneratorProvider); ok {
			identifierGenerator = provider.IdentifierGenerator()
		}
	}
	if identifierGenerator == nil {
		identifierGenerator = globalConfig.IdentifierGenerator
	}
	if identifierGenerator == nil {
		identifierGenerator = NewDefaultIdentifierGenerator()
	}
	metaObjectHandler := opts.metaObjectHandler
	if metaObjectHandler == nil {
		if provider, ok := session.(metaObjectHandlerProvider); ok {
			metaObjectHandler = provider.MetaObjectHandler()
		}
	}
	if metaObjectHandler == nil {
		metaObjectHandler = globalConfig.MetaObjectHandler
	}
	copied := copyEntityMeta(entity)
	primary, err := singlePrimaryColumn(copied)
	if err != nil {
		return nil, err
	}
	columns, err := collectBaseMapperSemanticColumnsWithDbConfig(copied, globalConfig.DbConfig)
	if err != nil {
		return nil, err
	}
	if reflect.TypeFor[T]().Kind() == reflect.Pointer {
		return nil, fmt.Errorf("goark-orm: BaseMapper entity type must be a struct, got pointer")
	}
	if _, err := quoteIdentifierPath(dialect, effectiveTableName(copied.Table, globalConfig.DbConfig)); err != nil {
		return nil, err
	}
	for _, column := range copied.Columns {
		if _, err := quoteIdentifierPath(dialect, column.ColumnName); err != nil {
			return nil, err
		}
	}
	return &BaseMapper[T, ID]{
		session:             session,
		entity:              copied,
		dbConfig:            globalConfig.DbConfig,
		dialect:             dialect,
		clock:               opts.clock,
		identifierGenerator: identifierGenerator,
		metaObjectHandler:   metaObjectHandler,
		primary:             primary,
		softDeleteColumn:    columns.softDeleteColumn,
		hasSoftDelete:       columns.hasSoftDelete,
		version:             columns.version,
		hasVersion:          columns.hasVersion,
		createdAt:           columns.createdAt,
		hasCreatedAt:        columns.hasCreatedAt,
		updatedAt:           columns.updatedAt,
		hasUpdatedAt:        columns.hasUpdatedAt,
	}, nil
}

// SelectByID 按主键查询单条记录。
func (m *BaseMapper[T, ID]) SelectByID(ctx context.Context, id ID) (*T, error) {
	args := NamedArgs{"id": id}
	sqlText, err := m.selectBaseSQL()
	if err != nil {
		return nil, err
	}
	primary, err := quoteIdentifierPath(m.dialect, m.primary.ColumnName)
	if err != nil {
		return nil, err
	}
	sqlText += " WHERE " + primary + " = #{id}"
	if m.hasSoftDelete {
		live, err := m.softDeleteLiveCondition(baseMapperSoftDeleteLiveArg)
		if err != nil {
			return nil, err
		}
		sqlText += " AND " + live
		args[baseMapperSoftDeleteLiveArg] = logicNotDeleteValue(m.dbConfig)
	}
	var out T
	if err := m.session.QueryOneStatement(ctx, m.statement("SelectByID", StatementCommandSelect, sqlText), args, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SelectBatchIDs 按主键集合查询记录。
func (m *BaseMapper[T, ID]) SelectBatchIDs(ctx context.Context, ids []ID) ([]T, error) {
	wrapper := NewQueryWrapper[T]().In(Field[T]{Column: m.primary.ColumnName}, ids)
	return m.SelectList(ctx, wrapper)
}

// SelectList 按条件查询记录列表。
func (m *BaseMapper[T, ID]) SelectList(ctx context.Context, wrapper *QueryWrapper[T]) ([]T, error) {
	sqlText, args, _, err := m.selectSQL(wrapper, true, 0)
	if err != nil {
		return nil, err
	}
	var out []T
	if err := m.session.QueryStatement(ctx, m.statement("SelectList", StatementCommandSelect, sqlText), args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SelectMaps 按条件查询 map 结果列表。
func (m *BaseMapper[T, ID]) SelectMaps(ctx context.Context, wrapper *QueryWrapper[T]) ([]map[string]any, error) {
	sqlText, args, _, err := m.selectSQL(wrapper, true, 0)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	if err := m.session.QueryStatement(ctx, m.statement("SelectMaps", StatementCommandSelect, sqlText), args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SelectObjs 按条件查询首列结果列表，默认选择实体主键列。
func (m *BaseMapper[T, ID]) SelectObjs(ctx context.Context, wrapper *QueryWrapper[T]) ([]any, error) {
	projection, err := m.selectProjection([]ColumnMeta{m.primary})
	if err != nil {
		return nil, err
	}
	sqlText, args, _, err := m.selectProjectionSQL(projection, wrapper, true, 0)
	if err != nil {
		return nil, err
	}
	var out []any
	if err := m.session.QueryStatement(ctx, m.statement("SelectObjs", StatementCommandSelect, sqlText), args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SelectPage 按条件执行分页查询。
func (m *BaseMapper[T, ID]) SelectPage(ctx context.Context, page PageRequest, wrapper *QueryWrapper[T]) (Page[T], error) {
	page = page.normalized()
	countSQLBase, countArgs, _, err := m.selectSQL(wrapper, false, 0)
	if err != nil {
		return Page[T]{}, err
	}
	result := Page[T]{
		Current: page.Current,
		Size:    page.Size,
	}
	if page.SearchCount {
		countSQL := "SELECT COUNT(*) FROM (" + countSQLBase + ") goark_orm_count"
		if err := m.session.QueryOneStatement(ctx, m.statement("Count", StatementCommandSelect, countSQL), countArgs, &result.Total); err != nil {
			return Page[T]{}, err
		}
		result.Pages = pageCount(result.Total, page.Size)
	}
	listSQL, listArgs, next, err := m.selectSQL(wrapper, true, 0)
	if err != nil {
		return Page[T]{}, err
	}
	if page.Size >= 0 {
		limitName := wrapperArgName(next)
		offsetName := wrapperArgName(next + 1)
		listArgs[limitName] = page.Size
		listArgs[offsetName] = page.offset()
		listSQL = limitOffsetSQL(m.dialect, listSQL, "#{"+limitName+"}", "#{"+offsetName+"}")
	}
	var records []T
	if err := m.session.QueryStatement(ctx, m.statement("SelectPage", StatementCommandSelect, listSQL), listArgs, &records); err != nil {
		return Page[T]{}, err
	}
	result.Records = records
	return result, nil
}

// Count 按条件统计记录数。
func (m *BaseMapper[T, ID]) Count(ctx context.Context, wrapper *QueryWrapper[T]) (int64, error) {
	sqlText, args, _, err := m.selectSQL(wrapper, false, 0)
	if err != nil {
		return 0, err
	}
	countSQL := "SELECT COUNT(*) FROM (" + sqlText + ") goark_orm_count"
	var total int64
	if err := m.session.QueryOneStatement(ctx, m.statement("Count", StatementCommandSelect, countSQL), args, &total); err != nil {
		return 0, err
	}
	return total, nil
}

// SelectCount 是 Count 的 MyBatis-Plus 命名别名。
func (m *BaseMapper[T, ID]) SelectCount(ctx context.Context, wrapper *QueryWrapper[T]) (int64, error) {
	return m.Count(ctx, wrapper)
}

// Insert 插入实体，返回写入结果。
func (m *BaseMapper[T, ID]) Insert(ctx context.Context, entity *T) (Result, error) {
	value, err := entityStructValue(entity)
	if err != nil {
		return Result{}, err
	}
	if err := m.assignInsertID(ctx, value); err != nil {
		return Result{}, err
	}
	if err := m.fillInsertTimeFields(value); err != nil {
		return Result{}, err
	}
	if err := applyMetaObjectHandler(ctx, m.metaObjectHandler, StatementCommandInsert, m.entity, value, nil); err != nil {
		return Result{}, err
	}
	table, columns, fields, err := m.insertColumns(value)
	if err != nil {
		return Result{}, err
	}
	args, err := m.entityArgs(value, fields)
	if err != nil {
		return Result{}, err
	}
	placeholders := make([]string, 0, len(fields))
	for _, field := range fields {
		placeholders = append(placeholders, "#{"+field.FieldName+"}")
	}
	sqlText := "INSERT INTO " + table + " (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	statement := m.statement("Insert", StatementCommandInsert, sqlText)
	statement.ParameterType = m.entity.TypeName
	statement.UseGeneratedKeys = m.effectiveColumnIDType(m.primary) == IDTypeAuto
	statement.KeyProperty = m.primary.FieldName
	return m.session.ExecStatement(ctx, statement, args)
}

func (m *BaseMapper[T, ID]) assignInsertID(ctx context.Context, entity reflect.Value) error {
	idType := m.effectiveColumnIDType(m.primary)
	if idType != IDTypeAssignID && idType != IDTypeAssignUUID {
		return nil
	}
	field := entity.FieldByName(m.primary.FieldName)
	if !field.IsValid() {
		return fmt.Errorf("goark-orm: entity %s missing field %s", entity.Type().Name(), m.primary.FieldName)
	}
	if !field.CanSet() {
		return fmt.Errorf("goark-orm: entity field %s.%s is not settable", entity.Type().Name(), m.primary.FieldName)
	}
	if !field.IsZero() {
		return nil
	}
	if m.identifierGenerator == nil {
		return fmt.Errorf("goark-orm: identifier generator is nil")
	}
	switch idType {
	case IDTypeAssignID:
		id, err := m.identifierGenerator.NextID(ctx, m.entity, m.primary)
		if err != nil {
			return err
		}
		return setReflectField(field, id)
	case IDTypeAssignUUID:
		uuid, err := m.identifierGenerator.NextUUID(ctx, m.entity, m.primary)
		if err != nil {
			return err
		}
		return setReflectField(field, uuid)
	default:
		return nil
	}
}

// SaveOrUpdate 根据主键零值判断插入或按主键更新。
func (m *BaseMapper[T, ID]) SaveOrUpdate(ctx context.Context, entity *T) (Result, error) {
	value, err := entityStructValue(entity)
	if err != nil {
		return Result{}, err
	}
	primaryValue, err := fieldValue(value, m.primary)
	if err != nil {
		return Result{}, err
	}
	if isZeroValue(primaryValue) {
		return m.Insert(ctx, entity)
	}
	rows, err := m.UpdateByID(ctx, entity)
	if err != nil {
		return Result{}, err
	}
	if rows > 0 {
		return Result{RowsAffected: rows}, nil
	}
	return m.Insert(ctx, entity)
}

// UpdateByID 按主键更新实体非主键字段。
func (m *BaseMapper[T, ID]) UpdateByID(ctx context.Context, entity *T) (int64, error) {
	value, err := entityStructValue(entity)
	if err != nil {
		return 0, err
	}
	if err := m.fillUpdateTimeFields(value); err != nil {
		return 0, err
	}
	if err := applyMetaObjectHandler(ctx, m.metaObjectHandler, StatementCommandUpdate, m.entity, value, nil); err != nil {
		return 0, err
	}
	var versionValue any
	if m.hasVersion {
		versionValue, err = fieldValue(value, m.version)
		if err != nil {
			return 0, err
		}
	}
	sets, fields, err := m.updateSetColumns(value, true)
	if err != nil {
		return 0, err
	}
	args, err := m.entityArgs(value, fields)
	if err != nil {
		return 0, err
	}
	pkValue, err := fieldValue(value, m.primary)
	if err != nil {
		return 0, err
	}
	args[baseMapperPrimaryKeyArg] = pkValue
	if m.hasVersion {
		args[baseMapperVersionOldArg] = versionValue
	}
	table, err := m.quotedTable()
	if err != nil {
		return 0, err
	}
	primary, err := quoteIdentifierPath(m.dialect, m.primary.ColumnName)
	if err != nil {
		return 0, err
	}
	whereParts := []string{primary + " = #{" + baseMapperPrimaryKeyArg + "}"}
	if m.hasVersion {
		version, err := quoteIdentifierPath(m.dialect, m.version.ColumnName)
		if err != nil {
			return 0, err
		}
		whereParts = append(whereParts, version+" = #{"+baseMapperVersionOldArg+"}")
	}
	if m.hasSoftDelete {
		live, err := m.softDeleteLiveCondition(baseMapperSoftDeleteLiveArg)
		if err != nil {
			return 0, err
		}
		whereParts = append(whereParts, live)
		args[baseMapperSoftDeleteLiveArg] = logicNotDeleteValue(m.dbConfig)
	}
	sqlText := "UPDATE " + table + " SET " + strings.Join(sets, ", ") + " WHERE " + strings.Join(whereParts, " AND ")
	statement := m.statement("UpdateByID", StatementCommandUpdate, sqlText)
	statement.ParameterType = m.entity.TypeName
	result, err := m.session.ExecStatement(ctx, statement, args)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// Update 按条件更新实体非主键字段，空条件会被拒绝。
func (m *BaseMapper[T, ID]) Update(ctx context.Context, entity *T, wrapper *QueryWrapper[T]) (int64, error) {
	if wrapper.Empty() {
		return 0, fmt.Errorf("goark-orm: update wrapper must contain conditions")
	}
	value, err := entityStructValue(entity)
	if err != nil {
		return 0, err
	}
	if err := m.fillUpdateTimeFields(value); err != nil {
		return 0, err
	}
	if err := applyMetaObjectHandler(ctx, m.metaObjectHandler, StatementCommandUpdate, m.entity, value, nil); err != nil {
		return 0, err
	}
	sets, fields, err := m.updateSetColumns(value, false)
	if err != nil {
		return 0, err
	}
	args, err := m.entityArgs(value, fields)
	if err != nil {
		return 0, err
	}
	rendered, err := wrapper.build(m.dialect, len(args))
	if err != nil {
		return 0, err
	}
	whereParts := []string{rendered.WhereSQL}
	if m.hasSoftDelete {
		liveName := wrapperArgName(rendered.Next)
		live, err := m.softDeleteLiveCondition(liveName)
		if err != nil {
			return 0, err
		}
		whereParts = append(whereParts, live)
		rendered.Args[liveName] = logicNotDeleteValue(m.dbConfig)
	}
	for key, value := range rendered.Args {
		args[key] = value
	}
	table, err := m.quotedTable()
	if err != nil {
		return 0, err
	}
	sqlText := "UPDATE " + table + " SET " + strings.Join(sets, ", ") + " WHERE " + strings.Join(whereParts, " AND ")
	if rendered.LastSQL != "" {
		sqlText += " " + rendered.LastSQL
	}
	statement := m.statement("Update", StatementCommandUpdate, sqlText)
	statement.ParameterType = m.entity.TypeName
	result, err := m.session.ExecStatement(ctx, statement, args)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// UpdateWithWrapper 按 UpdateWrapper 的 SET 和 WHERE 执行局部更新。
func (m *BaseMapper[T, ID]) UpdateWithWrapper(ctx context.Context, wrapper *UpdateWrapper[T]) (int64, error) {
	if err := requireUpdateWrapper(wrapper); err != nil {
		return 0, err
	}
	rendered, err := wrapper.build(m.dialect, 0)
	if err != nil {
		return 0, err
	}
	whereParts := []string{rendered.WhereSQL}
	if m.hasSoftDelete {
		liveName := wrapperArgName(rendered.Next)
		live, err := m.softDeleteLiveCondition(liveName)
		if err != nil {
			return 0, err
		}
		whereParts = append(whereParts, live)
		rendered.Args[liveName] = logicNotDeleteValue(m.dbConfig)
	}
	table, err := m.quotedTable()
	if err != nil {
		return 0, err
	}
	sqlText := "UPDATE " + table + " SET " + rendered.SetSQL + " WHERE " + strings.Join(whereParts, " AND ")
	if rendered.LastSQL != "" {
		sqlText += " " + rendered.LastSQL
	}
	result, err := m.session.ExecStatement(ctx, m.statement("UpdateWithWrapper", StatementCommandUpdate, sqlText), rendered.Args)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// DeleteByID 按主键删除记录。
func (m *BaseMapper[T, ID]) DeleteByID(ctx context.Context, id ID) (int64, error) {
	if m.hasSoftDelete {
		return m.softDeleteByID(ctx, id)
	}
	table, err := m.quotedTable()
	if err != nil {
		return 0, err
	}
	primary, err := quoteIdentifierPath(m.dialect, m.primary.ColumnName)
	if err != nil {
		return 0, err
	}
	sqlText := "DELETE FROM " + table + " WHERE " + primary + " = #{id}"
	result, err := m.session.ExecStatement(ctx, m.statement("DeleteByID", StatementCommandDelete, sqlText), NamedArgs{"id": id})
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// DeleteBatchIDs 按主键集合删除记录。
func (m *BaseMapper[T, ID]) DeleteBatchIDs(ctx context.Context, ids []ID) (int64, error) {
	return m.Delete(ctx, NewQueryWrapper[T]().In(Field[T]{Column: m.primary.ColumnName}, ids))
}

// Delete 按条件删除记录，空条件会被拒绝。
func (m *BaseMapper[T, ID]) Delete(ctx context.Context, wrapper *QueryWrapper[T]) (int64, error) {
	if wrapper.Empty() {
		return 0, fmt.Errorf("goark-orm: delete wrapper must contain conditions")
	}
	if m.hasSoftDelete {
		return m.softDeleteRows(ctx, wrapper)
	}
	rendered, err := wrapper.build(m.dialect, 0)
	if err != nil {
		return 0, err
	}
	table, err := m.quotedTable()
	if err != nil {
		return 0, err
	}
	sqlText := "DELETE FROM " + table + " WHERE " + rendered.WhereSQL
	if rendered.LastSQL != "" {
		sqlText += " " + rendered.LastSQL
	}
	result, err := m.session.ExecStatement(ctx, m.statement("Delete", StatementCommandDelete, sqlText), rendered.Args)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

func (m *BaseMapper[T, ID]) selectSQL(wrapper *QueryWrapper[T], includeOrder bool, start int) (string, NamedArgs, int, error) {
	projection, err := m.selectProjection(m.defaultSelectableColumns())
	if err != nil {
		return "", nil, start, err
	}
	if len(wrapperSelects(wrapper)) > 0 {
		projection, err = m.selectFieldsProjection(wrapperSelects(wrapper))
		if err != nil {
			return "", nil, start, err
		}
	}
	return m.selectProjectionSQL(projection, wrapper, includeOrder, start)
}

func (m *BaseMapper[T, ID]) selectProjectionSQL(projection string, wrapper *QueryWrapper[T], includeOrder bool, start int) (string, NamedArgs, int, error) {
	table, err := m.quotedTable()
	if err != nil {
		return "", nil, start, err
	}
	sqlText := "SELECT " + projection + " FROM " + table
	rendered, err := wrapper.build(m.dialect, start)
	if err != nil {
		return "", nil, start, err
	}
	conditions := make([]string, 0, 2)
	if rendered.WhereSQL != "" {
		conditions = append(conditions, rendered.WhereSQL)
	}
	if m.hasSoftDelete {
		liveName := wrapperArgName(rendered.Next)
		live, err := m.softDeleteLiveCondition(liveName)
		if err != nil {
			return "", nil, start, err
		}
		conditions = append(conditions, live)
		rendered.Args[liveName] = logicNotDeleteValue(m.dbConfig)
		rendered.Next++
	}
	if len(conditions) > 0 {
		sqlText += " WHERE " + strings.Join(conditions, " AND ")
	}
	if rendered.GroupSQL != "" {
		sqlText += " " + rendered.GroupSQL
	}
	if rendered.HavingSQL != "" {
		sqlText += " " + rendered.HavingSQL
	}
	if includeOrder && rendered.OrderSQL != "" {
		sqlText += " " + rendered.OrderSQL
	}
	if includeOrder && rendered.LastSQL != "" {
		sqlText += " " + rendered.LastSQL
	}
	return sqlText, rendered.Args, rendered.Next, nil
}

func (m *BaseMapper[T, ID]) selectBaseSQL() (string, error) {
	table, err := m.quotedTable()
	if err != nil {
		return "", err
	}
	columns := make([]string, 0, len(m.entity.Columns))
	for _, column := range m.defaultSelectableColumns() {
		quoted, err := quoteIdentifierPath(m.dialect, column.ColumnName)
		if err != nil {
			return "", err
		}
		columns = append(columns, quoted)
	}
	return "SELECT " + strings.Join(columns, ", ") + " FROM " + table, nil
}

func (m *BaseMapper[T, ID]) selectProjection(columns []ColumnMeta) (string, error) {
	quotedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted, err := quoteIdentifierPath(m.dialect, column.ColumnName)
		if err != nil {
			return "", err
		}
		quotedColumns = append(quotedColumns, quoted)
	}
	if len(quotedColumns) == 0 {
		return "", fmt.Errorf("goark-orm: entity %s has no selectable columns", m.entity.TypeName)
	}
	return strings.Join(quotedColumns, ", "), nil
}

func (m *BaseMapper[T, ID]) selectFieldsProjection(fields []Field[T]) (string, error) {
	quotedColumns := make([]string, 0, len(fields))
	for _, field := range fields {
		quoted, err := quoteIdentifierPath(m.dialect, field.Column)
		if err != nil {
			return "", err
		}
		quotedColumns = append(quotedColumns, quoted)
	}
	if len(quotedColumns) == 0 {
		return "", fmt.Errorf("goark-orm: wrapper select projection is empty")
	}
	return strings.Join(quotedColumns, ", "), nil
}

func wrapperSelects[T any](wrapper *QueryWrapper[T]) []Field[T] {
	if wrapper == nil || len(wrapper.selects) == 0 {
		return nil
	}
	return append([]Field[T](nil), wrapper.selects...)
}

func (m *BaseMapper[T, ID]) defaultSelectableColumns() []ColumnMeta {
	columns := make([]ColumnMeta, 0, len(m.entity.Columns))
	for _, column := range m.entity.Columns {
		if column.SelectDisabled {
			continue
		}
		columns = append(columns, column)
	}
	return columns
}

func (m *BaseMapper[T, ID]) insertColumns(entity reflect.Value) (string, []string, []ColumnMeta, error) {
	table, err := m.quotedTable()
	if err != nil {
		return "", nil, nil, err
	}
	columns := make([]string, 0, len(m.entity.Columns))
	fields := make([]ColumnMeta, 0, len(m.entity.Columns))
	for _, column := range m.entity.Columns {
		if column.PrimaryKey && m.effectiveColumnIDType(column) == IDTypeAuto {
			continue
		}
		value, err := fieldValue(entity, column)
		if err != nil {
			return "", nil, nil, err
		}
		strategy := effectiveFieldStrategy(column.InsertStrategy, m.dbConfig.InsertStrategy)
		if !fieldIncludedByStrategy(value, strategy) {
			continue
		}
		quoted, err := quoteIdentifierPath(m.dialect, column.ColumnName)
		if err != nil {
			return "", nil, nil, err
		}
		columns = append(columns, quoted)
		fields = append(fields, column)
	}
	if len(fields) == 0 {
		return "", nil, nil, fmt.Errorf("goark-orm: entity %s has no insertable columns", m.entity.TypeName)
	}
	return table, columns, fields, nil
}

func (m *BaseMapper[T, ID]) updateSetColumns(entity reflect.Value, incrementVersion bool) ([]string, []ColumnMeta, error) {
	sets := make([]string, 0, len(m.entity.Columns))
	fields := make([]ColumnMeta, 0, len(m.entity.Columns))
	for _, column := range m.entity.Columns {
		if column.PrimaryKey || column.Version || column.SoftDelete || column.CreatedAt {
			continue
		}
		value, err := fieldValue(entity, column)
		if err != nil {
			return nil, nil, err
		}
		strategy := effectiveFieldStrategy(column.UpdateStrategy, m.dbConfig.UpdateStrategy)
		if !fieldIncludedByStrategy(value, strategy) {
			continue
		}
		quoted, err := quoteIdentifierPath(m.dialect, column.ColumnName)
		if err != nil {
			return nil, nil, err
		}
		sets = append(sets, quoted+" = #{"+column.FieldName+"}")
		fields = append(fields, column)
	}
	if incrementVersion && m.hasVersion {
		version, err := quoteIdentifierPath(m.dialect, m.version.ColumnName)
		if err != nil {
			return nil, nil, err
		}
		sets = append(sets, version+" = "+version+" + 1")
	}
	if len(sets) == 0 {
		return nil, nil, fmt.Errorf("goark-orm: entity %s has no updatable columns", m.entity.TypeName)
	}
	return sets, fields, nil
}

func (m *BaseMapper[T, ID]) entityArgs(value reflect.Value, columns []ColumnMeta) (NamedArgs, error) {
	args := make(NamedArgs, len(columns))
	for _, column := range columns {
		value, err := fieldValue(value, column)
		if err != nil {
			return nil, err
		}
		args[column.FieldName] = value
	}
	return args, nil
}

func (m *BaseMapper[T, ID]) statement(id string, command StatementCommand, sqlText string) StatementMeta {
	namespace := "goark-orm.base." + m.entity.TypeName
	return StatementMeta{
		ID:         id,
		Namespace:  namespace,
		FullName:   namespace + "." + id,
		Command:    command,
		Source:     StatementSourceBase,
		SQL:        sqlText,
		ResultType: m.entity.TypeName,
	}
}

func singlePrimaryColumn(entity EntityMeta) (ColumnMeta, error) {
	if entity.TypeName == "" {
		return ColumnMeta{}, fmt.Errorf("goark-orm: entity type name is required")
	}
	if entity.Table == "" {
		return ColumnMeta{}, fmt.Errorf("goark-orm: entity %s table is required", entity.TypeName)
	}
	var primary ColumnMeta
	count := 0
	for _, column := range entity.Columns {
		if column.PrimaryKey {
			primary = column
			count++
		}
	}
	if count == 0 {
		return ColumnMeta{}, fmt.Errorf("goark-orm: entity %s missing primary-key field", entity.TypeName)
	}
	if count > 1 {
		return ColumnMeta{}, fmt.Errorf("goark-orm: entity %s has composite primary key; BaseMapper V1 supports one primary key", entity.TypeName)
	}
	return primary, nil
}

func entityStructValue(entity any) (reflect.Value, error) {
	if entity == nil {
		return reflect.Value{}, fmt.Errorf("goark-orm: entity is nil")
	}
	value := reflect.ValueOf(entity)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return reflect.Value{}, fmt.Errorf("goark-orm: entity must be non-nil pointer")
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("goark-orm: entity must point to struct")
	}
	return value, nil
}

func fieldValue(entity reflect.Value, column ColumnMeta) (any, error) {
	field := entity.FieldByName(column.FieldName)
	if !field.IsValid() {
		return nil, fmt.Errorf("goark-orm: entity %s missing field %s", entity.Type().Name(), column.FieldName)
	}
	if !field.CanInterface() {
		return nil, fmt.Errorf("goark-orm: entity field %s.%s is not exported", entity.Type().Name(), column.FieldName)
	}
	return field.Interface(), nil
}

type baseMapperSemanticColumns struct {
	softDeleteColumn ColumnMeta
	hasSoftDelete    bool
	version          ColumnMeta
	hasVersion       bool
	createdAt        ColumnMeta
	hasCreatedAt     bool
	updatedAt        ColumnMeta
	hasUpdatedAt     bool
}

func collectBaseMapperSemanticColumns(entity EntityMeta) (baseMapperSemanticColumns, error) {
	return collectBaseMapperSemanticColumnsWithDbConfig(entity, DbConfig{})
}

func collectBaseMapperSemanticColumnsWithDbConfig(entity EntityMeta, dbConfig DbConfig) (baseMapperSemanticColumns, error) {
	softDelete, hasSoftDelete, err := singleSemanticColumn(entity, "soft-delete", func(column ColumnMeta) bool {
		return column.SoftDelete
	})
	if err != nil {
		return baseMapperSemanticColumns{}, err
	}
	if !hasSoftDelete && strings.TrimSpace(dbConfig.LogicDeleteField) != "" {
		softDelete, hasSoftDelete, err = singleSemanticColumn(entity, "logic-delete", func(column ColumnMeta) bool {
			return columnMatchesConfiguredLogicDelete(column, dbConfig.LogicDeleteField)
		})
		if err != nil {
			return baseMapperSemanticColumns{}, err
		}
	}
	version, hasVersion, err := singleSemanticColumn(entity, "version", func(column ColumnMeta) bool {
		return column.Version
	})
	if err != nil {
		return baseMapperSemanticColumns{}, err
	}
	createdAt, hasCreatedAt, err := singleSemanticColumn(entity, "created-at", func(column ColumnMeta) bool {
		return column.CreatedAt
	})
	if err != nil {
		return baseMapperSemanticColumns{}, err
	}
	updatedAt, hasUpdatedAt, err := singleSemanticColumn(entity, "updated-at", func(column ColumnMeta) bool {
		return column.UpdatedAt
	})
	if err != nil {
		return baseMapperSemanticColumns{}, err
	}
	return baseMapperSemanticColumns{
		softDeleteColumn: softDelete,
		hasSoftDelete:    hasSoftDelete,
		version:          version,
		hasVersion:       hasVersion,
		createdAt:        createdAt,
		hasCreatedAt:     hasCreatedAt,
		updatedAt:        updatedAt,
		hasUpdatedAt:     hasUpdatedAt,
	}, nil
}

func singleSemanticColumn(entity EntityMeta, name string, match func(ColumnMeta) bool) (ColumnMeta, bool, error) {
	var out ColumnMeta
	count := 0
	for _, column := range entity.Columns {
		if !match(column) {
			continue
		}
		out = column
		count++
	}
	if count > 1 {
		return ColumnMeta{}, false, fmt.Errorf("goark-orm: entity %s has multiple %s fields", entity.TypeName, name)
	}
	return out, count == 1, nil
}

func (m *BaseMapper[T, ID]) softDeleteLiveCondition(argName string) (string, error) {
	column, err := quoteIdentifierPath(m.dialect, m.softDeleteColumn.ColumnName)
	if err != nil {
		return "", err
	}
	return column + " = #{" + argName + "}", nil
}

func (m *BaseMapper[T, ID]) softDeleteByID(ctx context.Context, id ID) (int64, error) {
	table, err := m.quotedTable()
	if err != nil {
		return 0, err
	}
	primary, err := quoteIdentifierPath(m.dialect, m.primary.ColumnName)
	if err != nil {
		return 0, err
	}
	deleted, err := quoteIdentifierPath(m.dialect, m.softDeleteColumn.ColumnName)
	if err != nil {
		return 0, err
	}
	live, err := m.softDeleteLiveCondition(baseMapperSoftDeleteLiveArg)
	if err != nil {
		return 0, err
	}
	args := NamedArgs{
		baseMapperSoftDeleteDeleteArg: logicDeleteValue(m.dbConfig),
		"id":                          id,
		baseMapperSoftDeleteLiveArg:   logicNotDeleteValue(m.dbConfig),
	}
	sqlText := "UPDATE " + table + " SET " + deleted + " = #{" + baseMapperSoftDeleteDeleteArg + "} WHERE " + primary + " = #{id} AND " + live
	result, err := m.session.ExecStatement(ctx, m.statement("DeleteByID", StatementCommandUpdate, sqlText), args)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

func (m *BaseMapper[T, ID]) softDeleteRows(ctx context.Context, wrapper *QueryWrapper[T]) (int64, error) {
	rendered, err := wrapper.build(m.dialect, 0)
	if err != nil {
		return 0, err
	}
	table, err := m.quotedTable()
	if err != nil {
		return 0, err
	}
	deleted, err := quoteIdentifierPath(m.dialect, m.softDeleteColumn.ColumnName)
	if err != nil {
		return 0, err
	}
	liveName := wrapperArgName(rendered.Next)
	live, err := m.softDeleteLiveCondition(liveName)
	if err != nil {
		return 0, err
	}
	rendered.Args[baseMapperSoftDeleteDeleteArg] = logicDeleteValue(m.dbConfig)
	rendered.Args[liveName] = logicNotDeleteValue(m.dbConfig)
	sqlText := "UPDATE " + table + " SET " + deleted + " = #{" + baseMapperSoftDeleteDeleteArg + "} WHERE " + rendered.WhereSQL + " AND " + live
	if rendered.LastSQL != "" {
		sqlText += " " + rendered.LastSQL
	}
	result, err := m.session.ExecStatement(ctx, m.statement("Delete", StatementCommandUpdate, sqlText), rendered.Args)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

func (m *BaseMapper[T, ID]) fillInsertTimeFields(entity reflect.Value) error {
	if !m.hasCreatedAt && !m.hasUpdatedAt {
		return nil
	}
	now := m.now()
	if m.hasCreatedAt {
		if err := setTimeField(entity, m.createdAt, now, false); err != nil {
			return err
		}
	}
	if m.hasUpdatedAt {
		if err := setTimeField(entity, m.updatedAt, now, false); err != nil {
			return err
		}
	}
	return nil
}

func (m *BaseMapper[T, ID]) quotedTable() (string, error) {
	return quoteIdentifierPath(m.dialect, effectiveTableName(m.entity.Table, m.dbConfig))
}

func (m *BaseMapper[T, ID]) effectiveColumnIDType(column ColumnMeta) IDType {
	return effectiveColumnIDTypeWithDbConfig(column, m.dbConfig)
}

func (m *BaseMapper[T, ID]) fillUpdateTimeFields(entity reflect.Value) error {
	if !m.hasUpdatedAt {
		return nil
	}
	return setTimeField(entity, m.updatedAt, m.now(), true)
}

func (m *BaseMapper[T, ID]) now() time.Time {
	if m.clock != nil {
		return m.clock()
	}
	return time.Now()
}

func setTimeField(entity reflect.Value, column ColumnMeta, value time.Time, overwrite bool) error {
	field := entity.FieldByName(column.FieldName)
	if !field.IsValid() {
		return fmt.Errorf("goark-orm: entity %s missing field %s", entity.Type().Name(), column.FieldName)
	}
	if !field.CanSet() {
		return fmt.Errorf("goark-orm: entity field %s.%s is not settable", entity.Type().Name(), column.FieldName)
	}
	if !overwrite && !field.IsZero() {
		return nil
	}
	timeValue := reflect.ValueOf(value)
	if timeValue.Type().AssignableTo(field.Type()) {
		field.Set(timeValue)
		return nil
	}
	if field.Kind() == reflect.Pointer && timeValue.Type().AssignableTo(field.Type().Elem()) {
		pointer := reflect.New(field.Type().Elem())
		pointer.Elem().Set(timeValue)
		field.Set(pointer)
		return nil
	}
	return fmt.Errorf("goark-orm: auto time field %s.%s must be time.Time or *time.Time", entity.Type().Name(), column.FieldName)
}

func limitOffsetSQL(dialect Dialect, query string, limitPlaceholder string, offsetPlaceholder string) string {
	if custom, ok := dialect.(interface {
		LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string
	}); ok {
		return custom.LimitOffsetSQL(query, limitPlaceholder, offsetPlaceholder)
	}
	return query + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
}
