package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// SQLExecutor 是 *sql.DB 和 *sql.Tx 共同满足的最小执行接口。
type SQLExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SQLSession 基于 database/sql 执行已经注册的 Statement。
type SQLSession struct {
	registry     *Registry
	executor     SQLExecutor
	dialect      Dialect
	typeHandlers map[string]TypeHandler
}

// SQLSessionOption 配置 SQLSession。
type SQLSessionOption func(*SQLSession) error

var _ Session = (*SQLSession)(nil)

// WithTypeHandler 注册运行时 TypeHandler。
func WithTypeHandler(name string, handler TypeHandler) SQLSessionOption {
	return func(session *SQLSession) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("goark-orm: type-handler name is required")
		}
		if handler == nil {
			return fmt.Errorf("goark-orm: type-handler %q is nil", name)
		}
		session.typeHandlers[name] = handler
		return nil
	}
}

// NewSQLSession 创建可独立使用的 database/sql ORM Session。
func NewSQLSession(registry *Registry, executor SQLExecutor, dialect Dialect, options ...SQLSessionOption) (*SQLSession, error) {
	if registry == nil {
		return nil, fmt.Errorf("goark-orm: registry is nil")
	}
	if executor == nil {
		return nil, fmt.Errorf("goark-orm: SQL executor is nil")
	}
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	session := &SQLSession{
		registry:     registry,
		executor:     executor,
		dialect:      dialect,
		typeHandlers: make(map[string]TypeHandler),
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

// Query 执行查询语句并扫描多行结果。
func (s *SQLSession) Query(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	meta, compiled, err := s.compile(ctx, statement, args)
	if err != nil {
		return err
	}
	if meta.Command != StatementCommandSelect {
		return fmt.Errorf("goark-orm: statement %s is %s, not select", meta.FullName, meta.Command)
	}
	rows, err := s.executor.QueryContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		return err
	}
	scanErr := s.scanRows(ctx, rows, meta, dest)
	closeErr := rows.Close()
	if scanErr != nil {
		return scanErr
	}
	return closeErr
}

// QueryOne 执行查询语句并要求最多返回一行。
func (s *SQLSession) QueryOne(ctx context.Context, statement string, args NamedArgs, dest any) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	meta, compiled, err := s.compile(ctx, statement, args)
	if err != nil {
		return err
	}
	if meta.Command != StatementCommandSelect {
		return fmt.Errorf("goark-orm: statement %s is %s, not select", meta.FullName, meta.Command)
	}
	rows, err := s.executor.QueryContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		return err
	}
	scanErr := s.scanOne(ctx, rows, meta, dest)
	closeErr := rows.Close()
	if scanErr != nil {
		return scanErr
	}
	return closeErr
}

// Exec 执行 insert、update、delete 语句。
func (s *SQLSession) Exec(ctx context.Context, statement string, args NamedArgs) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("goark-orm: context is nil")
	}
	meta, compiled, err := s.compile(ctx, statement, args)
	if err != nil {
		return Result{}, err
	}
	if meta.Command == StatementCommandSelect {
		return Result{}, fmt.Errorf("goark-orm: statement %s is select; use Query or QueryOne", meta.FullName)
	}
	sqlResult, err := s.executor.ExecContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		return Result{}, err
	}
	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return Result{}, err
	}
	result := Result{RowsAffected: rowsAffected}
	lastInsertID, err := sqlResult.LastInsertId()
	if err != nil {
		if meta.UseGeneratedKeys {
			return Result{}, err
		}
		return result, nil
	}
	result.LastInsertID = lastInsertID
	return result, nil
}

func (s *SQLSession) compile(ctx context.Context, statement string, args NamedArgs) (StatementMeta, CompiledSQL, error) {
	if s == nil {
		return StatementMeta{}, CompiledSQL{}, fmt.Errorf("goark-orm: session is nil")
	}
	meta, ok := s.registry.Statement(statement)
	if !ok {
		return StatementMeta{}, CompiledSQL{}, fmt.Errorf("goark-orm: statement %q is not registered", statement)
	}
	sqlText := meta.SQL
	renderArgs := args
	if len(meta.DynamicSQL) > 0 {
		rendered, err := RenderDynamicSQL(meta.DynamicSQL, args)
		if err != nil {
			return StatementMeta{}, CompiledSQL{}, fmt.Errorf("goark-orm: render dynamic statement %s failed: %w", meta.FullName, err)
		}
		sqlText = rendered.SQL
		renderArgs = rendered.Args
	}
	boundArgs, err := s.bindArgs(ctx, meta, renderArgs)
	if err != nil {
		return StatementMeta{}, CompiledSQL{}, fmt.Errorf("goark-orm: bind statement %s failed: %w", meta.FullName, err)
	}
	compiled, err := CompileSQL(sqlText, boundArgs, s.dialect)
	if err != nil {
		return StatementMeta{}, CompiledSQL{}, fmt.Errorf("goark-orm: compile statement %s failed: %w", meta.FullName, err)
	}
	return meta, compiled, nil
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
	for _, column := range entity.Columns {
		if column.TypeHandler == "" {
			continue
		}
		value, ok := args[column.FieldName]
		if !ok {
			continue
		}
		handler, ok := s.typeHandlers[column.TypeHandler]
		if !ok {
			return nil, fmt.Errorf("type-handler %q is not registered", column.TypeHandler)
		}
		converted, err := handler.ToDB(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("type-handler %q failed: %w", column.TypeHandler, err)
		}
		bound[column.FieldName] = converted
	}
	return bound, nil
}

