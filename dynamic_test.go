package orm

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
