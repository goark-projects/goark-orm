package runtime

import (
	"reflect"
	"testing"
)

func TestRenderDynamicSQL_whenWhereIfMatches_shouldRenderWhereClause(t *testing.T) {
	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{
					Kind: DynamicSQLNodeIf,
					Test: "status != nil and status != ''",
					Children: []DynamicSQLNode{
						{Kind: DynamicSQLNodeText, Text: "and status = #{status}"},
					},
				},
			},
		},
	}, NamedArgs{"status": "ACTIVE"})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}

	if rendered.SQL != "select id from sys_user WHERE status = #{status}" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
	if !reflect.DeepEqual(rendered.Args, NamedArgs{"status": "ACTIVE"}) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestRenderDynamicSQL_whenWhereIfSkipped_shouldOmitWhereClause(t *testing.T) {
	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{
					Kind: DynamicSQLNodeIf,
					Test: "status != nil and status != ''",
					Children: []DynamicSQLNode{
						{Kind: DynamicSQLNodeText, Text: "and status = #{status}"},
					},
				},
			},
		},
	}, NamedArgs{"status": ""})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}

	if rendered.SQL != "select id from sys_user" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
}

func TestRenderDynamicSQL_whenForeachCollection_shouldExpandItems(t *testing.T) {
	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user where id in"},
		{
			Kind:       DynamicSQLNodeForeach,
			Collection: "ids",
			Item:       "id",
			Open:       "(",
			Separator:  ",",
			Close:      ")",
			Children: []DynamicSQLNode{
				{Kind: DynamicSQLNodeText, Text: "#{id}"},
			},
		},
	}, NamedArgs{"ids": []int64{7, 8}})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}

	if rendered.SQL != "select id from sys_user where id in (#{__goark_orm_id_0}, #{__goark_orm_id_1})" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
	expectedArgs := NamedArgs{
		"ids":              []int64{7, 8},
		"__goark_orm_id_0": int64(7),
		"__goark_orm_id_1": int64(8),
	}
	if !reflect.DeepEqual(rendered.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", rendered.Args)
	}
}

func TestRenderDynamicSQLWithOptions_whenForeachCollectionNilAndNullableDisabled_shouldReject(t *testing.T) {
	_, err := RenderDynamicSQLWithOptions([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user where id in"},
		{
			Kind:       DynamicSQLNodeForeach,
			Collection: "ids",
			Item:       "id",
			Open:       "(",
			Separator:  ",",
			Close:      ")",
			Children: []DynamicSQLNode{
				{Kind: DynamicSQLNodeText, Text: "#{id}"},
			},
		},
	}, NamedArgs{"ids": []int64(nil)}, DynamicSQLRenderOptions{NullableOnForEach: false})
	if err == nil {
		t.Fatalf("expected nil foreach collection to be rejected")
	}
}

func TestRenderDynamicSQLWithOptions_whenForeachNodeAllowsNullable_shouldOverrideDefault(t *testing.T) {
	nullable := true
	rendered, err := RenderDynamicSQLWithOptions([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user where id in"},
		{
			Kind:       DynamicSQLNodeForeach,
			Collection: "ids",
			Nullable:   &nullable,
			Open:       "(",
			Close:      ")",
			Children: []DynamicSQLNode{
				{Kind: DynamicSQLNodeText, Text: "#{id}"},
			},
		},
	}, NamedArgs{}, DynamicSQLRenderOptions{NullableOnForEach: false})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}
	if rendered.SQL != "select id from sys_user where id in" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
}

func TestRenderDynamicSQLWithOptions_whenShrinkWhitespaceEnabled_shouldCollapseFinalSQL(t *testing.T) {
	rendered, err := RenderDynamicSQLWithOptions([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select  id\nfrom   sys_user"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{Kind: DynamicSQLNodeText, Text: "\n  status   =   #{status}  "},
			},
		},
	}, NamedArgs{"status": "ACTIVE"}, DynamicSQLRenderOptions{
		NullableOnForEach:      true,
		ShrinkWhitespacesInSQL: true,
	})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}
	if rendered.SQL != "select id from sys_user WHERE status = #{status}" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
}

