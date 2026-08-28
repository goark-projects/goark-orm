package runtime

import (
	"errors"
	"fmt"
	"strings"
)

const ormErrorPrefix = "goark-orm: "

var (
	// ErrORM 是 goark-orm 所有结构化错误的根分类。
	ErrORM = errors.New("goark-orm: orm error")
	// ErrConfiguration 表示运行期配置、选项或框架装配错误。
	ErrConfiguration = errors.New("goark-orm: configuration error")
	// ErrRegistry 表示元数据注册和查找错误。
	ErrRegistry = errors.New("goark-orm: registry error")
	// ErrStatementNotFound 表示 Mapper Statement 或 Provider 未注册。
	ErrStatementNotFound = errors.New("goark-orm: statement not found")
	// ErrBinding 表示 SQL 参数解析、绑定或占位符编译错误。
	ErrBinding = errors.New("goark-orm: binding error")
	// ErrMapping 表示结果集扫描、字段赋值或 TypeHandler 结果转换错误。
	ErrMapping = errors.New("goark-orm: mapping error")
	// ErrExecutor 表示数据库执行、预编译语句或行集生命周期错误。
	ErrExecutor = errors.New("goark-orm: executor error")
	// ErrTooManyResults 表示 QueryOne 返回了超过一行结果。
	ErrTooManyResults = errors.New("goark-orm: too many results")
)

// ConfigurationError 携带配置项名称和底层原因。
type ConfigurationError struct {
	Option  string
	Message string
	Err     error
}

func (e *ConfigurationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		option := strings.TrimSpace(e.Option)
		if option == "" {
			message = "configuration error"
		} else {
			message = fmt.Sprintf("configuration %s is invalid", option)
		}
	}
	return formatORMError(message, e.Err)
}

func (e *ConfigurationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *ConfigurationError) Is(target error) bool {
	return target == ErrORM || target == ErrConfiguration
}

// RegistryError 携带元数据资源类型、名称和底层原因。
type RegistryError struct {
	Resource string
	Name     string
	Message  string
	Err      error
}

func (e *RegistryError) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		resource := strings.TrimSpace(e.Resource)
		name := strings.TrimSpace(e.Name)
		switch {
		case resource != "" && name != "":
			message = fmt.Sprintf("%s %q registry error", resource, name)
		case resource != "":
			message = resource + " registry error"
		default:
			message = "registry error"
		}
	}
	return formatORMError(message, e.Err)
}

func (e *RegistryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *RegistryError) Is(target error) bool {
	return target == ErrORM || target == ErrRegistry
}

// StatementNotFoundError 表示指定 Statement 或 SQL Provider 没有注册。
type StatementNotFoundError struct {
	Statement string
	Message   string
	Err       error
}

func (e *StatementNotFoundError) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		statement := strings.TrimSpace(e.Statement)
		if statement == "" {
			message = "statement is not registered"
		} else {
			message = fmt.Sprintf("statement %q is not registered", statement)
		}
	}
	return formatORMError(message, e.Err)
}

func (e *StatementNotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *StatementNotFoundError) Is(target error) bool {
	return target == ErrORM || target == ErrStatementNotFound
}

// BindingError 携带参数绑定和 SQL 编译阶段的定位信息。
type BindingError struct {
	Statement string
	Operation string
	Parameter string
	Column    string
	Field     string
	Message   string
	Err       error
}

func (e *BindingError) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		operation := strings.TrimSpace(e.Operation)
		if operation == "" {
			operation = "bind"
		}
		statement := strings.TrimSpace(e.Statement)
		if statement == "" {
			message = operation + " failed"
		} else {
			message = fmt.Sprintf("%s statement %s failed", operation, statement)
		}
		message = appendErrorLocation(message, e.Parameter, e.Column, e.Field)
	}
	return formatORMError(message, e.Err)
}

func (e *BindingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *BindingError) Is(target error) bool {
	return target == ErrORM || target == ErrBinding
}

// MappingError 携带结果映射阶段的 Statement、列名、字段名和 resultMap 信息。
type MappingError struct {
	Statement string
	ResultMap string
	Column    string
	Field     string
	Message   string
	Err       error
}

func (e *MappingError) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		statement := strings.TrimSpace(e.Statement)
		if statement == "" {
			message = "result mapping failed"
		} else {
			message = fmt.Sprintf("map statement %s result failed", statement)
		}
		if resultMap := strings.TrimSpace(e.ResultMap); resultMap != "" {
			message += " for resultMap " + resultMap
		}
		message = appendErrorLocation(message, "", e.Column, e.Field)
	}
	return formatORMError(message, e.Err)
}

