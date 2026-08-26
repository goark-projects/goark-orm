package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// SQLExecutor 是 *sql.DB 和 *sql.Tx 共同满足的最小执行接口。
type SQLExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SQLSession 基于 database/sql 执行已经注册的 Statement。
type SQLSession struct {
	registry                 *Registry
	executor                 SQLExecutor
	dialect                  Dialect
	configuration            Configuration
	typeHandlers             map[string]TypeHandler
	interceptors             []StatementInterceptor
	statementExecutor        StatementExecutor
	statementHandler         StatementHandler
	parameterHandler         ParameterHandler
	resultSetHandler         ResultSetHandler
	identifierGenerator      IdentifierGenerator
	metaObjectHandler        MetaObjectHandler
	localCache               *localCache
	localCacheScope          LocalCacheScope
	cacheEnabled             bool
	mapUnderscoreToCamelCase bool
	preparedMu               sync.Mutex
	preparedStatements       map[string]*sql.Stmt
	columnBindingPlans       sync.Map
	statementParameterPlans  sync.Map

	transactionalCache           bool
	secondLevelCacheMu           sync.Mutex
	pendingSecondLevelCacheFlush map[string]struct{}
	pendingSecondLevelCachePuts  map[string]map[string]reflect.Value
}

// SQLSessionOption 配置 SQLSession。
type SQLSessionOption func(*SQLSession) error

var _ Session = (*SQLSession)(nil)

// WithTypeHandler 注册运行时 TypeHandler。
func WithTypeHandler(name string, handler TypeHandler) SQLSessionOption {
	return func(session *SQLSession) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return configurationErrorf("type-handler name is required")
		}
		if handler == nil {
			return configurationErrorf("type-handler %q is nil", name)
		}
		session.typeHandlers[name] = handler
		return nil
	}
}

// WithLocalCache 配置 Session 级一级缓存。
func WithLocalCache(enabled bool) SQLSessionOption {
	return func(session *SQLSession) error {
		session.configuration.LocalCacheEnabled = boolPointer(enabled)
		if enabled {
			session.localCache = newLocalCache()
		} else {
			session.localCache = nil
		}
		return nil
	}
}

// NewSQLSession 创建可独立使用的 database/sql ORM Session。
func NewSQLSession(registry *Registry, executor SQLExecutor, dialect Dialect, options ...SQLSessionOption) (*SQLSession, error) {
	if registry == nil {
		return nil, configurationErrorf("registry is nil")
	}
	if executor == nil {
		return nil, configurationErrorf("SQL executor is nil")
	}
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	configuration := DefaultConfiguration()
	configuration.Dialect = dialect
	session := &SQLSession{
		registry:            registry,
		executor:            executor,
		dialect:             dialect,
		configuration:       cloneConfiguration(configuration),
		typeHandlers:        registry.TypeHandlers(),
		interceptors:        make([]StatementInterceptor, 0),
		localCache:          newLocalCache(),
		localCacheScope:     LocalCacheScopeSession,
		cacheEnabled:        true,
		identifierGenerator: firstIdentifierGenerator(configuration.GlobalConfig.IdentifierGenerator, NewDefaultIdentifierGenerator()),
		metaObjectHandler:   firstMetaObjectHandler(configuration.MetaObjectHandler, configuration.GlobalConfig.MetaObjectHandler),
	}
	session.statementExecutor = defaultStatementExecutor{}
	session.statementHandler = &defaultStatementHandler{session: session}
	session.parameterHandler = &defaultParameterHandler{session: session}
	session.resultSetHandler = &defaultResultSetHandler{session: session}
	if session.typeHandlers == nil {
		session.typeHandlers = make(map[string]TypeHandler)
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(session); err != nil {
			return nil, err
		}
	}
	return session, nil
}

// Configuration 返回当前 Session 使用的配置快照。
func (s *SQLSession) Configuration() Configuration {
	if s == nil {
		config, _ := normalizeConfiguration(Configuration{}, nil)
		return config
	}
	return cloneConfiguration(s.configuration)
}

// GlobalConfig 返回当前 Session 使用的全局配置快照。
func (s *SQLSession) GlobalConfig() GlobalConfig {
	return s.Configuration().GlobalConfig
}

// Dialect 返回当前 Session 使用的数据库方言。
func (s *SQLSession) Dialect() Dialect {
	if s == nil || s.dialect == nil {
		return NewQuestionDialect()
	}
	return s.dialect
}