func (s *SQLSession) scanRows(ctx context.Context, rows *sql.Rows, statement StatementMeta, dest any) error {
	target, err := destination(dest)
	if err != nil {
		return err
	}
	if target.Kind() != reflect.Slice {
		return fmt.Errorf("goark-orm: Query destination must be pointer to slice")
	}
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	if target.IsNil() {
		target.Set(reflect.MakeSlice(target.Type(), 0, 0))
	}
	elementType := target.Type().Elem()
	for rows.Next() {
		element, err := s.scanSliceElement(ctx, rows, columns, statement, elementType)
		if err != nil {
			return err
		}
		target.Set(reflect.Append(target, element))
	}
	return rows.Err()
}

func (s *SQLSession) scanOne(ctx context.Context, rows *sql.Rows, statement StatementMeta, dest any) error {
	target, err := destination(dest)
	if err != nil {
		return err
	}
	if target.Kind() == reflect.Slice {
		return fmt.Errorf("goark-orm: QueryOne destination must not be slice")
	}
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := s.scanValue(ctx, rows, columns, statement, target); err != nil {
		return err
	}
	if rows.Next() {
		return fmt.Errorf("goark-orm: statement %s returned more than one row", statement.FullName)
	}
	return rows.Err()
}

func (s *SQLSession) scanSliceElement(ctx context.Context, rows *sql.Rows, columns []string, statement StatementMeta, elementType reflect.Type) (reflect.Value, error) {
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

func (s *SQLSession) scanValue(ctx context.Context, scanner interface{ Scan(dest ...any) error }, columns []string, statement StatementMeta, target reflect.Value) error {
	if !target.IsValid() {
		return fmt.Errorf("goark-orm: destination is invalid")
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
	if len(columns) != 1 {
		return fmt.Errorf("goark-orm: scalar destination requires exactly one column, got %d", len(columns))
	}
	if !target.CanAddr() {
		return fmt.Errorf("goark-orm: destination cannot be addressed")
	}
	return scanner.Scan(target.Addr().Interface())
}

type columnBinding struct {
	index       []int
	typeHandler string
}

func (s *SQLSession) scanStruct(ctx context.Context, scanner interface{ Scan(dest ...any) error }, columns []string, statement StatementMeta, target reflect.Value) error {
	bindings := s.columnBindings(statement, target.Type())
	targets := make([]any, len(columns))
	postScan := make([]func() error, 0)
	for index, column := range columns {
		binding, ok := bindings[normalizeColumnKey(column)]
		if !ok {
			var discard any
			targets[index] = &discard
			continue
		}
		field := target.FieldByIndex(binding.index)
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
			return fmt.Errorf("goark-orm: type-handler %q is not registered", binding.typeHandler)
		}
		holder := new(any)
		targets[index] = holder
		fieldTarget := field.Addr().Interface()
		handlerName := binding.typeHandler
		postScan = append(postScan, func() error {
			if err := handler.FromDB(ctx, *holder, fieldTarget); err != nil {
				return fmt.Errorf("goark-orm: type-handler %q failed: %w", handlerName, err)
			}
			return nil
		})
	}
	if err := scanner.Scan(targets...); err != nil {
		return err
	}
	for _, apply := range postScan {
		if err := apply(); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLSession) columnBindings(statement StatementMeta, typ reflect.Type) map[string]columnBinding {
	bindings := exportedFieldBindings(typ)
	if entity, ok := s.registry.Entity(typ.Name()); ok {
		for _, column := range entity.Columns {
			if field, ok := typ.FieldByName(column.FieldName); ok && field.PkgPath == "" {
				bindings[normalizeColumnKey(column.ColumnName)] = columnBinding{
					index:       field.Index,
					typeHandler: column.TypeHandler,
				}
			}
		}
	}
	if resultMap, ok := s.resultMap(statement); ok {
		for _, item := range resultMap.Fields {
			if field, ok := typ.FieldByName(item.Property); ok && field.PkgPath == "" {
				bindings[normalizeColumnKey(item.Column)] = columnBinding{
					index:       field.Index,
					typeHandler: item.TypeHandler,
				}
			}
		}
	}
	return bindings
}

func exportedFieldBindings(typ reflect.Type) map[string]columnBinding {
	bindings := make(map[string]columnBinding)
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
		bindings[normalizeColumnKey(field.Name)] = columnBinding{index: field.Index}
	}
	return bindings
}

func (s *SQLSession) resultMap(statement StatementMeta) (ResultMapMeta, bool) {
	if statement.ResultMap == "" {
		return ResultMapMeta{}, false
	}
	mapper, ok := s.registry.Mapper(statement.Namespace)
	if !ok {
		return ResultMapMeta{}, false
	}
	for _, resultMap := range mapper.ResultMaps {
		if resultMap.ID == statement.ResultMap {
			return resultMap, true
		}
	}
	return ResultMapMeta{}, false
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
