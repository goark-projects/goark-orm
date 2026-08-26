package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// CallResult 表示存储过程调用结果。
type CallResult struct {
	RowsAffected int64
}

// CallSession 描述支持按 Statement 名称调用存储过程的 Session。
type CallSession interface {
	Call(ctx context.Context, statement string, args NamedArgs, resultSets ...any) (CallResult, error)
}

// StatementCallSession 描述支持直接执行 StatementMeta 调用的 Session。
type StatementCallSession interface {
	CallStatement(ctx context.Context, statement StatementMeta, args NamedArgs, resultSets ...any) (CallResult, error)
}

// ResultSetRows 描述支持多结果集前进的行集。
type ResultSetRows interface {
	Rows
	NextResultSet() bool
}

// ParseParameterMode 解析可调用语句参数方向。
func ParseParameterMode(value string) (ParameterMode, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", string(ParameterModeIn):
		return ParameterModeIn, nil
	case string(ParameterModeOut):
		return ParameterModeOut, nil
	case "IN_OUT", "IN OUT", string(ParameterModeInOut):
		return ParameterModeInOut, nil
	default:
		return "", fmt.Errorf("goark-orm: parameter mode %q requires IN, OUT or INOUT", value)
	}
}

// NormalizeParameterMode 返回参数方向的安全默认值。
func NormalizeParameterMode(mode ParameterMode) ParameterMode {
	normalized, err := ParseParameterMode(string(mode))
	if err != nil {
		return ParameterModeIn
	}
	return normalized
}

// Call 执行生成 Mapper 使用的存储过程调用。
func Call(ctx context.Context, session Session, statement string, args NamedArgs, resultSets ...any) (CallResult, error) {
	if session == nil {
		return CallResult{}, configurationErrorf("session is nil")
	}
	callSession, ok := session.(CallSession)
	if !ok {
		return CallResult{}, configurationErrorf("session does not support stored procedure call")
	}
	return callSession.Call(ctx, statement, args, resultSets...)
}

// Call 执行存储过程或可调用语句。
func (s *SQLSession) Call(ctx context.Context, statement string, args NamedArgs, resultSets ...any) (CallResult, error) {
	if ctx == nil {
		return CallResult{}, fmt.Errorf("goark-orm: context is nil")
	}
	meta, err := s.lookupStatement(statement)
	if err != nil {
		return CallResult{}, err
	}
	return s.CallStatement(ctx, meta, args, resultSets...)
}

// CallStatement 基于 StatementMeta 执行存储过程或可调用语句。
func (s *SQLSession) CallStatement(ctx context.Context, meta StatementMeta, args NamedArgs, resultSets ...any) (CallResult, error) {
	if ctx == nil {
		return CallResult{}, fmt.Errorf("goark-orm: context is nil")
	}
	if s == nil {
		return CallResult{}, configurationErrorf("session is nil")
	}
	if meta.Command != StatementCommandCall {
		return CallResult{}, fmt.Errorf("goark-orm: statement %s is %s, not call", meta.FullName, meta.Command)
	}
	if err := validateCallResultSetCount(meta, resultSets); err != nil {
		return CallResult{}, err
	}
	compiled, outBindings, err := s.compileCallStatement(ctx, meta, args)
	if err != nil {
		return CallResult{}, err
	}
	if err := s.flushStatementCaches(ctx, meta); err != nil {
		return CallResult{}, err
	}
	if len(resultSets) > 0 {
		return s.callWithResultSets(ctx, meta, compiled, outBindings, resultSets)
	}
	return s.callWithoutResultSets(ctx, meta, compiled, outBindings)
}

func (s *SQLSession) compileCallStatement(ctx context.Context, meta StatementMeta, args NamedArgs) (CompiledSQL, []callOutBinding, error) {
	runtime, err := s.statementHandler.Prepare(ctx, meta, args)
	if err != nil {
		return CompiledSQL{}, nil, err
	}
	boundArgs, err := s.parameterHandler.Bind(ctx, runtime.Meta, runtime.Args)
	if err != nil {
		return CompiledSQL{}, nil, bindingFailure(runtime.Meta.FullName, "bind", err)
	}
	callArgs, outBindings, err := s.bindCallableParameters(ctx, runtime.Meta, boundArgs)
	if err != nil {
		return CompiledSQL{}, nil, bindingFailure(runtime.Meta.FullName, "bind call parameters", err)
	}
	compiled, err := CompileSQLContext(ctx, runtime.SQL, callArgs, runtime.Dialect)
	if err != nil {
		return CompiledSQL{}, nil, bindingFailure(runtime.Meta.FullName, "compile", err)
	}
	return compiled, outBindings, nil
}

