package runtime

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSelectFieldValues_whenTypedFieldProvided_shouldScanScalarList(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"name"},
		values:  [][]driver.Value{{"Alice"}, {"Bob"}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	names, err := SelectFieldValues(
		context.Background(),
		mapper,
		NewTypedField[baseMapperUser, string]("name"),
		NewQueryWrapper[baseMapperUser]().
			Eq(baseMapperUserStatus, "ACTIVE").
			OrderByAsc(baseMapperUserName),
	)
	if err != nil {
		t.Fatalf("select field values failed: %v", err)
	}

	if !reflect.DeepEqual(names, []string{"Alice", "Bob"}) {
		t.Fatalf("unexpected field values %#v", names)
	}
	if state.query != `SELECT "name" FROM "sys_user" WHERE "status" = $1 ORDER BY "name" ASC` {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestSelectFieldValue_whenSingleRowReturned_shouldScanScalar(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"status"},
		values:  [][]driver.Value{{"ACTIVE"}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	status, err := SelectFieldValue(
		context.Background(),
		mapper,
		NewTypedField[baseMapperUser, string]("status"),
		NewQueryWrapper[baseMapperUser]().Eq(baseMapperUserID, int64(7)),
	)
	if err != nil {
		t.Fatalf("select field value failed: %v", err)
	}

	if status != "ACTIVE" {
		t.Fatalf("unexpected status %q", status)
	}
	if state.query != `SELECT "status" FROM "sys_user" WHERE "id" = $1` {
		t.Fatalf("unexpected query %q", state.query)
	}
}

func TestSelectFieldValue_whenMultipleRowsReturned_shouldExposeTooManyResults(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}, {int64(8)}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = SelectFieldValue(context.Background(), mapper, NewTypedField[baseMapperUser, int64]("id"), nil)

	var tooMany *TooManyResultsError
	if !errors.As(err, &tooMany) {
		t.Fatalf("expected TooManyResultsError, got %v", err)
	}
	if !errors.Is(err, ErrTooManyResults) {
		t.Fatalf("expected ErrTooManyResults category, got %v", err)
	}
}

func TestSelectFirstFieldValue_whenWrapperNil_shouldApplyDefaultOrderAndLimit(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperMetricUser, int64](session, baseMapperMetricUserEntity(""))
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	id, err := SelectFirstFieldValue(context.Background(), mapper, NewTypedField[baseMapperMetricUser, int64]("id"), nil)
	if err != nil {
		t.Fatalf("select first field value failed: %v", err)
	}

	if id != 7 {
		t.Fatalf("unexpected id %d", id)
	}
	expectedQuery := `SELECT "id" FROM "sys_metric_user" ORDER BY "created_at" ASC, "id" DESC LIMIT $1 OFFSET $2`
	if state.query != expectedQuery {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: int64(1)},
		{Ordinal: 2, Value: int64(0)},
	}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestSelectFirstFieldValue_whenNoRowsReturned_shouldExposeNoRows(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = SelectFirstFieldValue(context.Background(), mapper, NewTypedField[baseMapperUser, int64]("id"), nil)

	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestSelectIDs_whenSoftDeleteFieldPresent_shouldSelectPrimaryKeyAndFilterLiveRows(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}, {int64(8)}},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperAuditUser, int64](session, baseMapperAuditUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	ids, err := mapper.SelectIDs(context.Background(), NewQueryWrapper[baseMapperAuditUser]().Eq(NewField[baseMapperAuditUser]("name"), "Alice"))
	if err != nil {
		t.Fatalf("select ids failed: %v", err)
	}

	if !reflect.DeepEqual(ids, []int64{7, 8}) {
		t.Fatalf("unexpected ids %#v", ids)
	}
	if state.query != `SELECT "id" FROM "sys_user" WHERE "name" = $1 AND "deleted" = $2` {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: "Alice"},
		{Ordinal: 2, Value: false},
	}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestServiceAndQueryChain_whenIDsRequested_shouldReturnTypedIDs(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{
			columns: []string{"id"},
			values:  [][]driver.Value{{int64(7)}, {int64(8)}},
		},
		{
			columns: []string{"id"},
			values:  [][]driver.Value{{int64(9)}},
		},
	}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	service, err := NewService[baseMapperUser, int64](mapper)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	listIDs, err := service.ListIDs(context.Background(), NewQueryWrapper[baseMapperUser]().OrderByAsc(baseMapperUserID))
	if err != nil {
		t.Fatalf("list ids failed: %v", err)
	}
	chainIDs, err := service.ChainQuery().Eq(baseMapperUserStatus, "ACTIVE").IDs(context.Background())
	if err != nil {
		t.Fatalf("chain ids failed: %v", err)
	}

	if !reflect.DeepEqual(listIDs, []int64{7, 8}) || !reflect.DeepEqual(chainIDs, []int64{9}) {
		t.Fatalf("unexpected ids list=%#v chain=%#v", listIDs, chainIDs)
	}
	expectedQueries := []string{
		`SELECT "id" FROM "sys_user" ORDER BY "id" ASC`,
		`SELECT "id" FROM "sys_user" WHERE "status" = $1`,
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
}

func TestSelectFieldValues_whenFieldIsNotMapped_shouldRejectBeforeQuery(t *testing.T) {
	state := openTestSQLState(t)
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}

	_, err = SelectFieldValues(context.Background(), mapper, NewTypedField[baseMapperUser, string]("missing"), nil)

	if err == nil || !strings.Contains(err.Error(), "not mapped") {
		t.Fatalf("expected unmapped field error, got %v", err)
	}
	if state.query != "" {
		t.Fatalf("query should not execute, got %q", state.query)
	}
}
