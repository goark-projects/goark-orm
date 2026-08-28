package runtime

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type sqlSessionUser struct {
	ID      int64
	Name    string
	Profile sqlSessionProfile
}

type sqlSessionOrder struct {
	ID       int64
	UserID   int64
	UserKind string
	Name     string
	User     sqlSessionUser
	Items    []sqlSessionOrderItem
}

type sqlSessionLazyOrder struct {
	ID     int64
	UserID int64
	Name   string
	User   Lazy[sqlSessionUser]
	Items  LazySlice[sqlSessionOrderItem]
}

type sqlSessionOrderItem struct {
	ID  int64
	SKU string
}

type sqlSessionInvoice struct {
	ID      int64
	Name    string
	Ignored string
	User    *sqlSessionUser
	Items   []sqlSessionOrderItem
}

type sqlSessionProfile struct {
	Text string
}

type sqlSessionAccount struct {
	ID    int64
	Kind  string
	Name  string
	Level int64
	Phone string
}

type sqlSessionConfigUser struct {
	UserName string
}

func TestSQLSession_QueryOne_whenStructPointerDestination_shouldScanEntityColumns(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user *sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &user)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if user == nil || user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
	if state.query != "select id, name from sys_user where id = $1" {
		t.Fatalf("unexpected query %q", state.query)
	}
	if !reflect.DeepEqual(state.queryArgs, []driver.NamedValue{{Ordinal: 1, Value: int64(7)}}) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestSQLSession_QueryOne_whenRowScannerRegistered_shouldUseFastPath(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	calls := 0
	if err := registry.RegisterRowScanner("sqlSessionUser", RowScannerFunc(func(ctx context.Context, columns []string, row RowScannerRow, dest any) error {
		_ = ctx
		calls++
		user, ok := dest.(*sqlSessionUser)
		if !ok || user == nil {
			t.Fatalf("unexpected scanner destination %#v", dest)
		}
		if !reflect.DeepEqual(columns, []string{"id", "name"}) {
			t.Fatalf("unexpected columns %#v", columns)
		}
		if err := row.Scan(&user.ID, &user.Name); err != nil {
			return err
		}
		user.Name = "fast:" + user.Name
		return nil
	})); err != nil {
		t.Fatalf("register row scanner failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &user)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected row scanner once, got %d", calls)
	}
	if user.ID != 7 || user.Name != "fast:Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
}

func TestSQLSession_whenDefaultStatementTimeoutConfigured_shouldApplyDeadline(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}},
	}
	registry := newSQLSessionRegistry(t,
		StatementMeta{
			ID:        "FindID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id from sys_user where id = #{id}",
		},
		StatementMeta{
			ID:        "UpdateName",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateName",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = #{id}",
		},
	)
	config := DefaultConfiguration()
	config.DefaultStatementTimeout = time.Second
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var id int64
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindID", NamedArgs{"id": int64(7)}, &id); err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	if _, err := session.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"}); err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if !state.queryHadDeadline || !state.execHadDeadline {
		t.Fatalf("expected query and exec deadlines, query=%v exec=%v", state.queryHadDeadline, state.execHadDeadline)
	}
}

func TestSQLSession_whenDefaultFetchSizeConfigured_shouldCallOptionalExecutorHook(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id from sys_user where id = #{id}",
	})
	executor := &recordingFetchSizeExecutor{db: state.db}
	config := DefaultConfiguration()
	config.DefaultFetchSize = 128
	session, err := NewSQLSession(registry, executor, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var id int64
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindID", NamedArgs{"id": int64(7)}, &id); err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if len(executor.fetchSizes) != 1 || executor.fetchSizes[0] != 128 {
		t.Fatalf("unexpected fetch sizes %#v", executor.fetchSizes)
	}
	if len(executor.fetchQueries) != 1 || executor.fetchQueries[0] != "select id from sys_user where id = $1" {
		t.Fatalf("unexpected fetch queries %#v", executor.fetchQueries)
	}
}

func TestSQLSession_whenStatementTimeoutConfigured_shouldOverrideDefaultTimeout(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id from sys_user where id = #{id}",
		Options: StatementOptions{
			Timeout: 2 * time.Second,
		},
	})
	config := DefaultConfiguration()
	config.DefaultStatementTimeout = time.Hour
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	started := time.Now()
	var id int64
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindID", NamedArgs{"id": int64(7)}, &id); err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if !state.queryHadDeadline {
		t.Fatalf("expected statement deadline")
	}
	if state.queryDeadline.Sub(started) > time.Minute {
		t.Fatalf("expected statement timeout to override default, got deadline delta %s", state.queryDeadline.Sub(started))
	}
}

func TestSQLSession_whenStatementFetchSizeConfigured_shouldOverrideDefaultFetchSize(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(7)}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id from sys_user where id = #{id}",
		Options: StatementOptions{
			FetchSize:     256,
			ResultSetType: ResultSetTypeForwardOnly,
			ResultOrdered: true,
		},
	})
	executor := &recordingStatementOptionsExecutor{db: state.db}
	config := DefaultConfiguration()
	config.DefaultFetchSize = 128
	session, err := NewSQLSession(registry, executor, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var id int64
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindID", NamedArgs{"id": int64(7)}, &id); err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if len(executor.options) != 1 {
		t.Fatalf("expected one statement options call, got %d", len(executor.options))
	}
	if executor.options[0].FetchSize != 256 || executor.options[0].ResultSetType != ResultSetTypeForwardOnly || !executor.options[0].ResultOrdered {
		t.Fatalf("unexpected statement options %#v", executor.options[0])
	}
}

func TestSQLSession_Query_whenSliceDestination_shouldScanRows(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values: [][]driver.Value{
			{int64(7), "Alice"},
			{int64(8), "Bob"},
		},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user",
	})
	session, err := NewSQLSession(registry, state.db, NewQuestionDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", nil, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(users) != 2 || users[0].ID != 7 || users[1].Name != "Bob" {
		t.Fatalf("unexpected users %#v", users)
	}
}

func TestSQLSession_Query_whenRowScannerRegistered_shouldScanSliceWithFastPath(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values: [][]driver.Value{
			{int64(7), "Alice"},
			{int64(8), "Bob"},
		},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user",
	})
	calls := 0
	if err := registry.RegisterRowScanner("sqlSessionUser", RowScannerFunc(func(ctx context.Context, columns []string, row RowScannerRow, dest any) error {
		_ = ctx
		_ = columns
		calls++
		user, ok := dest.(*sqlSessionUser)
		if !ok || user == nil {
			t.Fatalf("unexpected scanner destination %#v", dest)
		}
		if err := row.Scan(&user.ID, &user.Name); err != nil {
			return err
		}
		user.Name = strings.ToUpper(user.Name)
		return nil
	})); err != nil {
		t.Fatalf("register row scanner failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, NewQuestionDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.List", nil, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected row scanner twice, got %d", calls)
	}
	if len(users) != 2 || users[0].Name != "ALICE" || users[1].Name != "BOB" {
		t.Fatalf("unexpected users %#v", users)
	}
}

func TestQueryCursor_whenRowsExist_shouldScanOneByOne(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values: [][]driver.Value{
			{int64(7), "Alice"},
			{int64(8), "Bob"},
		},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user",
	})
	session, err := NewSQLSession(registry, state.db, NewQuestionDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	cursor, err := QueryCursor[sqlSessionUser](context.Background(), session, "system.user.UserMapper.List", nil)
	if err != nil {
		t.Fatalf("query cursor failed: %v", err)
	}
	first, ok, err := cursor.Next(context.Background())
	if err != nil || !ok {
		t.Fatalf("scan first cursor row failed, ok=%v err=%v", ok, err)
	}
	second, ok, err := cursor.Next(context.Background())
	if err != nil || !ok {
		t.Fatalf("scan second cursor row failed, ok=%v err=%v", ok, err)
	}
	_, ok, err = cursor.Next(context.Background())
	if err != nil || ok {
		t.Fatalf("expected cursor exhausted, ok=%v err=%v", ok, err)
	}
	if err := cursor.Close(); err != nil {
		t.Fatalf("close cursor failed: %v", err)
	}

	if first.ID != 7 || first.Name != "Alice" || second.ID != 8 || second.Name != "Bob" {
		t.Fatalf("unexpected cursor rows %#v %#v", first, second)
	}
	if state.rowsClosed != 1 {
		t.Fatalf("expected cursor to close rows once, got %d", state.rowsClosed)
	}
}

func TestQueryEach_whenHandlerReturnsError_shouldStopAndCloseRows(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values: [][]driver.Value{
			{int64(7), "Alice"},
			{int64(8), "Bob"},
		},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "List",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.List",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user",
	})
	session, err := NewSQLSession(registry, state.db, NewQuestionDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	stop := errors.New("stop")
	seen := make([]sqlSessionUser, 0)

	err = QueryEach[sqlSessionUser](context.Background(), session, "system.user.UserMapper.List", nil, func(ctx context.Context, user sqlSessionUser) error {
		seen = append(seen, user)
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("expected handler error, got %v", err)
	}
	if len(seen) != 1 || seen[0].ID != 7 {
		t.Fatalf("unexpected handled rows %#v", seen)
	}
	if state.rowsClosed != 1 {
		t.Fatalf("expected handler error to close rows once, got %d", state.rowsClosed)
	}
}

func TestSQLSession_QueryOne_whenNoRows_shouldReturnSQLNoRows(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{columns: []string{"id"}, values: nil}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id from sys_user where id = #{id}",
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &user)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestSQLSession_QueryOne_whenResultMapUsesTypeHandler_shouldConvertField(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "profile"},
		values:  [][]driver.Value{{int64(7), "admin"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindProfile",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindProfile",
		Command:   StatementCommandSelect,
		Source:    StatementSourceXML,
		SQL:       "select id, profile from sys_user where id = #{id}",
		ResultMap: "UserResult",
	})
	session, err := NewSQLSession(registry, state.db, nil, WithTypeHandler("profile", profileTypeHandler{}))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	if err := registry.RegisterRowScanner("sqlSessionUser", RowScannerFunc(func(context.Context, []string, RowScannerRow, any) error {
		t.Fatalf("type-handler resultMap must not use generated row scanner")
		return nil
	})); err != nil {
		t.Fatalf("register row scanner failed: %v", err)
	}

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindProfile", NamedArgs{"id": int64(7)}, &user)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	if user.Profile.Text != "admin" {
		t.Fatalf("unexpected profile %#v", user.Profile)
	}
}

func TestSQLSession_QueryOne_whenSimpleResultMapHasRowScanner_shouldUseFastPath(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"user_id", "user_name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	statement := StatementMeta{
		ID:        "FindAlias",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindAlias",
		Command:   StatementCommandSelect,
		Source:    StatementSourceXML,
		SQL:       "select id as user_id, name as user_name from sys_user where id = #{id}",
		ResultMap: "UserAliasResult",
	}
	registry := NewRegistry()
	if err := registry.RegisterEntity(EntityMeta{
		TypeName: "sqlSessionUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", ColumnName: "id", PrimaryKey: true},
			{FieldName: "Name", ColumnName: "name"},
		},
	}); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "UserAliasResult",
				TypeName: "sqlSessionUser",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "user_id", ID: true},
					{Property: "Name", Column: "user_name"},
				},
			},
		},
		Statements: []StatementMeta{statement},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	scanner := &recordingResultMapRowScanner{}
	if err := registry.RegisterRowScanner("sqlSessionUser", scanner); err != nil {
		t.Fatalf("register row scanner failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindAlias", NamedArgs{"id": int64(7)}, &user); err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	if scanner.calls != 1 {
		t.Fatalf("expected one row scanner call, got %d", scanner.calls)
	}
	if strings.Join(scanner.columns, ",") != "ID,Name" {
		t.Fatalf("unexpected scanner columns %#v", scanner.columns)
	}
	if user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected user %#v", user)
	}
}