// MetaObjectHandler 返回当前 Session 使用的自动填充处理器。
func (s *SQLSession) MetaObjectHandler() MetaObjectHandler {
	if s == nil {
		return nil
	}
	return s.metaObjectHandler
}

// Query 执行查询语句并扫描多行结果。
func (s *SQLSession) Query(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	meta, err := s.lookupStatement(statement)
	if err != nil {
		return err
	}
	return s.QueryStatement(ctx, meta, args, dest)
}

// QueryStatement 执行查询语句元数据并扫描多行结果。
func (s *SQLSession) QueryStatement(ctx context.Context, meta StatementMeta, args NamedArgs, dest any) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	return s.statementExecutor.Query(ctx, s, meta, args, dest)
}

// QueryOne 执行查询语句并要求最多返回一行。
func (s *SQLSession) QueryOne(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	meta, err := s.lookupStatement(statement)
	if err != nil {
		return err
	}
	return s.QueryOneStatement(ctx, meta, args, dest)
}

// QueryOneStatement 执行查询语句元数据并要求最多返回一行。
func (s *SQLSession) QueryOneStatement(ctx context.Context, meta StatementMeta, args NamedArgs, dest any) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	return s.statementExecutor.QueryOne(ctx, s, meta, args, dest)
}

// Exec 执行 insert、update、delete 语句。
func (s *SQLSession) Exec(ctx context.Context, statement string, args NamedArgs) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("goark-orm: context is nil")
	}
	meta, err := s.lookupStatement(statement)
	if err != nil {
		return Result{}, err
	}
	return s.ExecStatement(ctx, meta, args)
}

// ExecStatement 执行写入语句元数据。
func (s *SQLSession) ExecStatement(ctx context.Context, meta StatementMeta, args NamedArgs) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("goark-orm: context is nil")
	}
	return s.statementExecutor.Exec(ctx, s, meta, args)
}

func (s *SQLSession) clearLocalCache() {
	if s == nil {
		return
	}
	s.localCache.clear()
}

func (s *SQLSession) getLocalCache(key string, dest any) (bool, error) {
	if s == nil || s.localCacheScope != LocalCacheScopeSession {
		return false, nil
	}
	return s.localCache.get(key, dest)
}

func (s *SQLSession) putLocalCache(key string, dest any) error {
	if s == nil || s.localCacheScope != LocalCacheScopeSession {
		return nil
	}
	return s.localCache.put(key, dest)
}

func (s *SQLSession) lookupStatement(statement string) (StatementMeta, error) {
	if s == nil {
		return StatementMeta{}, configurationErrorf("session is nil")
	}
	meta, ok := s.registry.Statement(statement)
	if !ok {
		return StatementMeta{}, statementNotFoundError(statement)
	}
	return meta, nil
}

func (s *SQLSession) compileStatement(ctx context.Context, meta StatementMeta, args NamedArgs) (CompiledSQL, error) {
	if s == nil {
		return CompiledSQL{}, configurationErrorf("session is nil")
	}
	runtime, err := s.statementHandler.Prepare(ctx, meta, args)
	if err != nil {
		return CompiledSQL{}, err
	}
	return s.statementHandler.Compile(ctx, runtime)
}

