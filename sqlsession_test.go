package orm

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
)

type sqlSessionUser struct {
	ID      int64
	Name    string
	Profile sqlSessionProfile
}

type sqlSessionProfile struct {
	Text string
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

	var user sqlSessionUser
	err = session.QueryOne(context.Background(), "system.user.UserMapper.FindProfile", NamedArgs{"id": int64(7)}, &user)
	if err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	if user.Profile.Text != "admin" {
		t.Fatalf("unexpected profile %#v", user.Profile)
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

func newSQLSessionRegistry(t *testing.T, statement StatementMeta) *Registry {
	t.Helper()
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
		Statements: []StatementMeta{statement},
	})
	if err != nil {
		t.Fatalf("register mapper failed: %v", err)
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

type testSQLState struct {
	db         *sql.DB
	queryRows  testRowsData
	execResult driver.Result
	query      string
	exec       string
	queryArgs  []driver.NamedValue
	execArgs   []driver.NamedValue
	mu         sync.Mutex
}

type testRowsData struct {
	columns []string
	values  [][]driver.Value
}

var testSQLDrivers sync.Map

func openTestSQLState(t *testing.T) *testSQLState {
	t.Helper()
	name := "goark_orm_sqlsession_" + strings.ReplaceAll(t.Name(), "/", "_")
	dsn := name + "_" + strconv.FormatInt(int64(testNameCounter.Add(1)), 10)
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
	return nil, fmt.Errorf("prepare is not supported")
}

func (c *testConn) Close() error {
	return nil
}

func (c *testConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("tx is not supported")
}

func (c *testConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.query = query
	c.state.queryArgs = append([]driver.NamedValue(nil), args...)
	return &testRows{
		columns: append([]string(nil), c.state.queryRows.columns...),
		values:  append([][]driver.Value(nil), c.state.queryRows.values...),
	}, nil
}

func (c *testConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.exec = query
	c.state.execArgs = append([]driver.NamedValue(nil), args...)
	if c.state.execResult == nil {
		return testResult{}, nil
	}
	return c.state.execResult, nil
}

type testRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *testRows) Columns() []string {
	return r.columns
}

func (r *testRows) Close() error {
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