type recordingResultMapRowScanner struct {
	calls   int
	columns []string
}

func (s *recordingResultMapRowScanner) ScanRow(_ context.Context, columns []string, row RowScannerRow, dest any) error {
	s.calls++
	s.columns = append([]string(nil), columns...)
	user, ok := dest.(*sqlSessionUser)
	if !ok || user == nil {
		return fmt.Errorf("destination must be *sqlSessionUser")
	}
	targets := make([]any, len(columns))
	for index, column := range columns {
		switch column {
		case "ID":
			targets[index] = &user.ID
		case "Name":
			targets[index] = &user.Name
		default:
			var discard any
			targets[index] = &discard
		}
	}
	return row.Scan(targets...)
}

func TestSQLSession_QueryOne_whenResultMapUsesAssociation_shouldScanNestedStruct(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"order_id", "order_name", "user_id", "user_name"},
		values:  [][]driver.Value{{int64(100), "Order-100", int64(7), "Alice"}},
	}
	registry := newOrderSessionRegistry(t)
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var order sqlSessionOrder
	err = session.QueryOne(context.Background(), "system.order.OrderMapper.FindByID", NamedArgs{"id": int64(100)}, &order)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if order.ID != 100 || order.Name != "Order-100" || order.User.ID != 7 || order.User.Name != "Alice" {
		t.Fatalf("unexpected order %#v", order)
	}
}

func TestSQLSession_QueryOne_whenAssociationResultMapHasRowScanners_shouldUseComposedFastPath(t *testing.T) {
	disabled := false
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"order_id", "order_name", "user_id", "user_name"},
		values:  [][]driver.Value{{int64(100), "Order-100", int64(7), "Alice"}},
	}
	statement := StatementMeta{
		ID:        "FindByID",
		Namespace: "system.order.OrderMapper",
		FullName:  "system.order.OrderMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceXML,
		SQL:       "select * from orders where id = #{id}",
		ResultMap: "OrderResult",
	}
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: "system.order.OrderMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:          "OrderResult",
				TypeName:    "sqlSessionOrder",
				AutoMapping: &disabled,
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "order_id", ID: true},
					{Property: "Name", Column: "order_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property: "User",
						TypeName: "sqlSessionUser",
						Fields: []ResultFieldMeta{
							{Property: "ID", Column: "user_id", ID: true},
							{Property: "Name", Column: "user_name"},
						},
					},
				},
			},
		},
		Statements: []StatementMeta{statement},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	orderScanner := &recordingOrderRowScanner{}
	userScanner := &recordingResultMapRowScanner{}
	if err := registry.RegisterRowScanner("sqlSessionOrder", orderScanner); err != nil {
		t.Fatalf("register order row scanner failed: %v", err)
	}
	if err := registry.RegisterRowScanner("sqlSessionUser", userScanner); err != nil {
		t.Fatalf("register user row scanner failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var order sqlSessionOrder
	if err := session.QueryOne(context.Background(), "system.order.OrderMapper.FindByID", NamedArgs{"id": int64(100)}, &order); err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if order.ID != 100 || order.Name != "Order-100" || order.User.ID != 7 || order.User.Name != "Alice" {
		t.Fatalf("unexpected order %#v", order)
	}
	if orderScanner.calls != 1 || userScanner.calls != 1 {
		t.Fatalf("expected composed row scanners once, got order=%d user=%d", orderScanner.calls, userScanner.calls)
	}
	if strings.Join(orderScanner.columns, ",") != "ID,Name,__goark_orm_discard_2,__goark_orm_discard_3" {
		t.Fatalf("unexpected order scanner columns %#v", orderScanner.columns)
	}
	if strings.Join(userScanner.columns, ",") != "__goark_orm_discard_0,__goark_orm_discard_1,ID,Name" {
		t.Fatalf("unexpected user scanner columns %#v", userScanner.columns)
	}
}

func TestSQLSession_Query_whenResultMapDiscriminatorUsesInlineCase_shouldScanCaseFields(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "kind", "name", "admin_level"},
		values: [][]driver.Value{
			{int64(7), "admin", "Alice", int64(9)},
			{int64(8), "normal", "Bob", int64(0)},
		},
	}
	registry := newDiscriminatorAccountRegistry(t)
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var accounts []sqlSessionAccount
	err = session.Query(context.Background(), "system.account.AccountMapper.List", nil, &accounts)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("expected two accounts, got %#v", accounts)
	}
	if accounts[0].ID != 7 || accounts[0].Kind != "admin" || accounts[0].Level != 9 {
		t.Fatalf("unexpected admin account %#v", accounts[0])
	}
	if accounts[1].ID != 8 || accounts[1].Kind != "normal" || accounts[1].Level != 0 {
		t.Fatalf("unexpected normal account %#v", accounts[1])
	}
}

func TestSQLSession_Query_whenDiscriminatorEffectiveResultMapHasRowScanner_shouldUseFastPath(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "kind", "name", "admin_level"},
		values:  [][]driver.Value{{int64(7), "admin", "Alice", int64(9)}},
	}
	registry := newDiscriminatorAccountRegistry(t)
	scanner := &recordingAccountRowScanner{}
	if err := registry.RegisterRowScanner("sqlSessionAccount", scanner); err != nil {
		t.Fatalf("register row scanner failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var accounts []sqlSessionAccount
	err = session.Query(context.Background(), "system.account.AccountMapper.List", nil, &accounts)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(accounts) != 1 || accounts[0].ID != 7 || accounts[0].Level != 9 {
		t.Fatalf("unexpected accounts %#v", accounts)
	}
	if scanner.calls != 1 {
		t.Fatalf("expected discriminator effective resultMap to use row scanner, got %d calls", scanner.calls)
	}
	if strings.Join(scanner.columns, ",") != "ID,Kind,Name,Level" {
		t.Fatalf("unexpected scanner columns %#v", scanner.columns)
	}
}

type recordingAccountRowScanner struct {
	calls   int
	columns []string
}

func (s *recordingAccountRowScanner) ScanRow(_ context.Context, columns []string, row RowScannerRow, dest any) error {
	s.calls++
	s.columns = append([]string(nil), columns...)
	account, ok := dest.(*sqlSessionAccount)
	if !ok || account == nil {
		return fmt.Errorf("destination must be *sqlSessionAccount")
	}
	targets := make([]any, len(columns))
	for index, column := range columns {
		switch column {
		case "ID":
			targets[index] = &account.ID
		case "Kind":
			targets[index] = &account.Kind
		case "Name":
			targets[index] = &account.Name
		case "Level":
			targets[index] = &account.Level
		case "Phone":
			targets[index] = &account.Phone
		default:
			var discard any
			targets[index] = &discard
		}
	}
	return row.Scan(targets...)
}

func TestSQLSession_Query_whenResultMapDiscriminatorUsesReferencedResultMap_shouldScanReferencedFields(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "kind", "name", "phone_number"},
		values: [][]driver.Value{
			{int64(7), "vip", "Alice", "13800000000"},
			{int64(8), "normal", "Bob", "13900000000"},
		},
	}
	registry := newDiscriminatorAccountRegistry(t)
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var accounts []sqlSessionAccount
	err = session.Query(context.Background(), "system.account.AccountMapper.List", nil, &accounts)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("expected two accounts, got %#v", accounts)
	}
	if accounts[0].ID != 7 || accounts[0].Kind != "vip" || accounts[0].Phone != "13800000000" {
		t.Fatalf("unexpected vip account %#v", accounts[0])
	}
	if accounts[1].ID != 8 || accounts[1].Kind != "normal" || accounts[1].Phone != "" {
		t.Fatalf("unexpected normal account %#v", accounts[1])
	}
}

func TestSQLSession_Query_whenResultMapDiscriminatorUsesDifferentResultType_shouldReturnError(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "kind", "name"},
		values:  [][]driver.Value{{int64(7), "admin", "Alice"}},
	}
	registry := newMismatchedDiscriminatorAccountRegistry(t)
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var accounts []sqlSessionAccount
	err = session.Query(context.Background(), "system.account.AccountMapper.List", nil, &accounts)
	if err == nil || !strings.Contains(err.Error(), `discriminator resultType "sqlSessionAdminAccount"`) {
		t.Fatalf("expected discriminator resultType error, got %v", err)
	}
}

func TestSQLSession_Query_whenResultMapUsesCollection_shouldAggregateRowsByRootID(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"order_id", "order_name", "user_id", "user_name", "item_id", "item_sku"},
		values: [][]driver.Value{
			{int64(100), "Order-100", int64(7), "Alice", int64(501), "SKU-1"},
			{int64(100), "Order-100", int64(7), "Alice", int64(502), "SKU-2"},
		},
	}
	registry := newOrderSessionRegistry(t)
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var orders []sqlSessionOrder
	err = session.Query(context.Background(), "system.order.OrderMapper.FindByID", NamedArgs{"id": int64(100)}, &orders)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(orders) != 1 {
		t.Fatalf("expected one root order, got %#v", orders)
	}
	if orders[0].User.ID != 7 || orders[0].User.Name != "Alice" {
		t.Fatalf("unexpected association %#v", orders[0].User)
	}
	if len(orders[0].Items) != 2 || orders[0].Items[0].SKU != "SKU-1" || orders[0].Items[1].ID != 502 {
		t.Fatalf("unexpected collection %#v", orders[0].Items)
	}
}

