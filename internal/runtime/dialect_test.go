package runtime

import "testing"

func TestDialect_whenDbTypesProvided_shouldRenderDatabaseSpecificSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dialect     Dialect
		identifier  string
		placeholder string
		quoted      string
		paged       string
	}{
		{
			name:        "mysql",
			dialect:     NewMySQLDialect(),
			identifier:  "sys`user",
			placeholder: "?",
			quoted:      "`sys``user`",
			paged:       "select * from sys_user LIMIT ? OFFSET ?",
		},
		{
			name:        "mariadb",
			dialect:     NewMariaDBDialect(),
			identifier:  "sys`user",
			placeholder: "?",
			quoted:      "`sys``user`",
			paged:       "select * from sys_user LIMIT ? OFFSET ?",
		},
		{
			name:        "sqlite",
			dialect:     NewSQLiteDialect(),
			identifier:  `sys"user`,
			placeholder: "?",
			quoted:      `"sys""user"`,
			paged:       "select * from sys_user LIMIT ? OFFSET ?",
		},
		{
			name:        "sqlserver",
			dialect:     NewSQLServerDialect(),
			identifier:  "sys]user",
			placeholder: "@p3",
			quoted:      "[sys]]user]",
			paged:       "select * from sys_user ORDER BY (SELECT 0) OFFSET @p2 ROWS FETCH NEXT @p1 ROWS ONLY",
		},
		{
			name:        "oracle",
			dialect:     NewOracleDialect(),
			identifier:  `sys"user`,
			placeholder: ":3",
			quoted:      `"sys""user"`,
			paged:       "select * from sys_user OFFSET :2 ROWS FETCH NEXT :1 ROWS ONLY",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.dialect.Placeholder(3) != tt.placeholder {
				t.Fatalf("unexpected placeholder %q", tt.dialect.Placeholder(3))
			}
			if quoted := tt.dialect.QuoteIdent(tt.identifier); quoted != tt.quoted {
				t.Fatalf("unexpected quoted identifier %q", quoted)
			}
			if paged := limitOffsetSQL(tt.dialect, "select * from sys_user", tt.dialect.Placeholder(1), tt.dialect.Placeholder(2)); paged != tt.paged {
				t.Fatalf("unexpected paged SQL %q", paged)
			}
		})
	}
}

func TestNewDialect_whenAliasProvided_shouldResolveDbType(t *testing.T) {
	t.Parallel()

	tests := map[string]DbType{
		"postgresql": DbTypePostgres,
		"pg":         DbTypePostgres,
		"mysql":      DbTypeMySQL,
		"mariadb":    DbTypeMariaDB,
		"sqlite3":    DbTypeSQLite,
		"mssql":      DbTypeSQLServer,
		"oracle":     DbTypeOracle,
		"":           DbTypeQuestion,
	}

	for input, expected := range tests {
		input := input
		expected := expected
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			actual, err := ParseDbType(input)
			if err != nil {
				t.Fatalf("parse db type failed: %v", err)
			}
			if actual != expected {
				t.Fatalf("unexpected db type %q", actual)
			}
			dialect, err := NewDialect(actual)
			if err != nil {
				t.Fatalf("new dialect failed: %v", err)
			}
			if dialect == nil {
				t.Fatalf("expected dialect")
			}
		})
	}
}

func TestNewDialect_whenUnsupportedDbTypeProvided_shouldReturnError(t *testing.T) {
	t.Parallel()

	if _, err := NewDialect(DbType("db2")); err == nil {
		t.Fatalf("expected unsupported dialect error")
	}
	if _, err := ParseDbType("db2"); err == nil {
		t.Fatalf("expected unsupported db type error")
	}
}

func TestSQLServerDialect_LimitOffsetSQL_whenOrderByUsesFlexibleWhitespace_shouldKeepOrder(t *testing.T) {
	t.Parallel()

	query := "select * from sys_user\nORDER\tBY id"
	paged := limitOffsetSQL(NewSQLServerDialect(), query, "@p1", "@p2")
	expected := "select * from sys_user\nORDER\tBY id OFFSET @p2 ROWS FETCH NEXT @p1 ROWS ONLY"
	if paged != expected {
		t.Fatalf("unexpected paged SQL %q", paged)
	}
}

func TestSQLServerDialect_LimitOffsetSQL_whenOrderIdentifierQuoted_shouldAppendFallbackOrder(t *testing.T) {
	t.Parallel()

	query := "select [order] from sys_user"
	paged := limitOffsetSQL(NewSQLServerDialect(), query, "@p1", "@p2")
	expected := "select [order] from sys_user ORDER BY (SELECT 0) OFFSET @p2 ROWS FETCH NEXT @p1 ROWS ONLY"
	if paged != expected {
		t.Fatalf("unexpected paged SQL %q", paged)
	}
}

func TestCountSQLBase_whenNestedOrderByProvided_shouldStripOnlyTopLevelTail(t *testing.T) {
	t.Parallel()

	query := "select * from (select id from audit_log order by id) audit where audit.id > #{id} order by audit.id"
	expected := "select * from (select id from audit_log order by id) audit where audit.id > #{id}"
	if actual := countSQLBase(query); actual != expected {
		t.Fatalf("unexpected count SQL base %q", actual)
	}
}

func TestCountSQLBase_whenGroupedQueryProvided_shouldKeepGroupingAndStripOrder(t *testing.T) {
	t.Parallel()

	query := "select status, count(*) from sys_user where active = #{active} group by status having count(*) > 1 order by status"
	expected := "select status, count(*) from sys_user where active = #{active} group by status having count(*) > 1"
	if actual := countSQLBase(query); actual != expected {
		t.Fatalf("unexpected count SQL base %q", actual)
	}
}

func TestCountSQLBase_whenTailKeywordIsPredicateColumn_shouldStripOnlyRealTail(t *testing.T) {
	t.Parallel()

	query := "select id from sys_user where order = #{order} order by id"
	expected := "select id from sys_user where order = #{order}"
	if actual := countSQLBase(query); actual != expected {
		t.Fatalf("unexpected count SQL base %q", actual)
	}
}

func TestCountSQLBase_whenTailKeywordsAreProjectionColumns_shouldStripOnlyRealTail(t *testing.T) {
	t.Parallel()

	query := "select limit, offset from sys_user where status = #{status} order by id"
	expected := "select limit, offset from sys_user where status = #{status}"
	if actual := countSQLBase(query); actual != expected {
		t.Fatalf("unexpected count SQL base %q", actual)
	}
}

func TestCountSQLBase_whenLimitOffsetTailProvided_shouldStripTail(t *testing.T) {
	t.Parallel()

	query := "select id from sys_user where status = #{status} limit #{limit} offset #{offset}"
	expected := "select id from sys_user where status = #{status}"
	if actual := countSQLBase(query); actual != expected {
		t.Fatalf("unexpected count SQL base %q", actual)
	}
}

func TestCountSQLBase_whenCTESelectHasProjectionTailKeyword_shouldStripOnlyRealTail(t *testing.T) {
	t.Parallel()

	query := "with active_users as (select id from sys_user where active = true) select limit from active_users where id = #{id} order by id"
	expected := "with active_users as (select id from sys_user where active = true) select limit from active_users where id = #{id}"
	if actual := countSQLBase(query); actual != expected {
		t.Fatalf("unexpected count SQL base %q", actual)
	}
}
