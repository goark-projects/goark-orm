package orm

import (
	"strings"
	"testing"
)

func TestEvalExpression_whenParenthesesComparisonAndNotProvided_shouldEvaluateSafely(t *testing.T) {
	t.Parallel()

	args := NamedArgs{
		"status":  "ACTIVE",
		"age":     20,
		"admin":   false,
		"deleted": false,
	}
	ok, err := evalExpression("(status == 'ACTIVE' and age >= 18) and not deleted and admin == false", func(name string) (any, bool) {
		return args[name], args[name] != nil
	})
	if err != nil {
		t.Fatalf("eval expression failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected expression to match")
	}
}

func TestCompileExpressionPlan_whenRepeatedExpressionProvided_shouldReuseTokens(t *testing.T) {
	t.Parallel()

	first, err := compileExpressionPlan("status != nil and status != ''")
	if err != nil {
		t.Fatalf("compile first expression failed: %v", err)
	}
	second, err := compileExpressionPlan(" status != nil and status != '' ")
	if err != nil {
		t.Fatalf("compile second expression failed: %v", err)
	}
	if len(first.tokens) == 0 || len(second.tokens) == 0 {
		t.Fatalf("expected compiled tokens")
	}
	if &first.tokens[0] != &second.tokens[0] {
		t.Fatalf("expected cached expression tokens to be reused")
	}
	ok, err := evalExpression("status != nil and status != ''", func(name string) (any, bool) {
		if name == "status" {
			return "ACTIVE", true
		}
		return nil, false
	})
	if err != nil {
		t.Fatalf("eval cached expression failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected cached expression to match")
	}
}

func TestEvalExpression_whenCollectionSizeProvided_shouldCompareLength(t *testing.T) {
	t.Parallel()

	args := NamedArgs{
		"ids":   []int64{7, 8},
		"empty": []string{},
	}
	tests := []string{
		"ids.size > 0",
		"ids.length >= 2",
		"ids.size() == 2",
		"!empty",
		"empty.size == 0",
	}
	for _, expression := range tests {
		expression := expression
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			ok, err := evalExpression(expression, func(name string) (any, bool) {
				value, ok := args[name]
				if ok {
					return value, true
				}
				value, ok, err := resolveNamedArg(args, name)
				if err != nil || !ok {
					return nil, false
				}
				return value, true
			})
			if err != nil {
				t.Fatalf("eval expression failed: %v", err)
			}
			if !ok {
				t.Fatalf("expected expression %q to match", expression)
			}
		})
	}
}

func TestEvalExpression_whenLenFunctionProvided_shouldCompareLength(t *testing.T) {
	t.Parallel()

	args := NamedArgs{
		"ids":  []int64{7, 8, 9},
		"name": "Alice",
	}
	tests := []string{
		"len(ids) == 3",
		"len ( ids ) > 2",
		"len(name) >= 5",
	}
	for _, expression := range tests {
		expression := expression
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			ok, err := evalExpression(expression, func(name string) (any, bool) {
				value, ok := args[name]
				return value, ok
			})
			if err != nil {
				t.Fatalf("eval expression failed: %v", err)
			}
			if !ok {
				t.Fatalf("expected expression %q to match", expression)
			}
		})
	}
}

func TestEvalExpression_whenArithmeticTernaryAndMembershipProvided_shouldEvaluateOGNL(t *testing.T) {
	t.Parallel()

	args := NamedArgs{
		"age":    18,
		"bonus":  3,
		"status": "ACTIVE",
		"id":     int64(8),
		"ids":    []int64{7, 8, 9},
		"empty":  []string{},
	}
	tests := []string{
		"(age + bonus) >= 21",
		"(bonus * 2 + age) % 2 == 0",
		"status in {'ACTIVE', 'LOCKED'}",
		"status not in {'DELETED', 'DISABLED'}",
		"ids.contains(id)",
		"empty.isEmpty()",
		"(status == 'ACTIVE' ? age + bonus : 0) == 21",
	}
	for _, expression := range tests {
		expression := expression
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			ok, err := evalExpression(expression, func(name string) (any, bool) {
				value, ok, err := resolveNamedArg(args, name)
				if err != nil || !ok {
					return nil, false
				}
				return value, true
			})
			if err != nil {
				t.Fatalf("eval expression failed: %v", err)
			}
			if !ok {
				t.Fatalf("expected expression %q to match", expression)
			}
		})
	}
}