func TestSQLSession_Query_whenResultMapUsesCollection_shouldNotUseComposedFastPath(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"order_id", "order_name", "user_id", "user_name", "item_id", "item_sku"},
		values: [][]driver.Value{
			{int64(100), "Order-100", int64(7), "Alice", int64(501), "SKU-1"},
			{int64(100), "Order-100", int64(7), "Alice", int64(502), "SKU-2"},
		},
	}
	registry := newOrderSessionRegistry(t)
	if err := registry.RegisterRowScanner("sqlSessionOrder", RowScannerFunc(func(context.Context, []string, RowScannerRow, any) error {
		t.Fatalf("collection resultMap must not use composed row scanner")
		return nil
	})); err != nil {
		t.Fatalf("register order row scanner failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var orders []sqlSessionOrder
	if err := session.Query(context.Background(), "system.order.OrderMapper.FindByID", NamedArgs{"id": int64(100)}, &orders); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(orders) != 1 || len(orders[0].Items) != 2 {
		t.Fatalf("unexpected orders %#v", orders)
	}
}

func TestSQLSession_Query_whenResultMapUsesNamedResultSets_shouldAttachChildren(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"order_id", "user_id", "order_name"},
		values: [][]driver.Value{
			{int64(100), int64(7), "Order-100"},
			{int64(101), int64(8), "Order-101"},
		},
		resultSets: []testRowsData{
			{
				columns: []string{"user_id", "user_name"},
				values: [][]driver.Value{
					{int64(7), "Alice"},
					{int64(8), "Bob"},
				},
			},
			{
				columns: []string{"order_id", "item_id", "item_sku"},
				values: [][]driver.Value{
					{int64(100), int64(501), "SKU-1"},
					{int64(100), int64(502), "SKU-2"},
					{int64(101), int64(503), "SKU-3"},
				},
			},
		},
	}
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: "system.order.OrderMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "OrderResult",
				TypeName: "sqlSessionOrder",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "order_id", ID: true},
					{Property: "UserID", Column: "user_id"},
					{Property: "Name", Column: "order_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property:      "User",
						TypeName:      "sqlSessionUser",
						Column:        "user_id",
						ResultSet:     "users",
						ForeignColumn: "user_id",
						Fields: []ResultFieldMeta{
							{Property: "ID", Column: "user_id", ID: true},
							{Property: "Name", Column: "user_name"},
						},
					},
				},
				Collections: []ResultCollectionMeta{
					{
						Property:      "Items",
						TypeName:      "sqlSessionOrderItem",
						Column:        "order_id",
						ResultSet:     "items",
						ForeignColumn: "order_id",
						Fields: []ResultFieldMeta{
							{Property: "ID", Column: "item_id", ID: true},
							{Property: "SKU", Column: "item_sku"},
						},
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "LoadReport",
				Namespace: "system.order.OrderMapper",
				FullName:  "system.order.OrderMapper.LoadReport",
				Command:   StatementCommandSelect,
				Source:    StatementSourceXML,
				SQL:       "select order_id, user_id, order_name from orders",
				ResultMap: "OrderResult",
				ResultSets: []ResultSetMeta{
					{Name: "orders"},
					{Name: "users"},
					{Name: "items"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var orders []sqlSessionOrder
	if err := session.Query(context.Background(), "system.order.OrderMapper.LoadReport", nil, &orders); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(orders) != 2 {
		t.Fatalf("expected two orders, got %#v", orders)
	}
	if orders[0].User.Name != "Alice" || orders[1].User.Name != "Bob" {
		t.Fatalf("unexpected result-set associations %#v", orders)
	}
	if len(orders[0].Items) != 2 || orders[0].Items[1].SKU != "SKU-2" || len(orders[1].Items) != 1 || orders[1].Items[0].ID != 503 {
		t.Fatalf("unexpected result-set collections %#v", orders)
	}
}

type recordingOrderRowScanner struct {
	calls   int
	columns []string
}

func (s *recordingOrderRowScanner) ScanRow(_ context.Context, columns []string, row RowScannerRow, dest any) error {
	s.calls++
	s.columns = append([]string(nil), columns...)
	order, ok := dest.(*sqlSessionOrder)
	if !ok || order == nil {
		return fmt.Errorf("destination must be *sqlSessionOrder")
	}
	targets := make([]any, len(columns))
	for index, column := range columns {
		switch column {
		case "ID":
			targets[index] = &order.ID
		case "Name":
			targets[index] = &order.Name
		default:
			var discard any
			targets[index] = &discard
		}
	}
	return row.Scan(targets...)
}

func TestSQLSession_Query_whenResultMapUsesConstructorPrefixNotNullAndAutoMappingFalse_shouldScanExplicitGraph(t *testing.T) {
	disabled := false
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"invoice_id", "invoice_name", "ignored", "user_id", "user_name", "item_id", "item_sku"},
		values: [][]driver.Value{
			{int64(200), "Invoice-200", "auto-value", int64(7), "Alice", nil, nil},
			{int64(200), "Invoice-200", "auto-value", int64(7), "Alice", int64(501), "SKU-1"},
		},
	}
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "InvoiceMapper",
		Namespace: "system.invoice.InvoiceMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:          "InvoiceResult",
				TypeName:    "sqlSessionInvoice",
				AutoMapping: &disabled,
				Constructor: ResultConstructorMeta{
					Args: []ResultArgMeta{
						{Name: "ID", Column: "invoice_id", ID: true},
					},
				},
				Fields: []ResultFieldMeta{
					{Property: "Name", Column: "invoice_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property:       "User",
						TypeName:       "sqlSessionUser",
						ColumnPrefix:   "user_",
						NotNullColumns: []string{"id"},
						Fields: []ResultFieldMeta{
							{Property: "ID", Column: "id", ID: true},
							{Property: "Name", Column: "name"},
						},
					},
				},
				Collections: []ResultCollectionMeta{
					{
						Property:       "Items",
						TypeName:       "sqlSessionOrderItem",
						ColumnPrefix:   "item_",
						NotNullColumns: []string{"id"},
						Fields: []ResultFieldMeta{
							{Property: "ID", Column: "id", ID: true},
							{Property: "SKU", Column: "sku"},
						},
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "List",
				Namespace: "system.invoice.InvoiceMapper",
				FullName:  "system.invoice.InvoiceMapper.List",
				Command:   StatementCommandSelect,
				Source:    StatementSourceXML,
				SQL:       "select * from invoice",
				ResultMap: "InvoiceResult",
			},
		},
	})
	if err != nil {
		t.Fatalf("register invoice mapper failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var invoices []sqlSessionInvoice
	err = session.Query(context.Background(), "system.invoice.InvoiceMapper.List", nil, &invoices)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(invoices) != 1 {
		t.Fatalf("expected one invoice, got %#v", invoices)
	}
	invoice := invoices[0]
	if invoice.ID != 200 || invoice.Name != "Invoice-200" || invoice.Ignored != "" {
		t.Fatalf("unexpected root mapping %#v", invoice)
	}
	if invoice.User == nil || invoice.User.ID != 7 || invoice.User.Name != "Alice" {
		t.Fatalf("unexpected prefixed association %#v", invoice.User)
	}
	if len(invoice.Items) != 1 || invoice.Items[0].ID != 501 || invoice.Items[0].SKU != "SKU-1" {
		t.Fatalf("unexpected notNull collection %#v", invoice.Items)
	}
}

func TestSQLSession_QueryOne_whenResultMapUsesNestedSelects_shouldLoadAssociationAndCollection(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{
			columns: []string{"order_id", "user_id", "order_name"},
			values:  [][]driver.Value{{int64(100), int64(7), "Order-100"}},
		},
		{
			columns: []string{"id", "name"},
			values:  [][]driver.Value{{int64(7), "Alice"}},
		},
		{
			columns: []string{"id", "sku"},
			values: [][]driver.Value{
				{int64(501), "SKU-1"},
				{int64(502), "SKU-2"},
			},
		},
	}
	registry := newNestedSelectOrderRegistry(t)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var order sqlSessionOrder
	err = session.QueryOne(context.Background(), "system.order.OrderMapper.FindWithNested", NamedArgs{"id": int64(100)}, &order)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if order.ID != 100 || order.UserID != 7 || order.User.ID != 7 || order.User.Name != "Alice" {
		t.Fatalf("unexpected nested association %#v", order)
	}
	if len(order.Items) != 2 || order.Items[0].ID != 501 || order.Items[1].SKU != "SKU-2" {
		t.Fatalf("unexpected nested collection %#v", order.Items)
	}
	expectedQueries := []string{
		"select order_id, user_id, order_name from orders where order_id = $1",
		"select id, name from sys_user where id = $1",
		"select id, sku from order_item where order_id = $1",
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
	expectedArgs := [][]driver.NamedValue{
		{{Ordinal: 1, Value: int64(100)}},
		{{Ordinal: 1, Value: int64(7)}},
		{{Ordinal: 1, Value: int64(100)}},
	}
	if !reflect.DeepEqual(state.queryArgsList, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgsList)
	}
}

func TestSQLSession_QueryOne_whenNestedSelectUsesCompositeColumn_shouldBindNamedArguments(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{
			columns: []string{"order_id", "user_id", "user_kind", "order_name"},
			values:  [][]driver.Value{{int64(100), int64(7), "internal", "Order-100"}},
		},
		{
			columns: []string{"id", "name"},
			values:  [][]driver.Value{{int64(7), "Alice"}},
		},
	}
	registry := newCompositeNestedSelectOrderRegistry(t)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var order sqlSessionOrder
	err = session.QueryOne(context.Background(), "system.order.OrderMapper.FindWithCompositeNested", NamedArgs{"id": int64(100)}, &order)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}

	if order.ID != 100 || order.UserID != 7 || order.UserKind != "internal" || order.User.ID != 7 || order.User.Name != "Alice" {
		t.Fatalf("unexpected composite nested association %#v", order)
	}
	expectedQueries := []string{
		"select order_id, user_id, user_kind, order_name from orders where order_id = $1",
		"select id, name from sys_user where id = $1 and kind = $2",
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
	expectedArgs := [][]driver.NamedValue{
		{{Ordinal: 1, Value: int64(100)}},
		{{Ordinal: 1, Value: int64(7)}, {Ordinal: 2, Value: "internal"}},
	}
	if !reflect.DeepEqual(state.queryArgsList, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgsList)
	}
}

func TestSQLSession_QueryOne_whenNestedSelectFetchTypeLazyUsesExplicitLazyLoader(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{
			columns: []string{"order_id", "user_id", "order_name"},
			values:  [][]driver.Value{{int64(100), int64(7), "Order-100"}},
		},
		{
			columns: []string{"id", "name"},
			values:  [][]driver.Value{{int64(7), "Alice"}},
		},
		{
			columns: []string{"id", "sku"},
			values: [][]driver.Value{
				{int64(501), "SKU-1"},
				{int64(502), "SKU-2"},
			},
		},
	}
	registry := newLazyNestedSelectOrderRegistry(t)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var order sqlSessionLazyOrder
	err = session.QueryOne(context.Background(), "system.order.OrderMapper.FindWithLazy", NamedArgs{"id": int64(100)}, &order)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	if order.ID != 100 || order.UserID != 7 || order.Name != "Order-100" {
		t.Fatalf("unexpected root order id=%d userID=%d name=%q", order.ID, order.UserID, order.Name)
	}
	if order.User.Loaded() || order.Items.Loaded() {
		t.Fatalf("lazy fields must not be loaded by parent query")
	}
	if len(state.queries) != 1 {
		t.Fatalf("expected only parent query before lazy load, got %#v", state.queries)
	}

	user, err := order.User.Load(context.Background())
	if err != nil {
		t.Fatalf("load lazy user failed: %v", err)
	}
	if user.ID != 7 || user.Name != "Alice" {
		t.Fatalf("unexpected lazy user %#v", user)
	}
	again, err := order.User.Load(context.Background())
	if err != nil {
		t.Fatalf("load cached lazy user failed: %v", err)
	}
	if again.ID != 7 || len(state.queries) != 2 {
		t.Fatalf("expected lazy association cache hit, user=%#v queries=%#v", again, state.queries)
	}

	items, err := order.Items.Load(context.Background())
	if err != nil {
		t.Fatalf("load lazy items failed: %v", err)
	}
	if len(items) != 2 || items[0].ID != 501 || items[1].SKU != "SKU-2" {
		t.Fatalf("unexpected lazy items %#v", items)
	}
	expectedQueries := []string{
		"select order_id, user_id, order_name from orders where order_id = $1",
		"select id, name from sys_user where id = $1",
		"select id, sku from order_item where order_id = $1",
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
}

func TestSQLSession_Query_whenNestedSelectSeesSameArguments_shouldReuseQueryLocalResult(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{
			columns: []string{"order_id", "user_id", "order_name"},
			values: [][]driver.Value{
				{int64(100), int64(7), "Order-100"},
				{int64(101), int64(7), "Order-101"},
			},
		},
		{
			columns: []string{"id", "name"},
			values:  [][]driver.Value{{int64(7), "Alice"}},
		},
	}
	registry := newDedupNestedSelectOrderRegistry(t)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithLocalCache(false))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var orders []sqlSessionOrder
	err = session.Query(context.Background(), "system.order.OrderMapper.ListWithNested", nil, &orders)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(orders) != 2 || orders[0].User.Name != "Alice" || orders[1].User.Name != "Alice" {
		t.Fatalf("unexpected nested association reuse result %#v", orders)
	}
	expectedQueries := []string{
		"select order_id, user_id, order_name from orders",
		"select id, name from sys_user where id = $1",
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
}

