package orm

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var statementParamPattern = regexp.MustCompile(`#\{\s*([^{}]+?)\s*\}`)
var rawSQLParamPattern = regexp.MustCompile(`\$\{\s*([^{}]+?)\s*\}`)

// CompiledSQL 表示已经完成占位符改写和参数排序的 SQL。
type CompiledSQL struct {
	SQL  string
	Args []any
}

// CompileSQL 将 #{name} 改写为数据库方言占位符，并按出现顺序绑定参数。
func CompileSQL(query string, args NamedArgs, dialect Dialect) (CompiledSQL, error) {
	return CompileSQLContext(context.Background(), query, args, dialect)
}

// CompileSQLContext 将 #{name} 改写为数据库方言占位符，并使用调用方上下文完成参数转换。
func CompileSQLContext(ctx context.Context, query string, args NamedArgs, dialect Dialect) (CompiledSQL, error) {
	if ctx == nil {
		return CompiledSQL{}, bindingErrorf("context is nil")
	}
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	rendered, err := renderRawSQL(query, args, dialect)
	if err != nil {
		return CompiledSQL{}, err
	}
	query = rendered
	matches := statementParamPattern.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		if strings.Contains(query, "#{") {
			return CompiledSQL{}, bindingErrorf("SQL contains invalid parameter placeholder")
		}
		return CompiledSQL{SQL: query}, nil
	}

	var builder strings.Builder
	builder.Grow(len(query))
	compiled := CompiledSQL{Args: make([]any, 0, len(matches))}
	offset := 0
	for index, match := range matches {
		builder.WriteString(query[offset:match[0]])
		name := strings.TrimSpace(query[match[2]:match[3]])
		value, ok, err := resolveNamedArg(args, name)
		if err != nil {
			return CompiledSQL{}, &BindingError{Parameter: name, Err: err}
		}
		if !ok {
			return CompiledSQL{}, &BindingError{
				Parameter: name,
				Message:   fmt.Sprintf("SQL parameter %q is missing", name),
			}
		}
		value, err = databaseEnumValue(ctx, value)
		if err != nil {
			return CompiledSQL{}, &BindingError{Parameter: name, Err: err}
		}
		builder.WriteString(dialect.Placeholder(index + 1))
		compiled.Args = append(compiled.Args, value)
		offset = match[1]
	}
	builder.WriteString(query[offset:])
	compiled.SQL = builder.String()
	if strings.Contains(compiled.SQL, "#{") {
		return CompiledSQL{}, bindingErrorf("SQL contains invalid parameter placeholder")
	}
	return compiled, nil
}

func renderRawSQL(query string, args NamedArgs, dialect Dialect) (string, error) {
	matches := rawSQLParamPattern.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		if strings.Contains(query, "${") {
			return "", bindingErrorf("SQL contains invalid raw placeholder")
		}
		return query, nil
	}
	var builder strings.Builder
	builder.Grow(len(query))
	offset := 0
	for _, match := range matches {
		builder.WriteString(query[offset:match[0]])
		name := strings.TrimSpace(query[match[2]:match[3]])
		value, ok, err := resolveNamedArg(args, name)
		if err != nil {
			return "", &BindingError{Parameter: name, Err: err}
		}
		if !ok {
			return "", &BindingError{
				Parameter: name,
				Message:   fmt.Sprintf("raw SQL parameter %q is missing", name),
			}
		}
		token, ok := value.(RawSQLToken)
		if !ok {
			return "", &BindingError{
				Parameter: name,
				Message:   fmt.Sprintf("raw SQL parameter %q must use orm.RawSQLToken", name),
			}
		}
		rendered, err := token.renderRawSQL(dialect)
		if err != nil {
			return "", &BindingError{Parameter: name, Err: err}
		}
		builder.WriteString(rendered)
		offset = match[1]
	}
	builder.WriteString(query[offset:])
	rendered := builder.String()
	if strings.Contains(rendered, "${") {
		return "", bindingErrorf("SQL contains invalid raw placeholder")
	}
	return rendered, nil
}
