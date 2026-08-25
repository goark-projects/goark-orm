package orm

import (
	"reflect"
	"strings"
	"testing"
)

func TestCompileSQL_whenRawIdentifierProvided_shouldRenderQuotedIdentifier(t *testing.T) {
	t.Parallel()

	table, err := NewRawIdentifier("tenant_01.sys_user")
	if err != nil {
		t.Fatalf("new raw identifier failed: %v", err)
	}
	column, err := NewRawIdentifier("name")
	if err != nil {
		t.Fatalf("new raw identifier failed: %v", err)
	}
	compiled, err := CompileSQL(
		"select ${column} from ${table} where id = #{id}",
		NamedArgs{"table": table, "column": column, "id": int64(7)},
		NewPostgresDialect(),
	)
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	if compiled.SQL != `select "name" from "tenant_01"."sys_user" where id = $1` {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{int64(7)}) {
		t.Fatalf("unexpected args %#v", compiled.Args)
	}
}

func TestCompileSQL_whenRawOrderByProvided_shouldRenderSafeOrderList(t *testing.T) {
	t.Parallel()

	name, err := NewRawOrderItem("name", false)
	if err != nil {
		t.Fatalf("new order item failed: %v", err)
	}
	id, err := NewRawOrderItem("id", true)
	if err != nil {
		t.Fatalf("new order item failed: %v", err)
	}
	compiled, err := CompileSQL(
		"select id, name from sys_user order by ${orderBy}",
		NamedArgs{"orderBy": NewRawOrderBy(name, id)},
		NewMySQLDialect(),
	)
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	if compiled.SQL != "select id, name from sys_user order by `name` ASC, `id` DESC" {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
}

func TestCompileSQL_whenRawPlaceholderUsesPlainString_shouldReject(t *testing.T) {
	t.Parallel()

	_, err := CompileSQL("select * from ${table}", NamedArgs{"table": "sys_user"}, NewQuestionDialect())
	if err == nil || !strings.Contains(err.Error(), "RawSQLToken") {
		t.Fatalf("expected raw token error, got %v", err)
	}
}

func TestNewRawIdentifier_whenUnsafeIdentifierProvided_shouldReject(t *testing.T) {
	t.Parallel()

	if _, err := NewRawIdentifier("sys_user; drop table sys_user"); err == nil {
		t.Fatalf("expected unsafe identifier error")
	}
	if _, err := NewRawIdentifier("sys_user desc"); err == nil {
		t.Fatalf("expected identifier with direction to be rejected")
	}
}