func TestSQLSession_Exec_whenGeneratedKeys_shouldReturnLastInsertID(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1, lastInsertID: 42}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:               "Insert",
		Namespace:        "system.user.UserMapper",
		FullName:         "system.user.UserMapper.Insert",
		Command:          StatementCommandInsert,
		Source:           StatementSourceAnnotation,
		SQL:              "insert into sys_user(name) values(#{name})",
		UseGeneratedKeys: true,
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	result, err := session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{"name": "Alice"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if result.RowsAffected != 1 || result.LastInsertID != 42 {
		t.Fatalf("unexpected result %#v", result)
	}
	if state.exec != "insert into sys_user(name) values(?)" {
		t.Fatalf("unexpected exec SQL %q", state.exec)
	}
}

func TestSQLSession_Call_whenOutParameterConfigured_shouldBindSQLOut(t *testing.T) {
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "CountByStatus",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.CountByStatus",
		Command:       StatementCommandCall,
		StatementType: StatementTypeCallable,
		SQL:           "call count_users(#{status}, #{total})",
		ParameterModes: []ParameterMeta{
			{Name: "status", Mode: ParameterModeIn},
			{Name: "total", Mode: ParameterModeOut},
		},
	})
	state := openTestSQLState(t)
	state.outValues = []any{int64(42)}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	var total int64
	result, err := session.Call(context.Background(), "system.user.UserMapper.CountByStatus", NamedArgs{
		"status": "ACTIVE",
		"total":  &total,
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if result.RowsAffected != 0 {
		t.Fatalf("unexpected rows affected %d", result.RowsAffected)
	}
	if total != 42 {
		t.Fatalf("unexpected OUT value %d", total)
	}
	if state.exec != "call count_users($1, $2)" {
		t.Fatalf("unexpected SQL %q", state.exec)
	}
	if len(state.execArgs) != 2 || state.execArgs[0].Value != "ACTIVE" {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
	out, ok := state.execArgs[1].Value.(sql.Out)
	if !ok {
		t.Fatalf("second arg is %T, want sql.Out", state.execArgs[1].Value)
	}
	if out.In {
		t.Fatalf("OUT parameter must not carry input value")
	}
	if out.Dest != &total {
		t.Fatalf("unexpected OUT destination %#v", out.Dest)
	}
}

func TestSQLSession_Call_whenInOutParameterConfigured_shouldBindInputAndOutput(t *testing.T) {
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "NextRevision",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.NextRevision",
		Command:       StatementCommandCall,
		StatementType: StatementTypeCallable,
		SQL:           "call next_revision(#{revision})",
		ParameterModes: []ParameterMeta{
			{Name: "revision", Mode: ParameterModeInOut},
		},
	})
	state := openTestSQLState(t)
	state.outValues = []any{int64(8)}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	revision := int64(7)
	if _, err := session.Call(context.Background(), "system.user.UserMapper.NextRevision", NamedArgs{"revision": &revision}); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if revision != 8 {
		t.Fatalf("unexpected INOUT value %d", revision)
	}
	if len(state.execArgs) != 1 {
		t.Fatalf("unexpected args %#v", state.execArgs)
	}
	out, ok := state.execArgs[0].Value.(sql.Out)
	if !ok {
		t.Fatalf("first arg is %T, want sql.Out", state.execArgs[0].Value)
	}
	if !out.In {
		t.Fatalf("INOUT parameter must carry input value")
	}
}

func TestSQLSession_Call_whenMultipleResultSets_shouldScanAllDestinations(t *testing.T) {
	type callRole struct {
		ID   int64
		Code string
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "LoadUserReport",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.LoadUserReport",
		Command:       StatementCommandCall,
		StatementType: StatementTypeCallable,
		SQL:           "call load_user_report(#{status})",
		ParameterModes: []ParameterMeta{
			{Name: "status", Mode: ParameterModeIn},
		},
		ResultSets: []ResultSetMeta{
			{Name: "users", ResultType: "User"},
			{Name: "roles", ResultType: "callRole"},
		},
	})
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
		resultSets: []testRowsData{
			{
				columns: []string{"id", "code"},
				values:  [][]driver.Value{{int64(10), "admin"}, {int64(11), "audit"}},
			},
		},
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	var users []sqlSessionUser
	var roles []callRole
	if _, err := session.Call(context.Background(), "system.user.UserMapper.LoadUserReport", NamedArgs{"status": "ACTIVE"}, &users, &roles); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if len(users) != 1 || users[0].ID != 7 || users[0].Name != "Alice" {
		t.Fatalf("unexpected users %#v", users)
	}
	if len(roles) != 2 || roles[0].Code != "admin" || roles[1].ID != 11 {
		t.Fatalf("unexpected roles %#v", roles)
	}
	if state.query != "call load_user_report($1)" {
		t.Fatalf("unexpected SQL %q", state.query)
	}
	if state.rowsClosed != 1 {
		t.Fatalf("rows closed %d, want 1", state.rowsClosed)
	}
}

func TestSQLSession_Query_whenExecutorTypeReuse_shouldPrepareSQLOnce(t *testing.T) {
	state := openTestSQLState(t)
	state.db.SetMaxOpenConns(1)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(8), "Bob"}}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:         "ListByStatus",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.ListByStatus",
		Command:    StatementCommandSelect,
		Source:     StatementSourceAnnotation,
		SQL:        "select id, name from sys_user where status = #{status}",
		Parameters: []string{"status"},
	})
	session, err := NewSQLSession(
		registry,
		state.db,
		NewPostgresDialect(),
		WithConfiguration(Configuration{DefaultExecutorType: ExecutorTypeReuse}),
	)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var active []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.ListByStatus", NamedArgs{"status": "ACTIVE"}, &active); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	var locked []sqlSessionUser
	if err := session.Query(context.Background(), "system.user.UserMapper.ListByStatus", NamedArgs{"status": "LOCKED"}, &locked); err != nil {
		t.Fatalf("second query failed: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close session failed: %v", err)
	}

	if len(active) != 1 || active[0].Name != "Alice" || len(locked) != 1 || locked[0].Name != "Bob" {
		t.Fatalf("unexpected reuse query results active=%#v locked=%#v", active, locked)
	}
	if state.prepareCount != 1 {
		t.Fatalf("expected one prepared statement, got %d queries=%#v", state.prepareCount, state.prepareQueries)
	}
	if state.prepareQueries[0] != "select id, name from sys_user where status = $1" {
		t.Fatalf("unexpected prepared SQL %q", state.prepareQueries[0])
	}
	if len(state.queries) != 2 {
		t.Fatalf("expected two statement executions, got %#v", state.queries)
	}
	if state.stmtClosed != 1 {
		t.Fatalf("expected prepared statement close once, got %d", state.stmtClosed)
	}
}

func TestSQLSession_Exec_whenExecutorTypeReuse_shouldReusePreparedStatement(t *testing.T) {
	state := openTestSQLState(t)
	state.db.SetMaxOpenConns(1)
	state.execResults = []driver.Result{
		testResult{rowsAffected: 1},
		testResult{rowsAffected: 1},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:         "UpdateName",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.UpdateName",
		Command:    StatementCommandUpdate,
		Source:     StatementSourceAnnotation,
		SQL:        "update sys_user set name = #{name} where id = #{id}",
		Parameters: []string{"id", "name"},
	})
	session, err := NewSQLSession(
		registry,
		state.db,
		NewPostgresDialect(),
		WithConfiguration(Configuration{DefaultExecutorType: ExecutorTypeReuse}),
	)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	if _, err := session.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Alice"}); err != nil {
		t.Fatalf("first exec failed: %v", err)
	}
	if _, err := session.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(8), "name": "Bob"}); err != nil {
		t.Fatalf("second exec failed: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close session failed: %v", err)
	}

	if state.prepareCount != 1 {
		t.Fatalf("expected one prepared statement, got %d queries=%#v", state.prepareCount, state.prepareQueries)
	}
	if state.prepareQueries[0] != "update sys_user set name = $1 where id = $2" {
		t.Fatalf("unexpected prepared SQL %q", state.prepareQueries[0])
	}
	if len(state.execs) != 2 {
		t.Fatalf("expected two statement executions, got %#v", state.execs)
	}
	if state.stmtClosed != 1 {
		t.Fatalf("expected prepared statement close once, got %d", state.stmtClosed)
	}
}

func TestSQLSession_Exec_whenPreparedStatementCacheSizeExceeded_shouldEvictLeastRecentlyUsed(t *testing.T) {
	state := openTestSQLState(t)
	state.db.SetMaxOpenConns(1)
	registry := newSQLSessionRegistry(t,
		StatementMeta{
			ID:        "UpdateA",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateA",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = 1",
		},
		StatementMeta{
			ID:        "UpdateB",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateB",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = 2",
		},
		StatementMeta{
			ID:        "UpdateC",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateC",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = 3",
		},
	)
	session, err := NewSQLSession(
		registry,
		state.db,
		NewPostgresDialect(),
		WithConfiguration(Configuration{
			DefaultExecutorType:        ExecutorTypeReuse,
			PreparedStatementCacheSize: 2,
		}),
	)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	for _, statement := range []string{
		"system.user.UserMapper.UpdateA",
		"system.user.UserMapper.UpdateB",
		"system.user.UserMapper.UpdateA",
		"system.user.UserMapper.UpdateC",
		"system.user.UserMapper.UpdateB",
	} {
		if _, err := session.Exec(context.Background(), statement, NamedArgs{"name": "Alice"}); err != nil {
			t.Fatalf("exec %s failed: %v", statement, err)
		}
	}

	if state.prepareCount != 4 {
		t.Fatalf("expected four prepared statements after LRU eviction, got %d queries=%#v", state.prepareCount, state.prepareQueries)
	}
	if state.stmtClosed != 2 {
		t.Fatalf("expected two evicted statements to close, got %d", state.stmtClosed)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close session failed: %v", err)
	}
	if state.stmtClosed != 4 {
		t.Fatalf("expected all prepared statements to close, got %d", state.stmtClosed)
	}
}

func TestSQLSession_Exec_whenGeneratedKeysHasKeyProperty_shouldBackfillEntityField(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1, lastInsertID: 42}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:               "Insert",
		Namespace:        "system.user.UserMapper",
		FullName:         "system.user.UserMapper.Insert",
		Command:          StatementCommandInsert,
		Source:           StatementSourceAnnotation,
		SQL:              "insert into sys_user(name) values(#{Name})",
		ParameterType:    "sqlSessionUser",
		UseGeneratedKeys: true,
		KeyProperty:      "ID",
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	user := &sqlSessionUser{Name: "Alice"}

	result, err := session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{
		"user": user,
		"Name": user.Name,
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if result.LastInsertID != 42 || user.ID != 42 {
		t.Fatalf("expected generated key to be returned and backfilled, result=%#v user=%#v", result, user)
	}
}

