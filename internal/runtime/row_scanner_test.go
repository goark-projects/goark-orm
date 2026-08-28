package runtime

import (
	"context"
	"database/sql/driver"
	"fmt"
	"testing"
)

func TestSQLSession_QueryOne_whenTypeHandlerRowScannerRegistered_shouldUseSessionTypeHandler(t *testing.T) {
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:         "FindProfile",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.FindProfile",
		Command:    StatementCommandSelect,
		SQL:        "select id, profile from sys_user where id = #{id}",
		ResultType: "sqlSessionUser",
	})
	if err := registry.RegisterRowScanner("sqlSessionUser", TypeHandlerRowScannerFunc(func(ctx context.Context, columns []string, row RowScannerRow, dest any, handlers RowScannerTypeHandlers) error {
		user, ok := dest.(*sqlSessionUser)
		if !ok || user == nil {
			return fmt.Errorf("unexpected row scanner destination %T", dest)
		}
		var profileValue any
		if err := row.Scan(&user.ID, &profileValue); err != nil {
			return err
		}
		handler, ok := handlers.TypeHandler("profile")
		if !ok {
			return fmt.Errorf("profile handler missing")
		}
		return handler.FromDB(ctx, profileValue, &user.Profile)
	})); err != nil {
		t.Fatalf("register row scanner failed: %v", err)
	}
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "profile"},
		values:  [][]driver.Value{{int64(7), []byte("profile-fast-path")}},
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithLocalCache(false), WithTypeHandler("profile", profileTypeHandler{}))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindProfile", NamedArgs{"id": int64(7)}, &user); err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if user.ID != 7 || user.Profile.Text != "profile-fast-path" {
		t.Fatalf("unexpected scanned user %#v", user)
	}
}