func (s *SQLSession) prepareStatementRuntime(ctx context.Context, meta StatementMeta, args NamedArgs) (*StatementRuntime, error) {
	if s == nil {
		return nil, configurationErrorf("session is nil")
	}
	renderArgs := copyNamedArgs(args)
	if renderArgs == nil {
		renderArgs = NamedArgs{}
	}
	if strings.TrimSpace(meta.Provider) != "" {
		source, err := s.invokeSQLProvider(ctx, meta, renderArgs)
		if err != nil {
			return nil, err
		}
		meta.SQL = source.SQL
		meta.DynamicSQL = source.DynamicSQL
	}
	sqlText := meta.SQL
	if len(meta.DynamicSQL) > 0 {
		rendered, err := RenderDynamicSQL(meta.DynamicSQL, renderArgs)
		if err != nil {
			return nil, bindingFailure(meta.FullName, "render dynamic", err)
		}
		sqlText = rendered.SQL
		renderArgs = rendered.Args
	}
	runtime := &StatementRuntime{
		Meta:          meta,
		SQL:           sqlText,
		Args:          copyNamedArgs(renderArgs),
		Dialect:       s.Dialect(),
		Configuration: s.Configuration(),
	}
	if len(s.interceptors) > 0 {
		invocation := &StatementInvocation{
			statement:    runtime,
			interceptors: s.interceptors,
		}
		if err := invocation.Proceed(ctx); err != nil {
			return nil, &ExecutorError{Statement: meta.FullName, Operation: "intercept", Err: err}
		}
	}
	if err := s.applyMetaObjectFill(ctx, runtime); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (s *SQLSession) applyMetaObjectFill(ctx context.Context, runtime *StatementRuntime) error {
	if s == nil || runtime == nil || s.metaObjectHandler == nil || runtime.Meta.Source == StatementSourceBase {
		return nil
	}
	entity, ok := statementEntityFromRegistry(s.registry, runtime.Meta)
	if !ok {
		return nil
	}
	value, _ := entityValueFromArgs(runtime.Args, entity)
	return applyMetaObjectHandler(ctx, s.metaObjectHandler, runtime.Meta.Command, entity, value, runtime.Args)
}

func (s *SQLSession) invokeSQLProvider(ctx context.Context, meta StatementMeta, args NamedArgs) (SQLSource, error) {
	name := strings.TrimSpace(meta.Provider)
	provider, ok := s.registry.SQLProvider(name)
	if !ok {
		return SQLSource{}, statementNotFoundErrorf(name, "SQL provider %q is not registered", name)
	}
	source, err := provider(ctx, meta, copyNamedArgs(args))
	if err != nil {
		return SQLSource{}, &ExecutorError{
			Statement: meta.FullName,
			Operation: "invoke SQL provider",
			Message:   fmt.Sprintf("SQL provider %q for statement %s failed", name, meta.FullName),
			Err:       err,
		}
	}
	source.SQL = strings.TrimSpace(source.SQL)
	if source.SQL != "" && len(source.DynamicSQL) > 0 {
		return SQLSource{}, bindingErrorf("SQL provider %q for statement %s returned both SQL and DynamicSQL", name, meta.FullName)
	}
	if source.SQL == "" && len(source.DynamicSQL) == 0 {
		return SQLSource{}, bindingErrorf("SQL provider %q for statement %s returned empty SQL", name, meta.FullName)
	}
	return source, nil
}

func dynamicSQLContainsForbiddenSubstitution(nodes []DynamicSQLNode) bool {
	for _, node := range nodes {
		if strings.Contains(node.Text, "${") || strings.Contains(node.Value, "${") {
			return true
		}
		if dynamicSQLContainsForbiddenSubstitution(node.Children) {
			return true
		}
	}
	return false
}

func (s *SQLSession) compileRuntime(ctx context.Context, runtime *StatementRuntime) (CompiledSQL, error) {
	if runtime == nil {
		return CompiledSQL{}, configurationErrorf("statement runtime is nil")
	}
	boundArgs, err := s.parameterHandler.Bind(ctx, runtime.Meta, runtime.Args)
	if err != nil {
		return CompiledSQL{}, bindingFailure(runtime.Meta.FullName, "bind", err)
	}
	compiled, err := CompileSQLContext(ctx, runtime.SQL, boundArgs, runtime.Dialect)
	if err != nil {
		return CompiledSQL{}, bindingFailure(runtime.Meta.FullName, "compile", err)
	}
	return compiled, nil
}

func (s *SQLSession) bindArgs(ctx context.Context, statement StatementMeta, args NamedArgs) (NamedArgs, error) {
	if len(args) == 0 || statement.ParameterType == "" {
		return args, nil
	}
	entity, ok := s.registry.Entity(normalizeTypeIdentifier(statement.ParameterType))
	if !ok {
		return args, nil
	}
	bound := make(NamedArgs, len(args))
	for key, value := range args {
		bound[key] = value
	}
	usedParameters := s.statementParameterSet(statement)
	for _, column := range entity.Columns {
		for _, name := range columnParameterNames(column) {
			if !statementUsesParameter(usedParameters, name) {
				continue
			}
			value, ok := args[name]
			if !ok {
				continue
			}
			converted, err := s.convertColumnArgument(ctx, column, value)
			if err != nil {
				return nil, bindingFailure(statement.FullName, "bind", err)
			}
			bound[name] = converted
		}
	}
	for root, value := range args {
		if root == "" || !valueMatchesEntity(value, entity) {
			continue
		}
		for _, column := range entity.Columns {
			if !statementUsesColumnPath(usedParameters, root, column) {
				continue
			}
			field, ok, err := parameterProperty(value, column.FieldName)
			if err != nil {
				return nil, &BindingError{
					Statement: statement.FullName,
					Column:    column.ColumnName,
					Field:     column.FieldName,
					Err:       err,
				}
			}
			if !ok {
				if alias := parameterPropertyAlias(column.FieldName); alias != "" && alias != column.FieldName {
					field, ok, err = parameterProperty(value, alias)
					if err != nil {
						return nil, &BindingError{
							Statement: statement.FullName,
							Column:    column.ColumnName,
							Field:     column.FieldName,
							Err:       err,
						}
					}
				}
			}
			if !ok {
				continue
			}
			converted, err := s.convertColumnArgument(ctx, column, field)
			if err != nil {
				return nil, bindingFailure(statement.FullName, "bind", err)
			}
			for _, name := range columnParameterNames(column) {
				bound[root+"."+name] = converted
			}
		}
	}
	return bound, nil
}

func buildStatementParameterSet(statement StatementMeta) map[string]struct{} {
	parameters := make(map[string]struct{})
	for _, name := range statement.Parameters {
		name = strings.TrimSpace(name)
		if name != "" {
			parameters[name] = struct{}{}
		}
	}
	collectSQLParameterSet(statement.SQL, parameters)
	collectDynamicSQLParameterSet(statement.DynamicSQL, parameters)
	return parameters
}

func collectSQLParameterSet(sqlText string, parameters map[string]struct{}) {
	matches := statementParamPattern.FindAllStringSubmatch(sqlText, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name != "" {
			parameters[name] = struct{}{}
		}
	}
}

func collectDynamicSQLParameterSet(nodes []DynamicSQLNode, parameters map[string]struct{}) {
	for _, node := range nodes {
		collectSQLParameterSet(node.Text, parameters)
		collectSQLParameterSet(node.Value, parameters)
		collectDynamicSQLParameterSet(node.Children, parameters)
	}
}

func statementUsesParameter(parameters map[string]struct{}, name string) bool {
	if len(parameters) == 0 {
		return false
	}
	_, ok := parameters[name]
	return ok
}

func statementUsesColumnPath(parameters map[string]struct{}, root string, column ColumnMeta) bool {
	for _, name := range columnParameterNames(column) {
		if statementUsesParameter(parameters, root+"."+name) {
			return true
		}
	}
	return false
}

func (s *SQLSession) convertColumnArgument(ctx context.Context, column ColumnMeta, value any) (any, error) {
	if column.TypeHandler == "" {
		return value, nil
	}
	handler, ok := s.typeHandlers[column.TypeHandler]
	if !ok {
		return nil, &BindingError{
			Column:  column.ColumnName,
			Field:   column.FieldName,
			Message: fmt.Sprintf("type-handler %q is not registered", column.TypeHandler),
		}
	}
	converted, err := handler.ToDB(ctx, value)
	if err != nil {
		return nil, &BindingError{
			Column:  column.ColumnName,
			Field:   column.FieldName,
			Message: fmt.Sprintf("type-handler %q failed", column.TypeHandler),
			Err:     err,
		}
	}
	return converted, nil
}

func columnParameterNames(column ColumnMeta) []string {
	names := []string{column.FieldName}
	if alias := parameterPropertyAlias(column.FieldName); alias != "" && alias != column.FieldName {
		names = append(names, alias)
	}
	return names
}

func valueMatchesEntity(value any, entity EntityMeta) bool {
	if value == nil {
		return false
	}
	rv := reflect.ValueOf(value)
	for rv.IsValid() && (rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer) {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return false
	}
	return rv.Type().Name() == normalizeTypeIdentifier(entity.TypeName)
}

func (s *SQLSession) scanRows(ctx context.Context, rows Rows, statement StatementMeta, dest any) error {
	target, err := destination(dest)
	if err != nil {
		return mappingFailure(statement, err)
	}
	if target.Kind() != reflect.Slice {
		return &MappingError{Statement: statement.FullName, Message: "Query destination must be pointer to slice"}
	}
	columns, err := rows.Columns()
	if err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "read columns", Err: err}
	}
	if target.IsNil() {
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
	}
	if resultMap, ok := s.resultMap(statement); ok {
		if len(resultMap.Collections) > 0 {
			return mappingFailure(statement, s.scanRowsWithCollections(ctx, rows, columns, statement, resultMap, target))
		}
		defer func() {
			if resultMapHasNestedSelects(resultMap) {
				_ = rows.Close()
			}
		}()
		elementType := target.Type().Elem()
		for rows.Next() {
			element, err := s.scanSliceElementWithResultMap(ctx, rows, columns, statement, resultMap, elementType)
			if err != nil {
				return mappingFailure(statement, err)
			}
			target.Set(reflect.Append(target, element))
		}
		if err := rows.Err(); err != nil {
			return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
		}
		if resultMapHasNestedSelects(resultMap) {
			if err := rows.Close(); err != nil {
				return &ExecutorError{Statement: statement.FullName, Operation: "close rows", Err: err}
			}
			return mappingFailure(statement, s.applyNestedSelects(ctx, statement, resultMap, target))
		}
		return nil
	}
	elementType := target.Type().Elem()
	for rows.Next() {
		element, err := s.scanSliceElement(ctx, rows, columns, statement, elementType)
		if err != nil {
			return mappingFailure(statement, err)
		}
		target.Set(reflect.Append(target, element))
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
	}
	return nil
}