func TestSQLSession_Exec_whenSelectKeyBefore_shouldBackfillKeyBeforeInsert(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"next_id"},
		values:  [][]driver.Value{{int64(99)}},
	}
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "Insert",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.Insert",
		Command:       StatementCommandInsert,
		Source:        StatementSourceXML,
		SQL:           "insert into sys_user(id, name) values(#{ID}, #{Name})",
		ParameterType: "sqlSessionUser",
		KeyProperty:   "ID",
		SelectKey: SelectKeyMeta{
			Enabled:     true,
			KeyProperty: "ID",
			ResultType:  "int64",
			Order:       SelectKeyOrderBefore,
			SQL:         "select nextval('sys_user_id_seq')",
		},
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	user := &sqlSessionUser{Name: "Alice"}

	result, err := session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{
		"user": user,
		"Name": user.Name,
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if user.ID != 99 || result.LastInsertID != 99 {
		t.Fatalf("expected selectKey BEFORE to backfill id, result=%#v user=%#v", result, user)
	}
	if !reflect.DeepEqual(state.queries, []string{"select nextval('sys_user_id_seq')"}) {
		t.Fatalf("unexpected selectKey queries %#v", state.queries)
	}
	if state.exec != "insert into sys_user(id, name) values($1, $2)" {
		t.Fatalf("unexpected insert SQL %q", state.exec)
	}
	expectedArgs := []driver.NamedValue{{Ordinal: 1, Value: int64(99)}, {Ordinal: 2, Value: "Alice"}}
	if !reflect.DeepEqual(state.execArgs, expectedArgs) {
		t.Fatalf("unexpected insert args %#v", state.execArgs)
	}
}

func TestSQLSession_Exec_whenSelectKeyAfter_shouldBackfillKeyAfterInsert(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id"},
		values:  [][]driver.Value{{int64(100)}},
	}
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "Insert",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.Insert",
		Command:       StatementCommandInsert,
		Source:        StatementSourceXML,
		SQL:           "insert into sys_user(name) values(#{Name})",
		ParameterType: "sqlSessionUser",
		KeyProperty:   "ID",
		SelectKey: SelectKeyMeta{
			Enabled:     true,
			KeyProperty: "ID",
			ResultType:  "int64",
			Order:       SelectKeyOrderAfter,
			SQL:         "select currval('sys_user_id_seq')",
		},
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	user := &sqlSessionUser{Name: "Alice"}

	result, err := session.Exec(context.Background(), "system.user.UserMapper.Insert", NamedArgs{
		"user": user,
		"Name": user.Name,
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if user.ID != 100 || result.LastInsertID != 100 {
		t.Fatalf("expected selectKey AFTER to backfill id, result=%#v user=%#v", result, user)
	}
	if state.exec != "insert into sys_user(name) values($1)" {
		t.Fatalf("unexpected insert SQL %q", state.exec)
	}
	if !reflect.DeepEqual(state.queries, []string{"select currval('sys_user_id_seq')"}) {
		t.Fatalf("unexpected selectKey queries %#v", state.queries)
	}
}

func TestSQLSession_Exec_whenParameterTypeUsesTypeHandler_shouldConvertArgument(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "InsertProfile",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.InsertProfile",
		Command:       StatementCommandInsert,
		Source:        StatementSourceAnnotation,
		SQL:           "insert into sys_user(profile) values(#{Profile})",
		ParameterType: "sqlSessionUser",
	})
	session, err := NewSQLSession(registry, state.db, nil, WithTypeHandler("profile", profileTypeHandler{}))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.InsertProfile", NamedArgs{
		"Profile": sqlSessionProfile{Text: "admin"},
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: "admin"}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestSQLSession_Exec_whenNestedParameterTypeUsesTypeHandler_shouldConvertArgumentPath(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:            "InsertProfile",
		Namespace:     "system.user.UserMapper",
		FullName:      "system.user.UserMapper.InsertProfile",
		Command:       StatementCommandInsert,
		Source:        StatementSourceXML,
		SQL:           "insert into sys_user(profile) values(#{user.Profile})",
		ParameterType: "sqlSessionUser",
	})
	session, err := NewSQLSession(registry, state.db, nil, WithTypeHandler("profile", profileTypeHandler{}))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	_, err = session.Exec(context.Background(), "system.user.UserMapper.InsertProfile", NamedArgs{
		"user": &sqlSessionUser{Profile: sqlSessionProfile{Text: "admin"}},
	})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if !reflect.DeepEqual(state.execArgs, []driver.NamedValue{{Ordinal: 1, Value: "admin"}}) {
		t.Fatalf("unexpected exec args %#v", state.execArgs)
	}
}

func TestSQLSession_Query_whenStatementHasDynamicSQL_shouldRenderBeforeCompile(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "ListByIDs",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.ListByIDs",
		Command:   StatementCommandSelect,
		Source:    StatementSourceXML,
		DynamicSQL: []DynamicSQLNode{
			{Kind: DynamicSQLNodeText, Text: "select id, name from sys_user where id in"},
			{
				Kind:       DynamicSQLNodeForeach,
				Collection: "ids",
				Item:       "id",
				Open:       "(",
				Separator:  ",",
				Close:      ")",
				Children:   []DynamicSQLNode{{Kind: DynamicSQLNodeText, Text: "#{id}"}},
			},
		},
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.ListByIDs", NamedArgs{"ids": []int64{7, 8}}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != "select id, name from sys_user where id in ($1, $2)" {
		t.Fatalf("unexpected query %q", state.query)
	}
	expectedArgs := []driver.NamedValue{
		{Ordinal: 1, Value: int64(7)},
		{Ordinal: 2, Value: int64(8)},
	}
	if !reflect.DeepEqual(state.queryArgs, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestSQLSession_Query_whenStatementUsesProvider_shouldCompileProviderSQL(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:         "ListByStatus",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.ListByStatus",
		Command:    StatementCommandSelect,
		Source:     StatementSourceAnnotation,
		Provider:   "UserSQL.ListByStatus",
		Parameters: []string{"status"},
	})
	err := registry.RegisterSQLProvider("UserSQL.ListByStatus", func(ctx context.Context, statement StatementMeta, args NamedArgs) (SQLSource, error) {
		if statement.FullName != "system.user.UserMapper.ListByStatus" || args["status"] != "ACTIVE" {
			t.Fatalf("unexpected provider call statement=%s args=%#v", statement.FullName, args)
		}
		return SQLSource{SQL: "select id, name from sys_user where status = #{status}"}, nil
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.ListByStatus", NamedArgs{"status": "ACTIVE"}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("unexpected users %#v", users)
	}
	if state.query != "select id, name from sys_user where status = $1" {
		t.Fatalf("unexpected query %q", state.query)
	}
	if !reflect.DeepEqual(state.queryArgs, []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}}) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestSQLSession_Query_whenProviderReturnsBuilderArgs_shouldMergeAndBind(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:         "ListByStatus",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.ListByStatus",
		Command:    StatementCommandSelect,
		Source:     StatementSourceAnnotation,
		Provider:   "UserSQL.ListByStatus",
		Parameters: []string{"status"},
	})
	err := registry.RegisterSQLProvider("UserSQL.ListByStatus", func(ctx context.Context, statement StatementMeta, args NamedArgs) (SQLSource, error) {
		return NewSelectSQLBuilder().
			Select("id", "name").
			From("sys_user").
			WhereEq("status", args["status"]).
			OrderByAsc("id").
			Build()
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.ListByStatus", NamedArgs{"status": "ACTIVE"}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("unexpected users %#v", users)
	}
	if state.query != `SELECT "id", "name" FROM "sys_user" WHERE "status" = $1 ORDER BY "id" ASC` {
		t.Fatalf("unexpected query %q", state.query)
	}
	if !reflect.DeepEqual(state.queryArgs, []driver.NamedValue{{Ordinal: 1, Value: "ACTIVE"}}) {
		t.Fatalf("unexpected query args %#v", state.queryArgs)
	}
}

func TestSQLSession_Query_whenStatementUsesMissingProvider_shouldReturnError(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "ListByStatus",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.ListByStatus",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		Provider:  "UserSQL.ListByStatus",
	})
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.ListByStatus", NamedArgs{"status": "ACTIVE"}, &users)
	if err == nil || !strings.Contains(err.Error(), `SQL provider "UserSQL.ListByStatus" is not registered`) {
		t.Fatalf("expected missing provider error, got %v", err)
	}
}

func TestSQLSession_Query_whenProviderDescriptorRejectsStatement_shouldReturnBindingError(t *testing.T) {
	state := openTestSQLState(t)
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "ListByStatus",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.ListByStatus",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		Provider:  "UserSQL.ListByStatus",
	})
	err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor(
		"UserSQL.ListByStatus",
		func(context.Context, StatementMeta, NamedArgs) (SQLSource, error) {
			t.Fatal("provider should not be invoked after descriptor validation failure")
			return SQLSource{}, nil
		},
		WithSQLProviderStatements("system.user.UserMapper.ListOther"),
	))
	if err != nil {
		t.Fatalf("register provider descriptor failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, nil)
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var users []sqlSessionUser
	err = session.Query(context.Background(), "system.user.UserMapper.ListByStatus", NamedArgs{"status": "ACTIVE"}, &users)
	if err == nil || !errors.Is(err, ErrBinding) || !strings.Contains(err.Error(), "is not allowed for statement") {
		t.Fatalf("expected provider descriptor binding error, got %v", err)
	}
}

type sqlProviderCacheContextKey struct{}

func TestSQLSession_QueryOne_whenProviderCacheKeyChanges_shouldBypassLocalCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		Provider:  "UserSQL.FindByID",
	})
	err := registry.RegisterSQLProvider("UserSQL.FindByID", func(ctx context.Context, statement StatementMeta, args NamedArgs) (SQLSource, error) {
		return SQLSource{
			SQL:      "select id, name from sys_user where id = #{id}",
			Args:     NamedArgs{"id": args["id"]},
			CacheKey: ctx.Value(sqlProviderCacheContextKey{}).(string),
		}, nil
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var first sqlSessionUser
	firstCtx := context.WithValue(context.Background(), sqlProviderCacheContextKey{}, "tenant-a")
	if err := session.QueryOne(firstCtx, "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	var second sqlSessionUser
	secondCtx := context.WithValue(context.Background(), sqlProviderCacheContextKey{}, "tenant-b")
	if err := session.QueryOne(secondCtx, "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if first.Name != "Alice" || second.Name != "Bob" || len(state.queries) != 2 {
		t.Fatalf("expected provider cache key to isolate queries, first=%#v second=%#v queries=%#v", first, second, state.queries)
	}
}

func TestSQLSession_QueryPage_whenPageRequested_shouldCountAndQueryRecords(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"count"}, values: [][]driver.Value{{int64(2)}}},
		{
			columns: []string{"id", "name"},
			values: [][]driver.Value{
				{int64(7), "Alice"},
				{int64(8), "Bob"},
			},
		},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "ListPage",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.ListPage",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where status = #{status} order by id",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	page, err := QueryPage[sqlSessionUser](context.Background(), session, "system.user.UserMapper.ListPage", NamedArgs{"status": "ACTIVE"}, NewPageRequest(2, 10))
	if err != nil {
		t.Fatalf("query page failed: %v", err)
	}

	if page.Total != 2 || page.Pages != 1 || page.Current != 2 || page.Size != 10 {
		t.Fatalf("unexpected page metadata %#v", page)
	}
	if len(page.Records) != 2 || page.Records[0].ID != 7 || page.Records[1].Name != "Bob" {
		t.Fatalf("unexpected page records %#v", page.Records)
	}
	expectedQueries := []string{
		"SELECT COUNT(*) FROM (select id, name from sys_user where status = $1) goark_orm_count",
		"select id, name from sys_user where status = $1 order by id LIMIT $2 OFFSET $3",
	}
	if !reflect.DeepEqual(state.queries, expectedQueries) {
		t.Fatalf("unexpected queries %#v", state.queries)
	}
	expectedArgs := [][]driver.NamedValue{
		{{Ordinal: 1, Value: "ACTIVE"}},
		{{Ordinal: 1, Value: "ACTIVE"}, {Ordinal: 2, Value: int64(10)}, {Ordinal: 3, Value: int64(10)}},
	}
	if !reflect.DeepEqual(state.queryArgsList, expectedArgs) {
		t.Fatalf("unexpected query args %#v", state.queryArgsList)
	}
}