func (s *SQLSession) callWithResultSets(ctx context.Context, meta StatementMeta, compiled CompiledSQL, outBindings []callOutBinding, resultSets []any) (CallResult, error) {
	rows, err := s.querySQL(ctx, meta, compiled)
	if err != nil {
		return CallResult{}, executorFailure(meta, "call query", compiled, err)
	}
	scanErr := s.scanCallResultSets(ctx, rows, meta, resultSets)
	closeErr := rows.Close()
	if scanErr != nil {
		return CallResult{}, scanErr
	}
	if closeErr != nil {
		return CallResult{}, executorFailure(meta, "close call rows", compiled, closeErr)
	}
	if err := applyCallOutBindings(ctx, outBindings); err != nil {
		return CallResult{}, bindingFailure(meta.FullName, "apply out parameters", err)
	}
	return CallResult{}, nil
}

func (s *SQLSession) callWithoutResultSets(ctx context.Context, meta StatementMeta, compiled CompiledSQL, outBindings []callOutBinding) (CallResult, error) {
	sqlResult, err := s.execSQL(ctx, meta, compiled)
	if err != nil {
		return CallResult{}, executorFailure(meta, "call exec", compiled, err)
	}
	if err := applyCallOutBindings(ctx, outBindings); err != nil {
		return CallResult{}, bindingFailure(meta.FullName, "apply out parameters", err)
	}
	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return CallResult{}, executorFailure(meta, "rows affected", compiled, err)
	}
	return CallResult{RowsAffected: rowsAffected}, nil
}

func validateCallResultSetCount(meta StatementMeta, resultSets []any) error {
	if len(meta.ResultSets) == 0 || len(resultSets) == len(meta.ResultSets) {
		return nil
	}
	return &MappingError{
		Statement: meta.FullName,
		Message:   fmt.Sprintf("stored procedure declares %d result sets, got %d destinations", len(meta.ResultSets), len(resultSets)),
	}
}

func (s *SQLSession) scanCallResultSets(ctx context.Context, rows Rows, meta StatementMeta, resultSets []any) error {
	if len(resultSets) == 0 {
		return nil
	}
	for index, dest := range resultSets {
		if index > 0 {
			nextRows, ok := rows.(ResultSetRows)
			if !ok || !nextRows.NextResultSet() {
				return &MappingError{
					Statement: meta.FullName,
					Message:   fmt.Sprintf("stored procedure result set %d is missing", index+1),
				}
			}
		}
		statement := callResultSetStatement(meta, index)
		if err := s.resultSetHandler.ScanRows(ctx, rows, statement, dest); err != nil {
			return mappingFailure(statement, err)
		}
	}
	if err := rows.Err(); err != nil {
		return &ExecutorError{Statement: meta.FullName, Operation: "iterate call result sets", Err: err}
	}
	return nil
}

func callResultSetStatement(meta StatementMeta, index int) StatementMeta {
	if index < 0 || index >= len(meta.ResultSets) {
		return meta
	}
	resultSet := meta.ResultSets[index]
	if resultSet.ResultMap != "" {
		meta.ResultMap = resultSet.ResultMap
	}
	if resultSet.ResultType != "" {
		meta.ResultType = resultSet.ResultType
	}
	return meta
}

type callOutBinding struct {
	name        string
	dest        any
	holder      *any
	handler     TypeHandler
	handlerName string
}

func (s *SQLSession) bindCallableParameters(ctx context.Context, meta StatementMeta, args NamedArgs) (NamedArgs, []callOutBinding, error) {
	if len(meta.ParameterModes) == 0 {
		return args, nil, nil
	}
	bound := copyNamedArgs(args)
	if bound == nil {
		bound = NamedArgs{}
	}
	outBindings := make([]callOutBinding, 0)
	for _, parameter := range meta.ParameterModes {
		name := strings.TrimSpace(parameter.Name)
		if name == "" {
			return nil, nil, bindingErrorf("call parameter name is required")
		}
		mode := NormalizeParameterMode(parameter.Mode)
		value, ok, err := resolveNamedArg(bound, name)
		if err != nil {
			return nil, nil, &BindingError{Parameter: name, Err: err}
		}
		if !ok {
			return nil, nil, &BindingError{Parameter: name, Message: fmt.Sprintf("call parameter %q is missing", name)}
		}
		switch mode {
		case ParameterModeOut, ParameterModeInOut:
			out, binding, err := s.callOutParameter(ctx, parameter, value, mode == ParameterModeInOut)
			if err != nil {
				return nil, nil, &BindingError{Parameter: name, Err: err}
			}
			bound[name] = out
			if binding != nil {
				outBindings = append(outBindings, *binding)
			}
		case ParameterModeIn:
			converted, err := s.callInParameter(ctx, parameter, value)
			if err != nil {
				return nil, nil, &BindingError{Parameter: name, Err: err}
			}
			bound[name] = converted
		}
	}
	return bound, outBindings, nil
}