func (s *SQLSession) scanOne(ctx context.Context, rows Rows, statement StatementMeta, dest any) error {
	target, err := destination(dest)
	if err != nil {
		return mappingFailure(statement, err)
	}
	if target.Kind() == reflect.Slice {
		return &MappingError{Statement: statement.FullName, Message: "QueryOne destination must not be slice"}
	}
	columns, err := rows.Columns()
	if err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "read columns", Err: err}
	}
	if resultMap, ok := s.resultMap(statement); ok {
		if len(resultMap.Collections) > 0 {
			slice := reflect.New(reflect.SliceOf(target.Type())).Elem()
			if err := s.scanRowsWithCollections(ctx, rows, columns, statement, resultMap, slice); err != nil {
				return mappingFailure(statement, err)
			}
			if slice.Len() == 0 {
				return sql.ErrNoRows
			}
			if slice.Len() > 1 {
				return &TooManyResultsError{Statement: statement.FullName, Expected: 1, Actual: slice.Len()}
			}
			target.Set(slice.Index(0))
			return nil
		}
		defer func() {
			if resultMapHasNestedSelects(resultMap) {
				_ = rows.Close()
			}
		}()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
			}
			return sql.ErrNoRows
		}
		if err := s.scanValueWithResultMap(ctx, rows, columns, statement, resultMap, target); err != nil {
			return mappingFailure(statement, err)
		}
		if rows.Next() {
			return &TooManyResultsError{Statement: statement.FullName}
		}
		if err := rows.Err(); err != nil {
			return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
		}
		if resultMapHasNestedSelects(resultMap) {
			if err := rows.Close(); err != nil {
				return &ExecutorError{Statement: statement.FullName, Operation: "close rows", Err: err}
			}
			return mappingFailure(statement, s.applyNestedSelects(ctx, statement, resultMap, target))
		}
		return nil
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
		}
		return sql.ErrNoRows
	}
	if err := s.scanValue(ctx, rows, columns, statement, target); err != nil {
		return mappingFailure(statement, err)
	}
	if rows.Next() {
		return &TooManyResultsError{Statement: statement.FullName}
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: statement.FullName, Operation: "iterate rows", Err: err}
	}
	return nil
}

