package runtime

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// EntitySemanticInterceptorOption 配置实体语义拦截器。
type EntitySemanticInterceptorOption func(*entitySemanticInterceptor)

type entitySemanticInterceptor struct {
	registry *Registry
	clock    func() time.Time
}

// NewEntitySemanticInterceptor 创建实体语义拦截器。
func NewEntitySemanticInterceptor(registry *Registry, options ...EntitySemanticInterceptorOption) StatementInterceptor {
	interceptor := &entitySemanticInterceptor{
		registry: registry,
		clock:    time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(interceptor)
		}
	}
	return interceptor
}

// WithEntitySemanticClock 配置自动填充时间使用的时钟。
func WithEntitySemanticClock(clock func() time.Time) EntitySemanticInterceptorOption {
	return func(interceptor *entitySemanticInterceptor) {
		if clock != nil {
			interceptor.clock = clock
		}
	}
}

func (i *entitySemanticInterceptor) Intercept(ctx context.Context, invocation *StatementInvocation) error {
	_ = ctx
	statement := invocation.Statement()
	if statement == nil {
		return fmt.Errorf("goark-orm: statement runtime is nil")
	}
	if i == nil || i.registry == nil || statement.Meta.Source == StatementSourceBase {
		return invocation.Proceed(ctx)
	}
	if StatementInterceptorIgnored(statement.Meta, InterceptorNameEntitySemantic) {
		return invocation.Proceed(ctx)
	}
	entity, ok := i.statementEntity(statement.Meta)
	if !ok {
		return invocation.Proceed(ctx)
	}
	columns, err := collectBaseMapperSemanticColumnsWithDbConfig(entity, statement.Configuration.GlobalConfig.DbConfig)
	if err != nil {
		return err
	}
	if err := i.applyAutoFill(statement, entity, columns); err != nil {
		return err
	}
	if err := i.applyOptimisticLock(statement, entity, columns); err != nil {
		return err
	}
	if err := i.applyLogicDelete(statement, entity, columns); err != nil {
		return err
	}
	return invocation.Proceed(ctx)
}

func (i *entitySemanticInterceptor) statementEntity(statement StatementMeta) (EntityMeta, bool) {
	return statementEntityFromRegistry(i.registry, statement)
}

func (i *entitySemanticInterceptor) applyAutoFill(statement *StatementRuntime, entity EntityMeta, columns baseMapperSemanticColumns) error {
	switch statement.Meta.Command {
	case StatementCommandInsert:
		if !columns.hasCreatedAt && !columns.hasUpdatedAt {
			return nil
		}
		now := i.now()
		statement.ensureArgs()
		value, hasEntity := entityValueFromArgs(statement.Args, entity)
		if hasEntity && columns.hasCreatedAt {
			if err := setTimeField(value, columns.createdAt, now, false); err != nil {
				return err
			}
			syncEntityFieldArg(statement.Args, value, columns.createdAt)
		} else if columns.hasCreatedAt {
			setTimeArg(statement.Args, columns.createdAt.FieldName, now, false)
		}
		if hasEntity && columns.hasUpdatedAt {
			if err := setTimeField(value, columns.updatedAt, now, false); err != nil {
				return err
			}
			syncEntityFieldArg(statement.Args, value, columns.updatedAt)
		} else if columns.hasUpdatedAt {
			setTimeArg(statement.Args, columns.updatedAt.FieldName, now, false)
		}
	case StatementCommandUpdate:
		if !columns.hasUpdatedAt {
			return nil
		}
		now := i.now()
		statement.ensureArgs()
		if value, ok := entityValueFromArgs(statement.Args, entity); ok {
			if err := setTimeField(value, columns.updatedAt, now, true); err != nil {
				return err
			}
			syncEntityFieldArg(statement.Args, value, columns.updatedAt)
			return nil
		}
		setTimeArg(statement.Args, columns.updatedAt.FieldName, now, true)
	}
	return nil
}

