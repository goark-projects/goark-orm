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

func BenchmarkSQLSessionQueryOneGeneratedRowScanner(b *testing.B) {
	registry := newSQLSessionRegistry(b, StatementMeta{
		ID:         "FindByID",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.FindByID",
		Command:    StatementCommandSelect,
		SQL:        "select id, name, status from sys_user where id = #{id}",
		ResultType: "sqlSessionUser",
	})
	if err := registry.RegisterRowScanner("sqlSessionUser", RowScannerFunc(func(ctx context.Context, columns []string, row RowScannerRow, dest any) error {
		_ = ctx
		_ = columns
		user := dest.(*sqlSessionUser)
		var discard any
		return row.Scan(&user.ID, &user.Name, &discard)
	})); err != nil {
		b.Fatalf("register row scanner failed: %v", err)
	}
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

func BenchmarkSQLSessionQueryOneResultMapTypeHandler(b *testing.B) {
	registry := newSQLSessionRegistry(b, StatementMeta{
		ID:        "FindProfile",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindProfile",
		Command:   StatementCommandSelect,
		SQL:       "select id, profile from sys_user where id = #{id}",
		ResultMap: "UserResult",
	})
	state := openTestSQLState(b)
	state.queryRows = testRowsData{
		columns: []string{"id", "profile"},
		values:  [][]driver.Value{{int64(7), []byte("profile-fast-path")}},
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithLocalCache(false), WithTypeHandler("profile", profileTypeHandler{}))
	if err != nil {
		b.Fatalf("new SQL session failed: %v", err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var user sqlSessionUser
		if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindProfile", NamedArgs{"id": int64(7)}, &user); err != nil {
			b.Fatalf("query one failed: %v", err)
		}
		if user.ID != 7 || user.Profile.Text != "profile-fast-path" {
			b.Fatalf("unexpected user %#v", user)
		}
	}
}

func BenchmarkAppendSQLCondition_GroupedQuery(b *testing.B) {
	query := `select status, count(*) from sys_user where active = #{active} and exists (select 1 from audit_log where audit_log.user_id = sys_user.id and audit_log.type = #{type}) group by status having count(*) > #{min} order by status`
	condition := `"tenant_id" = #{tenantID}`
	expected := `select status, count(*) from sys_user where active = #{active} and exists (select 1 from audit_log where audit_log.user_id = sys_user.id and audit_log.type = #{type}) AND "tenant_id" = #{tenantID} group by status having count(*) > #{min} order by status`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		actual := appendSQLCondition(query, condition)
		if actual != expected {
			b.Fatalf("unexpected SQL %q", actual)
		}
	}
}

func BenchmarkCountSQLBase_NestedOrderBy(b *testing.B) {
	query := `select * from (select id, user_id from audit_log where level = #{level} order by id) audit where audit.user_id = #{userID} and audit.created_at >= #{begin} order by audit.id`
	expected := `select * from (select id, user_id from audit_log where level = #{level} order by id) audit where audit.user_id = #{userID} and audit.created_at >= #{begin}`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		actual := countSQLBase(query)
		if actual != expected {
			b.Fatalf("unexpected SQL %q", actual)
		}
	}
}

func BenchmarkResultObjectKey_Primitives(b *testing.B) {
	fields := []ResultFieldMeta{
		{Column: "id", ID: true},
		{Column: "tenant_id", ID: true},
	}
	indexes := map[string]int{"id": 0, "tenant_id": 1}
	values := []any{int64(1001), "tenant-alpha"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := resultObjectKey(fields, indexes, values)
		if key == "" {
			b.Fatalf("expected key")
		}
	}
}

func BenchmarkContainsOrderBy_BracketIdentifiers(b *testing.B) {
	query := `select [id], [order]],name] from [sys_user] where [where] = #{where} order by [id]`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !containsOrderBy(query) {
			b.Fatalf("expected order by")
		}
	}
}

func BenchmarkJSONTypeHandler_ToDB(b *testing.B) {
	handler := NewJSONTypeHandler()
	value := benchmarkJSONProfile{
		ID:    1001,
		Name:  "Alice",
		Roles: []string{"admin", "auditor", "operator"},
		Settings: benchmarkJSONSettings{
			Theme:         "dark",
			Notifications: true,
			Limits:        []int{10, 20, 50},
		},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		databaseValue, err := handler.ToDB(context.Background(), value)
		if err != nil {
			b.Fatalf("json ToDB failed: %v", err)
		}
		if len(databaseValue.([]byte)) == 0 {
			b.Fatalf("expected encoded json")
		}
	}
}

func BenchmarkJSONTypeHandler_FromDB(b *testing.B) {
	handler := NewJSONTypeHandler()
	data := []byte(`{"id":1001,"name":"Alice","roles":["admin","auditor","operator"],"settings":{"theme":"dark","notifications":true,"limits":[10,20,50]}}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var value benchmarkJSONProfile
		if err := handler.FromDB(context.Background(), data, &value); err != nil {
			b.Fatalf("json FromDB failed: %v", err)
		}
		if value.ID != 1001 || value.Settings.Theme != "dark" {
			b.Fatalf("unexpected decoded value %#v", value)
		}
	}
}

type benchmarkJSONProfile struct {
	ID       int64                 `json:"id"`
	Name     string                `json:"name"`
	Roles    []string              `json:"roles"`
	Settings benchmarkJSONSettings `json:"settings"`
}

type benchmarkJSONSettings struct {
	Theme         string `json:"theme"`
	Notifications bool   `json:"notifications"`
	Limits        []int  `json:"limits"`
}
