package orm

import (
	"context"
	"database/sql/driver"
	"testing"
)

func BenchmarkCompileSQL_Postgres(b *testing.B) {
	args := NamedArgs{"id": int64(7), "status": "ACTIVE", "name": "Alice"}
	dialect := NewPostgresDialect()
	sqlText := "select id, name from sys_user where id = #{id} and status = #{status} and name = #{name}"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		compiled, err := CompileSQL(sqlText, args, dialect)
		if err != nil {
			b.Fatalf("compile SQL failed: %v", err)
		}
		if len(compiled.Args) != 3 {
			b.Fatalf("unexpected args %#v", compiled.Args)
		}
	}
}

func BenchmarkRenderDynamicSQL(b *testing.B) {
	nodes := []DynamicSQLNode{
		{Kind: DynamicSQLNodeText, Text: "select id, name from sys_user"},
		{
			Kind: DynamicSQLNodeWhere,
			Children: []DynamicSQLNode{
				{Kind: DynamicSQLNodeIf, Test: "status != nil and status != ''", Children: []DynamicSQLNode{{Kind: DynamicSQLNodeText, Text: "and status = #{status}"}}},
				{Kind: DynamicSQLNodeIf, Test: "ids.size > 0", Children: []DynamicSQLNode{
					{Kind: DynamicSQLNodeText, Text: "and id in"},
					{Kind: DynamicSQLNodeForeach, Collection: "ids", Item: "id", Open: "(", Separator: ",", Close: ")", Children: []DynamicSQLNode{{Kind: DynamicSQLNodeText, Text: "#{id}"}}},
				}},
			},
		},
	}
	args := NamedArgs{"status": "ACTIVE", "ids": []int64{1, 2, 3, 4, 5}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rendered, err := RenderDynamicSQL(nodes, args)
		if err != nil {
			b.Fatalf("render dynamic SQL failed: %v", err)
		}
		if rendered.SQL == "" {
			b.Fatalf("expected SQL")
		}
	}
}

func BenchmarkQueryWrapperBuild(b *testing.B) {
	dialect := NewPostgresDialect()
	wrapper := NewQueryWrapper[baseMapperUser]().
		Eq(baseMapperUserStatus, "ACTIVE").
		Like(baseMapperUserName, "%Alice%").
		In(baseMapperUserID, []int64{1, 2, 3, 4, 5}).
		OrderByDesc(baseMapperUserID)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rendered, err := wrapper.build(dialect, 0)
		if err != nil {
			b.Fatalf("build wrapper failed: %v", err)
		}
		if rendered.WhereSQL == "" {
			b.Fatalf("expected where SQL")
		}
	}
}

func BenchmarkSQLSessionQueryOneScan(b *testing.B) {
	registry := newSQLSessionRegistry(b, StatementMeta{
		ID:         "FindByID",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.FindByID",
		Command:    StatementCommandSelect,
		SQL:        "select id, name, status from sys_user where id = #{id}",
		ResultType: "sqlSessionUser",
	})
	state := openTestSQLState(b)
	state.queryRows = testRowsData{
		columns: []string{"id", "name", "status"},
		values:  [][]driver.Value{{int64(7), "Alice", "ACTIVE"}},
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithLocalCache(false))
	if err != nil {
		b.Fatalf("new SQL session failed: %v", err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var user sqlSessionUser
		if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &user); err != nil {
			b.Fatalf("query one failed: %v", err)
		}
		if user.ID != 7 {
			b.Fatalf("unexpected user %#v", user)
		}
	}
}