func TestSQLSession_QueryPage_whenPaginationInterceptorEnabled_shouldNotApplyContextPaginationTwice(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"count"}, values: [][]driver.Value{{int64(1)}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "ListPage",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.ListPage",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where status = #{status}",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithInterceptors(NewPaginationInterceptor()))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	ctx := WithPageRequest(context.Background(), NewPageRequest(9, 90))

	_, err = QueryPage[sqlSessionUser](ctx, session, "system.user.UserMapper.ListPage", NamedArgs{"status": "ACTIVE"}, NewPageRequest(1, 10))
	if err != nil {
		t.Fatalf("query page failed: %v", err)
	}

	if state.queries[1] != "select id, name from sys_user where status = $1 LIMIT $2 OFFSET $3" {
		t.Fatalf("unexpected paged query %q", state.queries[1])
	}
}

func TestSQLSession_QueryOne_whenLocalCacheHit_shouldReuseDetachedResult(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var first sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	first.Name = "Mutated"
	var second sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if len(state.queries) != 1 {
		t.Fatalf("expected second query to hit local cache, got queries %#v", state.queries)
	}
	if second.ID != 7 || second.Name != "Alice" {
		t.Fatalf("expected detached cached value, got %#v", second)
	}
}

func TestSQLSession_QueryOne_whenConfigurationDisablesLocalCache_shouldQueryEachTime(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	config := DefaultConfiguration().WithLocalCache(false)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var first sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	var second sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if first.Name != "Alice" || second.Name != "Bob" || len(state.queries) != 2 {
		t.Fatalf("expected disabled local cache to query twice, first=%#v second=%#v queries=%#v", first, second, state.queries)
	}
}

func TestSQLSession_QueryOne_whenConfigurationUsesStatementLocalCacheScope_shouldQueryEachTime(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	config := DefaultConfiguration()
	config.LocalCacheScope = LocalCacheScopeStatement
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var first sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	var second sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if first.Name != "Alice" || second.Name != "Bob" || len(state.queries) != 2 {
		t.Fatalf("expected statement local cache scope to query twice, first=%#v second=%#v queries=%#v", first, second, state.queries)
	}
}

func TestSQLSession_QueryOne_whenConfigurationSetsDialect_shouldUseConfiguredDialect(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newSQLSessionRegistry(t, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	config := DefaultConfiguration()
	config.Dialect = NewPostgresDialect()
	session, err := NewSQLSession(registry, state.db, nil, WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &user); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if state.query != "select id, name from sys_user where id = $1" {
		t.Fatalf("unexpected configured dialect query %q", state.query)
	}
	if session.Configuration().Dialect.Name() != "postgres" {
		t.Fatalf("expected postgres configuration dialect, got %s", session.Configuration().Dialect.Name())
	}
}

func TestSQLSession_Configuration_whenGlobalConfigProvided_shouldExposeSnapshot(t *testing.T) {
	state := openTestSQLState(t)
	registry := NewRegistry()
	config := DefaultConfiguration()
	config.GlobalConfig = DefaultGlobalConfig()
	config.GlobalConfig.DbConfig.IDType = IDTypeAssignUUID
	config.GlobalConfig.DbConfig.TablePrefix = "sys_"
	config.GlobalConfig.DbConfig.Schema = "tenant_01"
	config.GlobalConfig.IdentifierGenerator = fixedIdentifierGenerator{uuid: "uuid-1"}
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	snapshot := session.Configuration()
	snapshot.GlobalConfig.DbConfig.TablePrefix = "changed_"

	if session.Configuration().GlobalConfig.DbConfig.IDType != IDTypeAssignUUID {
		t.Fatalf("expected global id type snapshot, got %#v", session.Configuration().GlobalConfig.DbConfig)
	}
	if session.Configuration().GlobalConfig.DbConfig.TablePrefix != "sys_" {
		t.Fatalf("configuration snapshot should not mutate session")
	}
	if session.IdentifierGenerator() == nil {
		t.Fatalf("expected configured identifier generator")
	}
}

func TestSQLSession_Configuration_whenGlobalDbConfigInvalid_shouldReject(t *testing.T) {
	state := openTestSQLState(t)
	config := DefaultConfiguration()
	config.GlobalConfig.DbConfig.IDType = IDType("BAD")

	_, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect(), WithConfiguration(config))
	if err == nil || !strings.Contains(err.Error(), "dbConfig idType") {
		t.Fatalf("expected dbConfig validation error, got %v", err)
	}
}

func TestSQLSession_QueryOne_whenConfigurationMapsUnderscoreToCamelCase_shouldScanAutoMappedField(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"user_name"},
		values:  [][]driver.Value{{"Alice"}},
	}
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{
			{
				ID:        "FindName",
				Namespace: "system.user.UserMapper",
				FullName:  "system.user.UserMapper.FindName",
				Command:   StatementCommandSelect,
				Source:    StatementSourceAnnotation,
				SQL:       "select user_name from sys_user",
			},
		},
	})
	if err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	config := DefaultConfiguration().WithMapUnderscoreToCamelCase(true)
	session, err := NewSQLSession(registry, state.db, nil, WithConfiguration(config))
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var user sqlSessionConfigUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindName", nil, &user); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if user.UserName != "Alice" {
		t.Fatalf("expected underscore column to map to UserName, got %#v", user)
	}
}

func TestSQLSession_QueryOne_whenNamespaceSecondLevelCacheHit_shouldReuseAcrossSessions(t *testing.T) {
	state := openTestSQLState(t)
	state.queryRows = testRowsData{
		columns: []string{"id", "name"},
		values:  [][]driver.Value{{int64(7), "Alice"}},
	}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16}, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	firstSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new first SQL session failed: %v", err)
	}
	secondSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new second SQL session failed: %v", err)
	}

	var first sqlSessionUser
	if err := firstSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	first.Name = "Mutated"
	var second sqlSessionUser
	if err := secondSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if len(state.queries) != 1 {
		t.Fatalf("expected second query to hit namespace cache, got queries %#v", state.queries)
	}
	if second.ID != 7 || second.Name != "Alice" {
		t.Fatalf("expected detached namespace cache value, got %#v", second)
	}
}

func TestSQLSession_QueryOne_whenConfigurationDisablesSecondLevelCache_shouldBypassNamespaceCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16}, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
	})
	config := DefaultConfiguration().WithSecondLevelCache(false)
	firstSession, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new first SQL session failed: %v", err)
	}
	secondSession, err := NewSQLSession(registry, state.db, NewPostgresDialect(), WithConfiguration(config))
	if err != nil {
		t.Fatalf("new second SQL session failed: %v", err)
	}

	var first sqlSessionUser
	if err := firstSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	var second sqlSessionUser
	if err := secondSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if first.Name != "Alice" || second.Name != "Bob" || len(state.queries) != 2 {
		t.Fatalf("expected disabled second-level cache to query twice, first=%#v second=%#v queries=%#v", first, second, state.queries)
	}
}

func TestSQLSession_QueryOne_whenUseCacheDisabled_shouldBypassSecondLevelCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16}, StatementMeta{
		ID:        "FindByID",
		Namespace: "system.user.UserMapper",
		FullName:  "system.user.UserMapper.FindByID",
		Command:   StatementCommandSelect,
		Source:    StatementSourceAnnotation,
		SQL:       "select id, name from sys_user where id = #{id}",
		UseCache:  StatementCacheDisabled,
	})
	firstSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new first SQL session failed: %v", err)
	}
	secondSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new second SQL session failed: %v", err)
	}

	var first sqlSessionUser
	if err := firstSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	var second sqlSessionUser
	if err := secondSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if len(state.queries) != 2 {
		t.Fatalf("expected useCache=false to bypass namespace cache, got queries %#v", state.queries)
	}
	if first.Name != "Alice" || second.Name != "Bob" {
		t.Fatalf("unexpected query results first=%#v second=%#v", first, second)
	}
}

func TestSQLSession_QueryOne_whenSelectFlushCacheEnabled_shouldRefreshSecondLevelCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16}, StatementMeta{
		ID:         "FindByID",
		Namespace:  "system.user.UserMapper",
		FullName:   "system.user.UserMapper.FindByID",
		Command:    StatementCommandSelect,
		Source:     StatementSourceAnnotation,
		SQL:        "select id, name from sys_user where id = #{id}",
		FlushCache: StatementCacheEnabled,
	})
	firstSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new first SQL session failed: %v", err)
	}
	secondSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new second SQL session failed: %v", err)
	}

	var first sqlSessionUser
	if err := firstSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &first); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	var second sqlSessionUser
	if err := secondSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &second); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if first.Name != "Alice" || second.Name != "Bob" {
		t.Fatalf("expected flushCache=true select to refresh cache, first=%#v second=%#v", first, second)
	}
	if len(state.queries) != 2 {
		t.Fatalf("expected select flushCache=true to force second query, got queries %#v", state.queries)
	}
}

func TestSQLSession_QueryOne_whenSelectAffectsData_shouldFlushAndBypassCaches(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Carol"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Dave"}}},
	}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16},
		StatementMeta{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where id = #{id}",
		},
		StatementMeta{
			ID:         "UpsertReturning",
			Namespace:  "system.user.UserMapper",
			FullName:   "system.user.UserMapper.UpsertReturning",
			Command:    StatementCommandSelect,
			Source:     StatementSourceAnnotation,
			SQL:        "insert into sys_user(id, name) values(#{id}, #{name}) returning id, name",
			AffectData: true,
		},
	)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var cached sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &cached); err != nil {
		t.Fatalf("cache warm query failed: %v", err)
	}
	var first sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.UpsertReturning", NamedArgs{"id": int64(7), "name": "Bob"}, &first); err != nil {
		t.Fatalf("first affectData query failed: %v", err)
	}
	var second sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.UpsertReturning", NamedArgs{"id": int64(7), "name": "Carol"}, &second); err != nil {
		t.Fatalf("second affectData query failed: %v", err)
	}
	var after sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &after); err != nil {
		t.Fatalf("post affectData query failed: %v", err)
	}

	if cached.Name != "Alice" || first.Name != "Bob" || second.Name != "Carol" || after.Name != "Dave" {
		t.Fatalf("unexpected affectData cache behavior cached=%#v first=%#v second=%#v after=%#v", cached, first, second, after)
	}
	if len(state.queries) != 4 {
		t.Fatalf("expected affectData select to bypass caches and flush old entries, got queries %#v", state.queries)
	}
}

func TestSQLSession_Exec_whenDefaultWriteOccurs_shouldClearSecondLevelCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	state.execResult = testResult{rowsAffected: 1}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16},
		StatementMeta{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where id = #{id}",
		},
		StatementMeta{
			ID:        "UpdateName",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateName",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = #{id}",
		},
	)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var before sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &before); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	if _, err := session.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Bob"}); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	secondSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new second SQL session failed: %v", err)
	}
	var after sqlSessionUser
	if err := secondSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &after); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if before.Name != "Alice" || after.Name != "Bob" {
		t.Fatalf("unexpected values before=%#v after=%#v", before, after)
	}
	if len(state.queries) != 2 {
		t.Fatalf("expected write to clear namespace cache, got queries %#v", state.queries)
	}
}