func (i *entitySemanticInterceptor) applyLogicDelete(statement *StatementRuntime, entity EntityMeta, columns baseMapperSemanticColumns) error {
	if !columns.hasSoftDelete || statement.Meta.Command == StatementCommandInsert {
		return nil
	}
	if logicDeleteAlreadyConstrained(statement, columns.softDeleteColumn.ColumnName) {
		return nil
	}
	statement.ensureArgs()
	deletedColumn, err := quoteIdentifierPath(statement.Dialect, columns.softDeleteColumn.ColumnName)
	if err != nil {
		return err
	}
	if statement.Meta.Command == StatementCommandDelete {
		where, ok := simpleDeleteWhere(statement.SQL)
		if ok {
			table, err := quoteIdentifierPath(statement.Dialect, effectiveTableName(entity.Table, statement.Configuration.GlobalConfig.DbConfig))
			if err != nil {
				return err
			}
			deleteArg := nextSQLArgName(statement.Args, baseMapperSoftDeleteDeleteArg)
			liveArg := nextSQLArgName(statement.Args, baseMapperSoftDeleteLiveArg)
			statement.Args[deleteArg] = logicDeleteValue(statement.Configuration.GlobalConfig.DbConfig)
			statement.Args[liveArg] = logicNotDeleteValue(statement.Configuration.GlobalConfig.DbConfig)
			statement.SQL = "UPDATE " + table + " SET " + deletedColumn + " = #{" + deleteArg + "} WHERE " + where + " AND " + deletedColumn + " = #{" + liveArg + "}"
			statement.Meta.Command = StatementCommandUpdate
			return nil
		}
	}
	liveArg := nextSQLArgName(statement.Args, baseMapperSoftDeleteLiveArg)
	statement.Args[liveArg] = logicNotDeleteValue(statement.Configuration.GlobalConfig.DbConfig)
	statement.SQL = appendSQLCondition(statement.SQL, deletedColumn+" = #{"+liveArg+"}")
	return nil
}

func logicDeleteAlreadyConstrained(statement *StatementRuntime, column string) bool {
	if statement == nil {
		return false
	}
	switch statement.Meta.Command {
	case StatementCommandSelect:
		return sqlWhereContainsColumn(statement.SQL, column)
	case StatementCommandUpdate:
		return sqlSetContainsColumn(statement.SQL, column) || sqlWhereContainsColumn(statement.SQL, column)
	default:
		return false
	}
}

func (i *entitySemanticInterceptor) applyOptimisticLock(statement *StatementRuntime, entity EntityMeta, columns baseMapperSemanticColumns) error {
	if !columns.hasVersion || statement.Meta.Command != StatementCommandUpdate {
		return nil
	}
	versionValue, ok, err := versionValueFromArgs(statement.Args, entity, columns.version)
	if err != nil || !ok {
		return err
	}
	statement.ensureArgs()
	versionColumn, err := quoteIdentifierPath(statement.Dialect, columns.version.ColumnName)
	if err != nil {
		return err
	}
	originalSQL := statement.SQL
	if !sqlSetContainsColumn(originalSQL, columns.version.ColumnName) {
		rewritten, err := appendUpdateAssignment(statement.SQL, versionColumn+" = "+versionColumn+" + 1")
		if err != nil {
			return err
		}
		statement.SQL = rewritten
	}
	if !sqlWhereContainsColumn(originalSQL, columns.version.ColumnName) {
		versionArg := nextSQLArgName(statement.Args, baseMapperVersionOldArg)
		statement.Args[versionArg] = versionValue
		statement.SQL = appendSQLCondition(statement.SQL, versionColumn+" = #{"+versionArg+"}")
	}
	return nil
}

func (i *entitySemanticInterceptor) now() time.Time {
	if i != nil && i.clock != nil {
		return i.clock()
	}
	return time.Now()
}

func entityValueFromArgs(args NamedArgs, entity EntityMeta) (reflect.Value, bool) {
	for _, value := range args {
		current := reflect.ValueOf(value)
		if !current.IsValid() {
			continue
		}
		for current.Kind() == reflect.Interface || current.Kind() == reflect.Pointer {
			if current.IsNil() {
				break
			}
			current = current.Elem()
		}
		if current.IsValid() && current.Kind() == reflect.Struct && current.Type().Name() == entity.TypeName && current.CanSet() {
			return current, true
		}
	}
	return reflect.Value{}, false
}

func syncEntityFieldArg(args NamedArgs, entity reflect.Value, column ColumnMeta) {
	value, err := fieldValue(entity, column)
	if err == nil {
		setMetaObjectArgs(args, column, value)
	}
}

func setTimeArg(args NamedArgs, name string, value time.Time, overwrite bool) {
	if !overwrite {
		if existing, ok := args[name]; ok && !isZeroValue(existing) {
			return
		}
	}
	args[name] = value
}

