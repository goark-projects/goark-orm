package runtime

import (
	"reflect"
	"strings"
	"testing"
)

func TestCompileSQL_whenQuestionDialect_shouldBindArgsInOccurrenceOrder(t *testing.T) {
	compiled, err := CompileSQL(
		"select * from sys_user where id = #{id} and status = #{status} or owner_id = #{id}",
		NamedArgs{"id": int64(7), "status": "ACTIVE"},
		NewQuestionDialect(),
	)
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	if compiled.SQL != "select * from sys_user where id = ? and status = ? or owner_id = ?" {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
	expectedArgs := []any{int64(7), "ACTIVE", int64(7)}
	if !reflect.DeepEqual(compiled.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", compiled.Args)
	}
}

func TestCompileSQL_whenPostgresDialect_shouldUseNumberedPlaceholders(t *testing.T) {
	compiled, err := CompileSQL(
		"update sys_user set name = #{name} where id = #{id}",
		NamedArgs{"id": int64(7), "name": "Alice"},
		NewPostgresDialect(),
	)
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	if compiled.SQL != "update sys_user set name = $1 where id = $2" {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
}

func TestCompileSQL_whenNestedParameterPathsProvided_shouldResolveStructMapAndSlice(t *testing.T) {
	user := sqlSessionUser{ID: 7, Name: "Alice"}
	compiled, err := CompileSQL(
		"select * from sys_user where name = #{user.name} and status = #{filter.status} and id = #{ids[1]}",
		NamedArgs{
			"user":   user,
			"filter": map[string]any{"status": "ACTIVE"},
			"ids":    []int64{1, 7},
		},
		NewQuestionDialect(),
	)
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	if compiled.SQL != "select * from sys_user where name = ? and status = ? and id = ?" {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
	expectedArgs := []any{"Alice", "ACTIVE", int64(7)}
	if !reflect.DeepEqual(compiled.Args, expectedArgs) {
		t.Fatalf("unexpected args %#v", compiled.Args)
	}
}

func TestCompileSQL_whenParameterMissing_shouldReturnError(t *testing.T) {
	_, err := CompileSQL("select * from sys_user where id = #{id}", nil, NewQuestionDialect())
	if err == nil || !strings.Contains(err.Error(), `parameter "id" is missing`) {
		t.Fatalf("expected missing parameter error, got %v", err)
	}
}

func TestCompileSQL_whenRawSubstitutionUsed_shouldReject(t *testing.T) {
	_, err := CompileSQL("select * from ${table}", nil, NewQuestionDialect())
	if err == nil || !strings.Contains(err.Error(), "raw SQL parameter") {
		t.Fatalf("expected raw substitution error, got %v", err)
	}
}