func (s *SQLSession) scanSliceElement(ctx context.Context, rows Rows, columns []string, statement StatementMeta, elementType reflect.Type) (reflect.Value, error) {
	if elementType.Kind() == reflect.Pointer {
		element := reflect.New(elementType.Elem())
		if err := s.scanValue(ctx, rows, columns, statement, element); err != nil {
			return reflect.Value{}, err
		}
		return element, nil
	}
	element := reflect.New(elementType).Elem()
	if err := s.scanValue(ctx, rows, columns, statement, element); err != nil {
		return reflect.Value{}, err
	}
	return element, nil
}

func (s *SQLSession) scanSliceElementWithResultMap(ctx context.Context, rows Rows, columns []string, statement StatementMeta, resultMap ResultMapMeta, elementType reflect.Type) (reflect.Value, error) {
	if elementType.Kind() == reflect.Pointer {
		element := reflect.New(elementType.Elem())
		if err := s.scanValueWithResultMap(ctx, rows, columns, statement, resultMap, element); err != nil {
			return reflect.Value{}, err
		}
		return element, nil
	}
	element := reflect.New(elementType).Elem()
	if err := s.scanValueWithResultMap(ctx, rows, columns, statement, resultMap, element); err != nil {
		return reflect.Value{}, err
	}
	return element, nil
}

