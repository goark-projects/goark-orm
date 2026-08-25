package orm

import (
	"fmt"
	"regexp"
	"strings"
)

var statementParamPattern = regexp.MustCompile(`#\{\s*([^{}]+?)\s*\}`)

// CompiledSQL 表示已经完成占位符改写和参数排序的 SQL。
type CompiledSQL struct {
	SQL  string
	Args []any
}

// CompileSQL 将 #{name} 改写为数据库方言占位符，并按出现顺序绑定参数。
func CompileSQL(query string, args NamedArgs, dialect Dialect) (CompiledSQL, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	if strings.Contains(query, "${") {
		return CompiledSQL{}, fmt.Errorf("goark-orm: SQL uses forbidden ${}")
	}
	matches := statementParamPattern.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		if strings.Contains(query, "#{") {
			return CompiledSQL{}, fmt.Errorf("goark-orm: SQL contains invalid parameter placeholder")
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
			return CompiledSQL{}, err
		}
		if !ok {
			return CompiledSQL{}, fmt.Errorf("goark-orm: SQL parameter %q is missing", name)
		}
		builder.WriteString(dialect.Placeholder(index + 1))
		compiled.Args = append(compiled.Args, value)
		offset = match[1]
	}
	builder.WriteString(query[offset:])
	compiled.SQL = builder.String()
	if strings.Contains(compiled.SQL, "#{") {
		return CompiledSQL{}, fmt.Errorf("goark-orm: SQL contains invalid parameter placeholder")
	}
	return compiled, nil
}