func TestSQLSession_Exec_whenFlushCacheDisabled_shouldKeepSecondLevelCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	state.execResult = testResult{rowsAffected: 1}
	registry := newCachedSQLSessionRegistry(t, CacheMeta{Enabled: true, Size: 16},
		StatementMeta{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where id = #{id}",
		},
		StatementMeta{
			ID:         "UpdateName",
			Namespace:  "system.user.UserMapper",
			FullName:   "system.user.UserMapper.UpdateName",
			Command:    StatementCommandUpdate,
			Source:     StatementSourceAnnotation,
			SQL:        "update sys_user set name = #{name} where id = #{id}",
			FlushCache: StatementCacheDisabled,
		},
	)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var before sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &before); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	if _, err := session.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Bob"}); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	secondSession, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new second SQL session failed: %v", err)
	}
	var after sqlSessionUser
	if err := secondSession.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &after); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if before.Name != "Alice" || after.Name != "Alice" {
		t.Fatalf("expected flushCache=false to keep cached value, before=%#v after=%#v", before, after)
	}
	if len(state.queries) != 1 {
		t.Fatalf("expected second query to hit namespace cache, got queries %#v", state.queries)
	}
}

func TestSQLSession_Exec_whenWriteOccurs_shouldClearLocalCache(t *testing.T) {
	state := openTestSQLState(t)
	state.queryResults = []testRowsData{
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Alice"}}},
		{columns: []string{"id", "name"}, values: [][]driver.Value{{int64(7), "Bob"}}},
	}
	state.execResult = testResult{rowsAffected: 1}
	registry := newSQLSessionRegistry(t,
		StatementMeta{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			Source:    StatementSourceAnnotation,
			SQL:       "select id, name from sys_user where id = #{id}",
		},
		StatementMeta{
			ID:        "UpdateName",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.UpdateName",
			Command:   StatementCommandUpdate,
			Source:    StatementSourceAnnotation,
			SQL:       "update sys_user set name = #{name} where id = #{id}",
		},
	)
	session, err := NewSQLSession(registry, state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}

	var before sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &before); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	_, err = session.Exec(context.Background(), "system.user.UserMapper.UpdateName", NamedArgs{"id": int64(7), "name": "Bob"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	var after sqlSessionUser
	if err := session.QueryOne(context.Background(), "system.user.UserMapper.FindByID", NamedArgs{"id": int64(7)}, &after); err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if before.Name != "Alice" || after.Name != "Bob" {
		t.Fatalf("unexpected cached values before=%#v after=%#v", before, after)
	}
	if len(state.queries) != 2 {
		t.Fatalf("expected cache to be cleared after write, got queries %#v", state.queries)
	}
}

func newSQLSessionRegistry(t testing.TB, statements ...StatementMeta) *Registry {
	return newCachedSQLSessionRegistry(t, CacheMeta{}, statements...)
}

func newCachedSQLSessionRegistry(t testing.TB, cache CacheMeta, statements ...StatementMeta) *Registry {
	t.Helper()
	if len(statements) == 0 {
		t.Fatalf("test registry requires at least one statement")
	}
	registry := NewRegistry()
	err := registry.RegisterEntity(EntityMeta{
		TypeName: "sqlSessionUser",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", ColumnName: "id", PrimaryKey: true},
			{FieldName: "Name", ColumnName: "name"},
			{FieldName: "Profile", ColumnName: "profile", TypeHandler: "profile"},
		},
	})
	if err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	err = registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Cache:     cache,
		ResultMaps: []ResultMapMeta{
			{
				ID:       "UserResult",
				TypeName: "sqlSessionUser",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "id", ID: true},
					{Property: "Profile", Column: "profile", TypeHandler: "profile"},
				},
			},
		},
		Statements: statements,
	})
	if err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	return registry
}

func newOrderSessionRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: "system.order.OrderMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "OrderResult",
				TypeName: "sqlSessionOrder",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "order_id", ID: true},
					{Property: "Name", Column: "order_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property: "User",
						TypeName: "sqlSessionUser",
						Fields: []ResultFieldMeta{
							{Property: "ID", Column: "user_id", ID: true},
							{Property: "Name", Column: "user_name"},
						},
					},
				},
				Collections: []ResultCollectionMeta{
					{
						Property: "Items",
						TypeName: "sqlSessionOrderItem",
						Fields: []ResultFieldMeta{
							{Property: "ID", Column: "item_id", ID: true},
							{Property: "SKU", Column: "item_sku"},
						},
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "FindByID",
				Namespace: "system.order.OrderMapper",
				FullName:  "system.order.OrderMapper.FindByID",
				Command:   StatementCommandSelect,
				Source:    StatementSourceXML,
				SQL:       "select * from orders where id = #{id}",
				ResultMap: "OrderResult",
			},
		},
	})
	if err != nil {
		t.Fatalf("register order mapper failed: %v", err)
	}
	return registry
}

func newDiscriminatorAccountRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "AccountMapper",
		Namespace: "system.account.AccountMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "AccountResult",
				TypeName: "sqlSessionAccount",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "id", ID: true},
					{Property: "Kind", Column: "kind"},
					{Property: "Name", Column: "name"},
				},
				Discriminator: ResultDiscriminatorMeta{
					Column:   "kind",
					TypeName: "string",
					Cases: []ResultDiscriminatorCaseMeta{
						{
							Value:      "admin",
							ResultType: "sqlSessionAccount",
							Fields: []ResultFieldMeta{
								{Property: "Level", Column: "admin_level"},
							},
						},
						{
							Value:     "vip",
							ResultMap: "VipAccountResult",
						},
					},
				},
			},
			{
				ID:       "VipAccountResult",
				TypeName: "sqlSessionAccount",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "id", ID: true},
					{Property: "Kind", Column: "kind"},
					{Property: "Name", Column: "name"},
					{Property: "Phone", Column: "phone_number"},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "List",
				Namespace: "system.account.AccountMapper",
				FullName:  "system.account.AccountMapper.List",
				Command:   StatementCommandSelect,
				Source:    StatementSourceXML,
				SQL:       "select * from account",
				ResultMap: "AccountResult",
			},
		},
	})
	if err != nil {
		t.Fatalf("register discriminator mapper failed: %v", err)
	}
	return registry
}

func newMismatchedDiscriminatorAccountRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "AccountMapper",
		Namespace: "system.account.AccountMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "AccountResult",
				TypeName: "sqlSessionAccount",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "id", ID: true},
					{Property: "Kind", Column: "kind"},
					{Property: "Name", Column: "name"},
				},
				Discriminator: ResultDiscriminatorMeta{
					Column:   "kind",
					TypeName: "string",
					Cases: []ResultDiscriminatorCaseMeta{
						{Value: "admin", ResultType: "sqlSessionAdminAccount"},
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "List",
				Namespace: "system.account.AccountMapper",
				FullName:  "system.account.AccountMapper.List",
				Command:   StatementCommandSelect,
				Source:    StatementSourceXML,
				SQL:       "select * from account",
				ResultMap: "AccountResult",
			},
		},
	})
	if err != nil {
		t.Fatalf("register mismatched discriminator mapper failed: %v", err)
	}
	return registry
}

func newNestedSelectOrderRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: "system.order.OrderMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "OrderNestedResult",
				TypeName: "sqlSessionOrder",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "order_id", ID: true},
					{Property: "UserID", Column: "user_id"},
					{Property: "Name", Column: "order_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property:  "User",
						TypeName:  "sqlSessionUser",
						Column:    "user_id",
						Select:    "system.user.UserMapper.FindByID",
						FetchType: "eager",
					},
				},
				Collections: []ResultCollectionMeta{
					{
						Property:  "Items",
						TypeName:  "sqlSessionOrderItem",
						Column:    "order_id",
						Select:    "system.order.OrderItemMapper.ListByOrderID",
						FetchType: "eager",
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:         "FindWithNested",
				Namespace:  "system.order.OrderMapper",
				FullName:   "system.order.OrderMapper.FindWithNested",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select order_id, user_id, order_name from orders where order_id = #{id}",
				ResultMap:  "OrderNestedResult",
				Parameters: []string{"id"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register order mapper failed: %v", err)
	}
	err = registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{
			{
				ID:         "FindByID",
				Namespace:  "system.user.UserMapper",
				FullName:   "system.user.UserMapper.FindByID",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select id, name from sys_user where id = #{id}",
				Parameters: []string{"id"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register user mapper failed: %v", err)
	}
	err = registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderItemMapper",
		Namespace: "system.order.OrderItemMapper",
		Statements: []StatementMeta{
			{
				ID:         "ListByOrderID",
				Namespace:  "system.order.OrderItemMapper",
				FullName:   "system.order.OrderItemMapper.ListByOrderID",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select id, sku from order_item where order_id = #{orderID}",
				Parameters: []string{"orderID"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register order item mapper failed: %v", err)
	}
	return registry
}

func newLazyNestedSelectOrderRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: "system.order.OrderMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "OrderLazyResult",
				TypeName: "sqlSessionLazyOrder",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "order_id", ID: true},
					{Property: "UserID", Column: "user_id"},
					{Property: "Name", Column: "order_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property:  "User",
						TypeName:  "sqlSessionUser",
						Column:    "user_id",
						Select:    "system.user.UserMapper.FindByID",
						FetchType: "lazy",
					},
				},
				Collections: []ResultCollectionMeta{
					{
						Property:  "Items",
						TypeName:  "sqlSessionOrderItem",
						Column:    "order_id",
						Select:    "system.order.OrderItemMapper.ListByOrderID",
						FetchType: "lazy",
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:         "FindWithLazy",
				Namespace:  "system.order.OrderMapper",
				FullName:   "system.order.OrderMapper.FindWithLazy",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select order_id, user_id, order_name from orders where order_id = #{id}",
				ResultMap:  "OrderLazyResult",
				Parameters: []string{"id"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register lazy order mapper failed: %v", err)
	}
	err = registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{
			{
				ID:         "FindByID",
				Namespace:  "system.user.UserMapper",
				FullName:   "system.user.UserMapper.FindByID",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select id, name from sys_user where id = #{id}",
				Parameters: []string{"id"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register lazy user mapper failed: %v", err)
	}
	err = registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderItemMapper",
		Namespace: "system.order.OrderItemMapper",
		Statements: []StatementMeta{
			{
				ID:         "ListByOrderID",
				Namespace:  "system.order.OrderItemMapper",
				FullName:   "system.order.OrderItemMapper.ListByOrderID",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select id, sku from order_item where order_id = #{orderID}",
				Parameters: []string{"orderID"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register lazy order item mapper failed: %v", err)
	}
	return registry
}

func newCompositeNestedSelectOrderRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: "system.order.OrderMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "OrderCompositeNestedResult",
				TypeName: "sqlSessionOrder",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "order_id", ID: true},
					{Property: "UserID", Column: "user_id"},
					{Property: "UserKind", Column: "user_kind"},
					{Property: "Name", Column: "order_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property:  "User",
						TypeName:  "sqlSessionUser",
						Column:    "{id=user_id,kind=user_kind}",
						Select:    "system.user.UserMapper.FindByIDAndKind",
						FetchType: "eager",
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:         "FindWithCompositeNested",
				Namespace:  "system.order.OrderMapper",
				FullName:   "system.order.OrderMapper.FindWithCompositeNested",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select order_id, user_id, user_kind, order_name from orders where order_id = #{id}",
				ResultMap:  "OrderCompositeNestedResult",
				Parameters: []string{"id"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register composite order mapper failed: %v", err)
	}
	err = registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{
			{
				ID:         "FindByIDAndKind",
				Namespace:  "system.user.UserMapper",
				FullName:   "system.user.UserMapper.FindByIDAndKind",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select id, name from sys_user where id = #{id} and kind = #{kind}",
				Parameters: []string{"id", "kind"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register composite user mapper failed: %v", err)
	}
	return registry
}

func newDedupNestedSelectOrderRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: "system.order.OrderMapper",
		ResultMaps: []ResultMapMeta{
			{
				ID:       "OrderNestedResult",
				TypeName: "sqlSessionOrder",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "order_id", ID: true},
					{Property: "UserID", Column: "user_id"},
					{Property: "Name", Column: "order_name"},
				},
				Associations: []ResultAssociationMeta{
					{
						Property:  "User",
						TypeName:  "sqlSessionUser",
						Column:    "user_id",
						Select:    "system.user.UserMapper.FindByID",
						FetchType: "eager",
					},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "ListWithNested",
				Namespace: "system.order.OrderMapper",
				FullName:  "system.order.OrderMapper.ListWithNested",
				Command:   StatementCommandSelect,
				Source:    StatementSourceXML,
				SQL:       "select order_id, user_id, order_name from orders",
				ResultMap: "OrderNestedResult",
			},
		},
	})
	if err != nil {
		t.Fatalf("register dedup order mapper failed: %v", err)
	}
	err = registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{
			{
				ID:         "FindByID",
				Namespace:  "system.user.UserMapper",
				FullName:   "system.user.UserMapper.FindByID",
				Command:    StatementCommandSelect,
				Source:     StatementSourceXML,
				SQL:        "select id, name from sys_user where id = #{id}",
				Parameters: []string{"id"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register dedup user mapper failed: %v", err)
	}
	return registry
}

type profileTypeHandler struct{}

func (profileTypeHandler) ToDB(ctx context.Context, value any) (any, error) {
	profile, ok := value.(sqlSessionProfile)
	if !ok {
		return nil, fmt.Errorf("unsupported profile value %T", value)
	}
	return profile.Text, nil
}

func (profileTypeHandler) FromDB(ctx context.Context, value any, target any) error {
	profile, ok := target.(*sqlSessionProfile)
	if !ok {
		return fmt.Errorf("unsupported profile target %T", target)
	}
	switch item := value.(type) {
	case string:
		profile.Text = item
	case []byte:
		profile.Text = string(item)
	default:
		return fmt.Errorf("unsupported profile database value %T", value)
	}
	return nil
}

type recordingFetchSizeExecutor struct {
	db           *sql.DB
	mu           sync.Mutex
	fetchSizes   []int
	fetchQueries []string
}

func (e *recordingFetchSizeExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return e.db.QueryContext(ctx, query, args...)
}

func (e *recordingFetchSizeExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return e.db.ExecContext(ctx, query, args...)
}

func (e *recordingFetchSizeExecutor) ApplyFetchSize(ctx context.Context, query string, fetchSize int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fetchSizes = append(e.fetchSizes, fetchSize)
	e.fetchQueries = append(e.fetchQueries, query)
	return nil
}

type recordingStatementOptionsExecutor struct {
	db      *sql.DB
	mu      sync.Mutex
	options []StatementOptions
	queries []string
}

func (e *recordingStatementOptionsExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return e.db.QueryContext(ctx, query, args...)
}

func (e *recordingStatementOptionsExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return e.db.ExecContext(ctx, query, args...)
}

func (e *recordingStatementOptionsExecutor) ApplyStatementOptions(ctx context.Context, query string, options StatementOptions) error {
	_ = ctx
	e.mu.Lock()
	defer e.mu.Unlock()
	e.options = append(e.options, options)
	e.queries = append(e.queries, query)
	return nil
}

type testSQLState struct {
	db                 *sql.DB
	queryRows          testRowsData
	queryResults       []testRowsData
	execResult         driver.Result
	execResults        []driver.Result
	execErrors         []error
	outValues          []any
	query              string
	queries            []string
	exec               string
	execs              []string
	queryArgs          []driver.NamedValue
	queryArgsList      [][]driver.NamedValue
	execArgs           []driver.NamedValue
	execArgsList       [][]driver.NamedValue
	beginCount         int
	commitCount        int
	rollbackCount      int
	txOptions          []driver.TxOptions
	rowsClosed         int
	prepareCount       int
	prepareQueries     []string
	stmtClosed         int
	queryHadDeadline   bool
	queryDeadline      time.Time
	execHadDeadline    bool
	execDeadline       time.Time
	prepareHadDeadline bool
	mu                 sync.Mutex
}

type testRowsData struct {
	columns    []string
	values     [][]driver.Value
	resultSets []testRowsData
}

var testSQLDrivers sync.Map

func openTestSQLState(t testing.TB) *testSQLState {
	t.Helper()
	id := strconv.FormatInt(int64(testNameCounter.Add(1)), 10)
	name := "goark_orm_sqlsession_" + strings.ReplaceAll(t.Name(), "/", "_") + "_" + id
	dsn := name
	sql.Register(name, testDriver{})
	state := &testSQLState{}
	testSQLDrivers.Store(dsn, state)
	t.Cleanup(func() {
		testSQLDrivers.Delete(dsn)
		if state.db != nil {
			_ = state.db.Close()
		}
	})
	db, err := sql.Open(name, dsn)
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	state.db = db
	return state
}

var testNameCounter atomicCounter

type atomicCounter struct {
	mu    sync.Mutex
	value int
}

func (c *atomicCounter) Add(delta int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += delta
	return c.value
}

type testDriver struct{}

func (testDriver) Open(name string) (driver.Conn, error) {
	value, ok := testSQLDrivers.Load(name)
	if !ok {
		return nil, fmt.Errorf("test SQL state %q not found", name)
	}
	return &testConn{state: value.(*testSQLState)}, nil
}

type testConn struct {
	state *testSQLState
}

func (c *testConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *testConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.prepareCount++
	c.state.prepareQueries = append(c.state.prepareQueries, query)
	_, c.state.prepareHadDeadline = ctx.Deadline()
	return &testStmt{state: c.state, query: query}, nil
}

func (c *testConn) Close() error {
	return nil
}

func (c *testConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *testConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.beginCount++
	c.state.txOptions = append(c.state.txOptions, opts)
	return &testTx{state: c.state}, nil
}

func (c *testConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.state.queryContext(ctx, query, args)
}

func (c *testConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.state.execContext(ctx, query, args)
}

func (c *testConn) CheckNamedValue(value *driver.NamedValue) error {
	return nil
}

func (s *testSQLState) queryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.query = query
	s.queries = append(s.queries, query)
	s.queryArgs = append([]driver.NamedValue(nil), args...)
	s.queryArgsList = append(s.queryArgsList, append([]driver.NamedValue(nil), args...))
	s.queryDeadline, s.queryHadDeadline = ctx.Deadline()
	rows := s.queryRows
	if len(s.queryResults) > 0 {
		rows = s.queryResults[0]
		s.queryResults = s.queryResults[1:]
	}
	return &testRows{
		columns:    append([]string(nil), rows.columns...),
		values:     append([][]driver.Value(nil), rows.values...),
		resultSets: append([]testRowsData(nil), rows.resultSets...),
		state:      s,
	}, nil
}

func (s *testSQLState) execContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exec = query
	s.execs = append(s.execs, query)
	s.execArgs = append([]driver.NamedValue(nil), args...)
	s.execArgsList = append(s.execArgsList, append([]driver.NamedValue(nil), args...))
	s.execDeadline, s.execHadDeadline = ctx.Deadline()
	if len(s.execErrors) > 0 {
		err := s.execErrors[0]
		s.execErrors = s.execErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(s.execResults) > 0 {
		result := s.execResults[0]
		s.execResults = s.execResults[1:]
		assignOutValues(args, s.takeOutValues(len(args)))
		return result, nil
	}
	assignOutValues(args, s.takeOutValues(len(args)))
	if s.execResult == nil {
		return testResult{}, nil
	}
	return s.execResult, nil
}

func (s *testSQLState) takeOutValues(max int) []any {
	if len(s.outValues) == 0 || max <= 0 {
		return nil
	}
	count := len(s.outValues)
	if count > max {
		count = max
	}
	out := append([]any(nil), s.outValues[:count]...)
	s.outValues = s.outValues[count:]
	return out
}

func assignOutValues(args []driver.NamedValue, values []any) {
	if len(values) == 0 {
		return
	}
	valueIndex := 0
	for _, arg := range args {
		out, ok := arg.Value.(sql.Out)
		if !ok {
			continue
		}
		if valueIndex >= len(values) {
			return
		}
		assignOutDestination(out.Dest, values[valueIndex])
		valueIndex++
	}
}

func assignOutDestination(dest any, value any) {
	rv := reflect.ValueOf(dest)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	target := rv.Elem()
	source := reflect.ValueOf(value)
	if !source.IsValid() {
		target.Set(reflect.Zero(target.Type()))
		return
	}
	if source.Type().AssignableTo(target.Type()) {
		target.Set(source)
		return
	}
	if source.Type().ConvertibleTo(target.Type()) {
		target.Set(source.Convert(target.Type()))
	}
}

type testStmt struct {
	state *testSQLState
	query string
}

func (s *testStmt) Close() error {
	if s.state != nil {
		s.state.mu.Lock()
		defer s.state.mu.Unlock()
		s.state.stmtClosed++
	}
	return nil
}

func (s *testStmt) NumInput() int {
	return -1
}

func (s *testStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), driverValuesToNamedValues(args))
}

func (s *testStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), driverValuesToNamedValues(args))
}

func (s *testStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.state.execContext(ctx, s.query, args)
}

func (s *testStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.state.queryContext(ctx, s.query, args)
}

func driverValuesToNamedValues(values []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, 0, len(values))
	for index, value := range values {
		out = append(out, driver.NamedValue{Ordinal: index + 1, Value: value})
	}
	return out
}

type testTx struct {
	state *testSQLState
}

func (t *testTx) Commit() error {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	t.state.commitCount++
	return nil
}

func (t *testTx) Rollback() error {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	t.state.rollbackCount++
	return nil
}

type testRows struct {
	columns    []string
	values     [][]driver.Value
	resultSets []testRowsData
	index      int
	state      *testSQLState
}

func (r *testRows) Columns() []string {
	return r.columns
}

func (r *testRows) Close() error {
	if r.state != nil {
		r.state.mu.Lock()
		defer r.state.mu.Unlock()
		r.state.rowsClosed++
	}
	return nil
}

func (r *testRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	row := r.values[r.index]
	r.index++
	copy(dest, row)
	return nil
}

func (r *testRows) HasNextResultSet() bool {
	return len(r.resultSets) > 0
}

func (r *testRows) NextResultSet() error {
	if len(r.resultSets) == 0 {
		return io.EOF
	}
	next := r.resultSets[0]
	remaining := append([]testRowsData(nil), r.resultSets[1:]...)
	r.columns = append([]string(nil), next.columns...)
	r.values = append([][]driver.Value(nil), next.values...)
	r.resultSets = append(append([]testRowsData(nil), next.resultSets...), remaining...)
	r.index = 0
	return nil
}

type testResult struct {
	rowsAffected int64
	lastInsertID int64
}

func (r testResult) LastInsertId() (int64, error) {
	return r.lastInsertID, nil
}

func (r testResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}