func (s *SQLSession) scanValue(ctx context.Context, scanner interface{ Scan(dest ...any) error }, columns []string, statement StatementMeta, target reflect.Value) error {
	if !target.IsValid() {
		return &MappingError{Statement: statement.FullName, Message: "destination is invalid"}
	}
	if target.Kind() == reflect.Pointer {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		return s.scanValue(ctx, scanner, columns, statement, target.Elem())
	}
	if target.Kind() == reflect.Struct {
		return s.scanStruct(ctx, scanner, columns, statement, target)
	}
	if target.Kind() == reflect.Map {
		return mappingFailure(statement, scanMap(scanner, columns, target))
	}
	if len(columns) != 1 {
		return &MappingError{
			Statement: statement.FullName,
			Message:   fmt.Sprintf("scalar destination requires exactly one column, got %d", len(columns)),
		}
	}
	if !target.CanAddr() {
		return &MappingError{Statement: statement.FullName, Message: "destination cannot be addressed"}
	}
	if err := scanner.Scan(target.Addr().Interface()); err != nil {
		return &MappingError{Statement: statement.FullName, Column: columns[0], Err: err}
	}
	return nil
}

func (s *SQLSession) scanValueWithResultMap(ctx context.Context, scanner interface{ Scan(dest ...any) error }, columns []string, statement StatementMeta, resultMap ResultMapMeta, target reflect.Value) error {
	if !target.IsValid() {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Message: "destination is invalid"}
	}
	if target.Kind() == reflect.Pointer {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		return s.scanValueWithResultMap(ctx, scanner, columns, statement, resultMap, target.Elem())
	}
	if target.Kind() != reflect.Struct {
		return s.scanValue(ctx, scanner, columns, statement, target)
	}
	values, err := scanRowValues(scanner, len(columns))
	if err != nil {
		return &MappingError{Statement: statement.FullName, ResultMap: resultMap.ID, Err: err}
	}
	effective := resultMap
	if resultMapHasDiscriminator(resultMap) {
		columnIndexes := resultColumnIndexes(columns)
		selected, err := s.effectiveResultMapForRow(ctx, statement, resultMap, columnIndexes, values, target.Type())
		if err != nil {
			return mappingFailure(statement, err)
		}
		effective = selected
	}
	bindings := s.columnBindingsForResultMap(statement, target.Type(), effective)
	return mappingFailure(statement, s.applyBindings(ctx, target, bindings, columns, values))
}

func scanMap(scanner interface{ Scan(dest ...any) error }, columns []string, target reflect.Value) error {
	if target.Type().Key().Kind() != reflect.String || target.Type().Elem().Kind() != reflect.Interface {
		return mappingErrorf("map destination must be map[string]any")
	}
	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for index := range values {
		scanTargets[index] = &values[index]
	}
	if err := scanner.Scan(scanTargets...); err != nil {
		return &MappingError{Err: err}
	}
	if target.IsNil() {
		target.Set(reflect.MakeMapWithSize(target.Type(), len(columns)))
	}
	for index, column := range columns {
		value := reflect.ValueOf(values[index])
		if !value.IsValid() {
			value = reflect.Zero(target.Type().Elem())
		}
		target.SetMapIndex(reflect.ValueOf(column), value)
	}
	return nil
}

type columnBinding struct {
	index           []int
	typeHandler     string
	presenceColumns []string
	auto            bool
	fieldName       string
}