func TestEvalExpression_whenStringAndMapMethodsProvided_shouldEvaluateWhitelistMethods(t *testing.T) {
	t.Parallel()

	args := NamedArgs{
		"name": " Alice ",
		"meta": map[string]any{
			"tenant": "goark",
			"count":  int64(3),
		},
	}
	tests := []string{
		"name.trim().startsWith('Alice')",
		"name.trim().toLowerCase() == 'alice'",
		"name.trim().equalsIgnoreCase('alice')",
		"meta.containsKey('tenant')",
		"meta.containsValue(3)",
	}
	for _, expression := range tests {
		expression := expression
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			ok, err := evalExpression(expression, func(name string) (any, bool) {
				value, ok, err := resolveNamedArg(args, name)
				if err != nil || !ok {
					return nil, false
				}
				return value, true
			})
			if err != nil {
				t.Fatalf("eval expression failed: %v", err)
			}
			if !ok {
				t.Fatalf("expected expression %q to match", expression)
			}
		})
	}
}

func TestEvalValueExpression_whenTernaryAndStringConcatProvided_shouldReturnValue(t *testing.T) {
	t.Parallel()

	args := NamedArgs{
		"name": "",
	}
	value, err := evalValueExpression("'%' + (name != '' ? name : 'guest') + '%'", func(name string) (any, bool) {
		value, ok := args[name]
		return value, ok
	})
	if err != nil {
		t.Fatalf("eval value expression failed: %v", err)
	}
	if value != "%guest%" {
		t.Fatalf("unexpected value %#v", value)
	}
}

func TestEvalExpression_whenUnsafeMethodProvided_shouldReturnError(t *testing.T) {
	t.Parallel()

	_, err := evalExpression("name.Format()", func(name string) (any, bool) {
		if name == "name" {
			return "Alice", true
		}
		return nil, false
	})
	if err == nil || !strings.Contains(err.Error(), "method \"Format\" is not allowed") {
		t.Fatalf("expected disallowed method error, got %v", err)
	}
}

func TestEvalExpression_whenInvalidNumericLiteralProvided_shouldReturnError(t *testing.T) {
	t.Parallel()

	_, err := evalExpression("age > 1e", func(name string) (any, bool) {
		if name == "age" {
			return 20, true
		}
		return nil, false
	})
	if err == nil || !strings.Contains(err.Error(), "invalid dynamic SQL numeric literal") {
		t.Fatalf("expected invalid numeric literal error, got %v", err)
	}
}

func TestEvalExpression_whenInvalidStringLiteralProvided_shouldReturnError(t *testing.T) {
	t.Parallel()

	_, err := evalExpression(`name == "bad\q"`, func(name string) (any, bool) {
		if name == "name" {
			return "Alice", true
		}
		return nil, false
	})
	if err == nil || !strings.Contains(err.Error(), "invalid dynamic SQL string literal") {
		t.Fatalf("expected invalid string literal error, got %v", err)
	}
}

func TestRenderDynamicSQL_whenRichExpressionMatches_shouldRenderClause(t *testing.T) {
	t.Parallel()

	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{
					Kind: DynamicSQLNodeIf,
					Test: "(filter.status == 'ACTIVE' or admin == true) and ids.size > 0 and age < 65",
					Children: []DynamicSQLNode{
						{Kind: DynamicSQLNodeText, Text: "and status = #{filter.status}"},
					},
				},
			},
		},
	}, NamedArgs{
		"filter": map[string]any{"status": "ACTIVE"},
		"admin":  false,
		"ids":    []int64{7},
		"age":    42,
	})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}
	if rendered.SQL != "select id from sys_user WHERE status = #{filter.status}" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
}
