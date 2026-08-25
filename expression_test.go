package orm

import "testing"

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