func (s *SQLSession) scanStruct(ctx context.Context, scanner interface{ Scan(dest ...any) error }, columns []string, statement StatementMeta, target reflect.Value) error {
	bindings := s.columnBindings(statement, target.Type())
	targets := make([]any, len(columns))
	postScan := make([]func() error, 0)
	for index, column := range columns {
		binding, ok := s.lookupColumnBinding(bindings, column)
		if !ok {
			var discard any
			targets[index] = &discard
			continue
		}
		field, ok := fieldByIndexAlloc(target, binding.index)
		if !ok {
			var discard any
			targets[index] = &discard
			continue
		}
		if !field.IsValid() || !field.CanSet() || !field.CanAddr() {
			var discard any
			targets[index] = &discard
			continue
		}
		if binding.typeHandler == "" {
			targets[index] = field.Addr().Interface()
			continue
		}
		handler, ok := s.typeHandlers[binding.typeHandler]
		if !ok {
			return &MappingError{
				Statement: statement.FullName,
				Column:    column,
				Field:     binding.fieldName,
				Message:   fmt.Sprintf("type-handler %q is not registered", binding.typeHandler),
			}
		}
		holder := new(any)
		targets[index] = holder
		fieldTarget := field.Addr().Interface()
		handlerName := binding.typeHandler
		fieldName := binding.fieldName
		columnName := column
		postScan = append(postScan, func() error {
			if err := handler.FromDB(ctx, *holder, fieldTarget); err != nil {
				return &MappingError{
					Statement: statement.FullName,
					Column:    columnName,
					Field:     fieldName,
					Message:   fmt.Sprintf("type-handler %q failed", handlerName),
					Err:       err,
				}
			}
			return nil
		})
	}
	if err := scanner.Scan(targets...); err != nil {
		return &MappingError{Statement: statement.FullName, Err: err}
	}
	for _, apply := range postScan {
		if err := apply(); err != nil {
			return mappingFailure(statement, err)
		}
	}
	return nil
}

func (s *SQLSession) columnBindings(statement StatementMeta, typ reflect.Type) map[string]columnBinding {
	if resultMap, ok := s.resultMap(statement); ok {
		return s.columnBindingsForResultMap(statement, typ, resultMap)
	}
	return s.columnBindingsWithResultMap(statement, typ, ResultMapMeta{}, false)
}

func (s *SQLSession) columnBindingsForResultMap(statement StatementMeta, typ reflect.Type, resultMap ResultMapMeta) map[string]columnBinding {
	return s.columnBindingsWithResultMap(statement, typ, resultMap, true)
}

func (s *SQLSession) columnBindingsWithResultMap(statement StatementMeta, typ reflect.Type, resultMap ResultMapMeta, hasResultMap bool) map[string]columnBinding {
	if s != nil {
		return s.cachedColumnBindingsWithResultMap(statement, typ, resultMap, hasResultMap)
	}
	return buildColumnBindingsWithResultMap(nil, statement, typ, resultMap, hasResultMap)
}

func buildColumnBindingsWithResultMap(s *SQLSession, statement StatementMeta, typ reflect.Type, resultMap ResultMapMeta, hasResultMap bool) map[string]columnBinding {
	autoMapping := true
	if hasResultMap && resultMap.AutoMapping != nil {
		autoMapping = *resultMap.AutoMapping
	}
	bindings := make(map[string]columnBinding)
	if !hasResultMap || autoMapping {
		bindings = exportedFieldBindings(typ)
	}
	if (!hasResultMap || autoMapping) && s != nil && s.registry != nil {
		if entity, ok := s.registry.Entity(typ.Name()); ok {
			addEntityColumnBindings(bindings, typ, entity)
		}
	}
	if hasResultMap {
		for _, item := range resultMapFieldMetas(resultMap) {
			addDirectFieldBinding(bindings, typ, item, nil)
		}
		for _, association := range resultMap.Associations {
			addAssociationBindings(bindings, typ, nil, association, "")
		}
	}
	return bindings
}

func addEntityColumnBindings(bindings map[string]columnBinding, typ reflect.Type, entity EntityMeta) {
	for _, column := range entity.Columns {
		if field, ok := typ.FieldByName(column.FieldName); ok && field.PkgPath == "" {
			bindings[normalizeColumnKey(column.ColumnName)] = columnBinding{
				index:       field.Index,
				typeHandler: column.TypeHandler,
				fieldName:   field.Name,
			}
		}
	}
}

func addDirectFieldBinding(bindings map[string]columnBinding, typ reflect.Type, item ResultFieldMeta, presenceColumns []string) {
	if item.Column == "" {
		return
	}
	field, ok := typ.FieldByName(item.Property)
	if !ok || field.PkgPath != "" {
		return
	}
	bindings[normalizeColumnKey(item.Column)] = columnBinding{
		index:           field.Index,
		typeHandler:     item.TypeHandler,
		presenceColumns: append([]string(nil), presenceColumns...),
		fieldName:       field.Name,
	}
}