func (e *MappingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *MappingError) Is(target error) bool {
	return target == ErrORM || target == ErrMapping
}

// TooManyResultsError 表示单行查询返回了超过一行结果。
type TooManyResultsError struct {
	Statement string
	Expected  int
	Actual    int
	Message   string
	Err       error
}

func (e *TooManyResultsError) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		statement := strings.TrimSpace(e.Statement)
		if statement == "" {
			message = "query returned more than one row"
		} else {
			message = fmt.Sprintf("statement %s returned more than one row", statement)
		}
		if e.Actual > 0 && e.Expected > 0 {
			message = fmt.Sprintf("%s; expected %d, got %d", message, e.Expected, e.Actual)
		}
	}
	return formatORMError(message, e.Err)
}

func (e *TooManyResultsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *TooManyResultsError) Is(target error) bool {
	return target == ErrORM || target == ErrTooManyResults
}

// ExecutorError 携带数据库执行阶段的操作、SQL 和底层原因。
type ExecutorError struct {
	Statement string
	Operation string
	SQL       string
	Message   string
	Err       error
}

func (e *ExecutorError) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		operation := strings.TrimSpace(e.Operation)
		if operation == "" {
			operation = "execute"
		}
		statement := strings.TrimSpace(e.Statement)
		if statement == "" {
			message = operation + " failed"
		} else {
			message = fmt.Sprintf("%s statement %s failed", operation, statement)
		}
	}
	return formatORMError(message, e.Err)
}

func (e *ExecutorError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *ExecutorError) Is(target error) bool {
	return target == ErrORM || target == ErrExecutor
}

func configurationErrorf(format string, args ...any) error {
	return &ConfigurationError{Message: fmt.Sprintf(format, args...)}
}

func registryErrorf(resource string, name string, format string, args ...any) error {
	return &RegistryError{
		Resource: resource,
		Name:     name,
		Message:  fmt.Sprintf(format, args...),
	}
}

func statementNotFoundError(statement string) error {
	return &StatementNotFoundError{Statement: statement}
}

func statementNotFoundErrorf(statement string, format string, args ...any) error {
	return &StatementNotFoundError{
		Statement: statement,
		Message:   fmt.Sprintf(format, args...),
	}
}

func bindingErrorf(format string, args ...any) error {
	return &BindingError{Message: fmt.Sprintf(format, args...)}
}

func bindingFailure(statement string, operation string, err error) error {
	if err == nil {
		return nil
	}
	out := &BindingError{
		Statement: statement,
		Operation: operation,
		Err:       err,
	}
	var typed *BindingError
	if errors.As(err, &typed) {
		out.Parameter = typed.Parameter
		out.Column = typed.Column
		out.Field = typed.Field
	}
	return out
}

func mappingErrorf(format string, args ...any) error {
	return &MappingError{Message: fmt.Sprintf(format, args...)}
}

func mappingFailure(statement StatementMeta, err error) error {
	if err == nil {
		return nil
	}
	var typed *MappingError
	if errors.As(err, &typed) {
		if strings.TrimSpace(typed.Statement) != "" {
			return err
		}
		out := *typed
		out.Statement = statement.FullName
		return &out
	}
	if errors.Is(err, ErrConfiguration) ||
		errors.Is(err, ErrStatementNotFound) ||
		errors.Is(err, ErrBinding) ||
		errors.Is(err, ErrExecutor) ||
		errors.Is(err, ErrTooManyResults) {
		return err
	}
	return &MappingError{Statement: statement.FullName, ResultMap: statement.ResultMap, Err: err}
}

func executorFailure(statement StatementMeta, operation string, compiled CompiledSQL, err error) error {
	if err == nil {
		return nil
	}
	return &ExecutorError{
		Statement: statement.FullName,
		Operation: operation,
		SQL:       compiled.SQL,
		Err:       err,
	}
}

func formatORMError(message string, err error) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "error"
	}
	if !strings.HasPrefix(message, ormErrorPrefix) {
		message = ormErrorPrefix + message
	}
	if err == nil {
		return message
	}
	cause := strings.TrimSpace(err.Error())
	cause = strings.TrimPrefix(cause, ormErrorPrefix)
	if cause == "" {
		return message
	}
	return message + ": " + cause
}

func appendErrorLocation(message string, parameter string, column string, field string) string {
	if parameter = strings.TrimSpace(parameter); parameter != "" {
		message += fmt.Sprintf(" for parameter %q", parameter)
	}
	if column = strings.TrimSpace(column); column != "" {
		message += fmt.Sprintf(" for column %q", column)
	}
	if field = strings.TrimSpace(field); field != "" {
		message += " for field " + field
	}
	return message
}