func (s *SQLSession) callInParameter(ctx context.Context, parameter ParameterMeta, value any) (any, error) {
	handlerName := strings.TrimSpace(parameter.TypeHandler)
	if handlerName == "" {
		return value, nil
	}
	handler, ok := s.typeHandlers[handlerName]
	if !ok {
		return nil, fmt.Errorf("type-handler %q is not registered", handlerName)
	}
	return handler.ToDB(ctx, value)
}

func (s *SQLSession) callOutParameter(ctx context.Context, parameter ParameterMeta, dest any, in bool) (sql.Out, *callOutBinding, error) {
	if !isNonNilPointer(dest) {
		return sql.Out{}, nil, fmt.Errorf("OUT parameter %q destination must be non-nil pointer", parameter.Name)
	}
	handlerName := strings.TrimSpace(parameter.TypeHandler)
	if handlerName == "" {
		return sql.Out{Dest: dest, In: in}, nil, nil
	}
	handler, ok := s.typeHandlers[handlerName]
	if !ok {
		return sql.Out{}, nil, fmt.Errorf("type-handler %q is not registered", handlerName)
	}
	holder := any(nil)
	if in {
		current, err := pointerValue(dest)
		if err != nil {
			return sql.Out{}, nil, err
		}
		converted, err := handler.ToDB(ctx, current)
		if err != nil {
			return sql.Out{}, nil, err
		}
		holder = converted
	}
	binding := &callOutBinding{
		name:        strings.TrimSpace(parameter.Name),
		dest:        dest,
		holder:      &holder,
		handler:     handler,
		handlerName: handlerName,
	}
	return sql.Out{Dest: &holder, In: in}, binding, nil
}

func applyCallOutBindings(ctx context.Context, bindings []callOutBinding) error {
	for _, binding := range bindings {
		if binding.handler == nil {
			continue
		}
		if err := binding.handler.FromDB(ctx, *binding.holder, binding.dest); err != nil {
			return fmt.Errorf("type-handler %q failed for OUT parameter %q: %w", binding.handlerName, binding.name, err)
		}
	}
	return nil
}

func isNonNilPointer(value any) bool {
	if value == nil {
		return false
	}
	rv := reflect.ValueOf(value)
	return rv.Kind() == reflect.Pointer && !rv.IsNil()
}

func pointerValue(value any) (any, error) {
	if !isNonNilPointer(value) {
		return nil, fmt.Errorf("destination must be non-nil pointer")
	}
	return reflect.ValueOf(value).Elem().Interface(), nil
}

// Call 会先刷新批处理队列，再转发存储过程调用。
func (s *BatchSession) Call(ctx context.Context, statement string, args NamedArgs, resultSets ...any) (CallResult, error) {
	if _, err := s.Flush(ctx); err != nil {
		return CallResult{}, err
	}
	callSession, ok := s.session.(CallSession)
	if !ok {
		return CallResult{}, configurationErrorf("batch delegate does not support stored procedure call")
	}
	return callSession.Call(ctx, statement, args, resultSets...)
}

// CallStatement 会先刷新批处理队列，再转发 StatementMeta 调用。
func (s *BatchSession) CallStatement(ctx context.Context, statement StatementMeta, args NamedArgs, resultSets ...any) (CallResult, error) {
	if _, err := s.Flush(ctx); err != nil {
		return CallResult{}, err
	}
	callSession, ok := s.session.(StatementCallSession)
	if !ok {
		return CallResult{}, configurationErrorf("batch delegate does not support statement stored procedure call")
	}
	return callSession.CallStatement(ctx, statement, args, resultSets...)
}

// Call 执行事务内存储过程调用。
func (s *TxSession) Call(ctx context.Context, statement string, args NamedArgs, resultSets ...any) (CallResult, error) {
	if err := s.ensureActive(); err != nil {
		return CallResult{}, err
	}
	return s.session.Call(ctx, statement, args, resultSets...)
}

// CallStatement 执行事务内 StatementMeta 存储过程调用。
func (s *TxSession) CallStatement(ctx context.Context, statement StatementMeta, args NamedArgs, resultSets ...any) (CallResult, error) {
	if err := s.ensureActive(); err != nil {
		return CallResult{}, err
	}
	return s.session.CallStatement(ctx, statement, args, resultSets...)
}

func (r *cancelRows) NextResultSet() bool {
	if r == nil || r.Rows == nil {
		return false
	}
	nextRows, ok := r.Rows.(interface{ NextResultSet() bool })
	if !ok {
		return false
	}
	return nextRows.NextResultSet()
}

var (
	_ CallSession          = (*SQLSession)(nil)
	_ StatementCallSession = (*SQLSession)(nil)
	_ CallSession          = (*BatchSession)(nil)
	_ StatementCallSession = (*BatchSession)(nil)
	_ CallSession          = (*TxSession)(nil)
	_ StatementCallSession = (*TxSession)(nil)
	_ ResultSetRows        = (*cancelRows)(nil)
)