func addAssociationBindings(bindings map[string]columnBinding, typ reflect.Type, path []int, association ResultAssociationMeta, inheritedPrefix string) {
	field, ok := typ.FieldByName(association.Property)
	if !ok || field.PkgPath != "" {
		return
	}
	nestedType := field.Type
	for nestedType.Kind() == reflect.Pointer {
		nestedType = nestedType.Elem()
	}
	if nestedType.Kind() != reflect.Struct {
		return
	}
	fieldPath := append(append([]int(nil), path...), field.Index...)
	prefix := inheritedPrefix + association.ColumnPrefix
	presenceColumns := prefixedColumns(association.NotNullColumns, prefix)
	for _, item := range association.Fields {
		nestedField, ok := nestedType.FieldByName(item.Property)
		if !ok || nestedField.PkgPath != "" {
			continue
		}
		column := prefixColumn(prefix, item.Column)
		if column == "" {
			continue
		}
		bindings[normalizeColumnKey(column)] = columnBinding{
			index:           append(append([]int(nil), fieldPath...), nestedField.Index...),
			typeHandler:     item.TypeHandler,
			presenceColumns: presenceColumns,
			fieldName:       nestedField.Name,
		}
	}
	for _, child := range association.Associations {
		addAssociationBindings(bindings, nestedType, fieldPath, child, prefix)
	}
}

func prefixColumn(prefix string, column string) string {
	column = strings.TrimSpace(column)
	if column == "" {
		return ""
	}
	return strings.TrimSpace(prefix) + column
}

func prefixedColumns(columns []string, prefix string) []string {
	if len(columns) == 0 {
		return nil
	}
	out := make([]string, 0, len(columns))
	for _, column := range columns {
		column = prefixColumn(prefix, column)
		if column != "" {
			out = append(out, column)
		}
	}
	return out
}

func exportedFieldBindings(typ reflect.Type) map[string]columnBinding {
	bindings := make(map[string]columnBinding)
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
		bindings[normalizeColumnKey(field.Name)] = columnBinding{
			index:     field.Index,
			auto:      true,
			fieldName: field.Name,
		}
	}
	return bindings
}

func (s *SQLSession) lookupColumnBinding(bindings map[string]columnBinding, column string) (columnBinding, bool) {
	binding, ok := bindings[normalizeColumnKey(column)]
	if !ok {
		return columnBinding{}, false
	}
	if !binding.auto || s == nil || s.mapUnderscoreToCamelCase {
		return binding, true
	}
	if strictColumnKey(column) != strictColumnKey(binding.fieldName) {
		return columnBinding{}, false
	}
	return binding, true
}

func (s *SQLSession) resultMap(statement StatementMeta) (ResultMapMeta, bool) {
	if statement.ResultMap == "" {
		return ResultMapMeta{}, false
	}
	return s.lookupResultMap(statement.Namespace, statement.ResultMap)
}

func (s *SQLSession) lookupResultMap(namespace string, id string) (ResultMapMeta, bool) {
	id = normalizeRuntimeResultMapID(namespace, id)
	if id == "" {
		return ResultMapMeta{}, false
	}
	mapper, ok := s.registry.Mapper(namespace)
	if !ok {
		return ResultMapMeta{}, false
	}
	for _, resultMap := range mapper.ResultMaps {
		if resultMap.ID == id {
			return resultMap, true
		}
	}
	return ResultMapMeta{}, false
}

func normalizeRuntimeResultMapID(namespace string, id string) string {
	id = strings.TrimSpace(id)
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return id
	}
	prefix := namespace + "."
	return strings.TrimPrefix(id, prefix)
}

func destination(dest any) (reflect.Value, error) {
	if dest == nil {
		return reflect.Value{}, fmt.Errorf("goark-orm: destination is nil")
	}
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return reflect.Value{}, fmt.Errorf("goark-orm: destination must be non-nil pointer")
	}
	return value.Elem(), nil
}

func normalizeColumnKey(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch r {
		case '_', '-', '.', ' ':
			continue
		default:
			builder.WriteRune(r)
		}
	}
	return strings.ToLower(builder.String())
}

func strictColumnKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeTypeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	for strings.HasPrefix(value, "[]") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "[]"))
	}
	for strings.HasPrefix(value, "*") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "*"))
	}
	if index := strings.LastIndex(value, "."); index >= 0 {
		return value[index+1:]
	}
	return value
}