func isZeroValue(value any) bool {
	if value == nil {
		return true
	}
	current := reflect.ValueOf(value)
	return !current.IsValid() || current.IsZero()
}

func versionValueFromArgs(args NamedArgs, entity EntityMeta, column ColumnMeta) (any, bool, error) {
	if value, ok := args[column.FieldName]; ok {
		return value, true, nil
	}
	if value, ok := entityValueFromArgs(args, entity); ok {
		out, err := fieldValue(value, column)
		if err != nil {
			return nil, false, err
		}
		return out, true, nil
	}
	return nil, false, nil
}

func simpleDeleteWhere(query string) (string, bool) {
	deleteIndex := findSQLKeyword(query, "delete")
	if deleteIndex < 0 || deleteIndex != skipSQLSpacesAndComments(query, 0) || findSQLKeyword(query, "from") < 0 {
		return "", false
	}
	whereIndex := findSQLKeyword(query, "where")
	if whereIndex < 0 {
		return "", false
	}
	where := strings.TrimSpace(query[whereIndex+len("where"):])
	return where, where != ""
}

func appendUpdateAssignment(query string, assignment string) (string, error) {
	setIndex := findSQLKeyword(query, "set")
	if setIndex < 0 {
		return "", fmt.Errorf("goark-orm: update statement has no SET clause")
	}
	afterSet := setIndex + len("set")
	whereRelative := findSQLKeyword(query[afterSet:], "where")
	if whereRelative >= 0 {
		whereIndex := afterSet + whereRelative
		return strings.TrimRight(query[:whereIndex], " \t\r\n") + ", " + assignment + " " + strings.TrimSpace(query[whereIndex:]), nil
	}
	return strings.TrimRight(query, " \t\r\n") + ", " + assignment, nil
}

func sqlSetContainsColumn(query string, column string) bool {
	setIndex := findSQLKeyword(query, "set")
	if setIndex < 0 {
		return false
	}
	start := setIndex + len("set")
	end := len(query)
	if whereRelative := findSQLKeyword(query[start:], "where"); whereRelative >= 0 {
		end = start + whereRelative
	}
	return containsSQLColumn(query[start:end], column)
}

func sqlWhereContainsColumn(query string, column string) bool {
	whereIndex := findSQLKeyword(query, "where")
	if whereIndex < 0 {
		return false
	}
	where, _ := splitSQLConditionTail(query[whereIndex+len("where"):])
	return containsSQLColumn(where, column)
}

func containsSQLColumn(query string, column string) bool {
	target := normalizeColumnKey(lastSQLIdentifierPart(column))
	if target == "" {
		return false
	}
	for index := 0; index < len(query); {
		if next, ok := skipSQLComment(query, index); ok {
			index = next
			continue
		}
		switch query[index] {
		case '\'':
			index = skipSQLSingleQuoted(query, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(query, index); ok {
				index = next
				continue
			}
		case '(':
			if sqlParenStartsSubquery(query, index) {
				next, ok := findClosingSQLParen(query, index)
				if !ok {
					return false
				}
				index = next + 1
				continue
			}
		}
		if name, next, ok := readSQLIdentifierNamePath(query, index); ok {
			if normalizeColumnKey(name) == target {
				return true
			}
			index = next
			continue
		}
		index++
	}
	return false
}

func sqlParenStartsSubquery(query string, index int) bool {
	if index >= len(query) || query[index] != '(' {
		return false
	}
	next := skipSQLSpacesAndComments(query, index+1)
	return hasSQLKeywordAt(query, next, "select") || hasSQLKeywordAt(query, next, "with")
}

func statementEntityFromRegistry(registry *Registry, statement StatementMeta) (EntityMeta, bool) {
	if registry == nil {
		return EntityMeta{}, false
	}
	for _, candidate := range []string{statement.ParameterType, statement.ResultType} {
		if entity, ok := registry.Entity(normalizeTypeIdentifier(candidate)); ok {
			return entity, true
		}
	}
	if statement.ResultMap == "" {
		return EntityMeta{}, false
	}
	mapper, ok := registry.Mapper(statement.Namespace)
	if !ok {
		return EntityMeta{}, false
	}
	for _, resultMap := range mapper.ResultMaps {
		if resultMap.ID != statement.ResultMap {
			continue
		}
		return registry.Entity(normalizeTypeIdentifier(resultMap.TypeName))
	}
	return EntityMeta{}, false
}