func TestRenderDynamicSQL_whenSetNode_shouldTrimTrailingComma(t *testing.T) {
	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "update sys_user"},
		{
			Kind: DynamicSQLNodeSet,
			Children: []DynamicSQLNode{
				{
					Kind: DynamicSQLNodeIf,
					Test: "name != nil and name != ''",
					Children: []DynamicSQLNode{
						{Kind: DynamicSQLNodeText, Text: "name = #{name},"},
					},
				},
				{
					Kind: DynamicSQLNodeIf,
					Test: "status != nil and status != ''",
					Children: []DynamicSQLNode{
						{Kind: DynamicSQLNodeText, Text: "status = #{status},"},
					},
				},
			},
		},
		{Kind: DynamicSQLNodeText, Text: "where id = #{id}"},
	}, NamedArgs{"id": int64(7), "name": "Alice", "status": ""})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}

	if rendered.SQL != "update sys_user SET name = #{name} where id = #{id}" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
}

func TestRenderDynamicSQL_whenChooseMatchesFirstWhen_shouldSkipOtherwise(t *testing.T) {
	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{
					Kind: DynamicSQLNodeChoose,
					Children: []DynamicSQLNode{
						{
							Kind: DynamicSQLNodeWhen,
							Test: "status == 'ACTIVE'",
							Children: []DynamicSQLNode{
								{Kind: DynamicSQLNodeText, Text: "and status = #{status}"},
							},
						},
						{
							Kind: DynamicSQLNodeOtherwise,
							Children: []DynamicSQLNode{
								{Kind: DynamicSQLNodeText, Text: "and deleted = false"},
							},
						},
					},
				},
			},
		},
	}, NamedArgs{"status": "ACTIVE"})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}

	if rendered.SQL != "select id from sys_user WHERE status = #{status}" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
}

func TestRenderDynamicSQL_whenBindProvided_shouldExposeBoundArgument(t *testing.T) {
	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user"},
		{Kind: DynamicSQLNodeBind, Name: "pattern", Value: "'%' + name + '%'"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{Kind: DynamicSQLNodeText, Text: "name like #{pattern}"},
			},
		},
	}, NamedArgs{"name": "Alice"})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}

	if rendered.SQL != "select id from sys_user WHERE name like #{pattern}" {
		t.Fatalf("unexpected SQL %q", rendered.SQL)
	}
	if rendered.Args["pattern"] != "%Alice%" {
		t.Fatalf("unexpected bind value %#v", rendered.Args["pattern"])
	}
}

func TestRenderDynamicSQL_whenNestedPathsAndForeachAliasesProvided_shouldRenderCompilableSQL(t *testing.T) {
	rendered, err := RenderDynamicSQL([]DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id from sys_user"},
		{Kind: DynamicSQLNodeBind, Name: "pattern", Value: "'%' + user.name + '%'"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{
					Kind: DynamicSQLNodeIf,
					Test: "filter.status != nil and user.name != ''",
					Children: []DynamicSQLNode{
						{Kind: DynamicSQLNodeText, Text: "and status = #{filter.status} and name like #{pattern} and id in"},
						{
							Kind:       DynamicSQLNodeForeach,
							Collection: "users",
							Item:       "item",
							Index:      "idx",
							Open:       "(",
							Separator:  ",",
							Close:      ")",
							Children: []DynamicSQLNode{
								{Kind: DynamicSQLNodeText, Text: "#{idx} + #{item.ID}"},
							},
						},
					},
				},
			},
		},
	}, NamedArgs{
		"user":   sqlSessionUser{Name: "Alice"},
		"filter": map[string]any{"status": "ACTIVE"},
		"users": []sqlSessionUser{
			{ID: 7},
			{ID: 8},
		},
	})
	if err != nil {
		t.Fatalf("render dynamic SQL failed: %v", err)
	}

	compiled, err := CompileSQL(rendered.SQL, rendered.Args, NewQuestionDialect())
	if err != nil {
		t.Fatalf("compile rendered dynamic SQL failed: %v", err)
	}
	expectedArgs := []any{"ACTIVE", "%Alice%", 0, int64(7), 1, int64(8)}
	if !reflect.DeepEqual(compiled.Args, expectedArgs) {
		t.Fatalf("unexpected compiled args %#v; SQL=%s renderedArgs=%#v", compiled.Args, compiled.SQL, rendered.Args)
	}
}
