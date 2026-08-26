package orm

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRoutingSession_Query_whenContextDataSourceProvided_shouldRouteToContextSession(t *testing.T) {
	primary := &recordingRoutingSession{}
	tenant := &recordingRoutingSession{}
	session, err := NewRoutingSession(map[DataSourceKey]Session{
		DefaultDataSourceKey: primary,
		"tenant_b":           tenant,
	}, StaticDataSourceResolver(DefaultDataSourceKey))
	if err != nil {
		t.Fatalf("new routing session failed: %v", err)
	}

	var users []baseMapperUser
	err = session.Query(WithDataSource(context.Background(), "tenant_b"), "system.user.UserMapper.List", NamedArgs{"status": "ACTIVE"}, &users)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(primary.queries) != 0 {
		t.Fatalf("primary should not receive query: %#v", primary.queries)
	}
	if !reflect.DeepEqual(tenant.queries, []string{"system.user.UserMapper.List"}) {
		t.Fatalf("unexpected tenant queries %#v", tenant.queries)
	}
}

func TestRoutingSession_whenReadWriteResolverProvided_shouldSplitReadAndWrite(t *testing.T) {
	read := &recordingRoutingSession{}
	write := &recordingRoutingSession{recordingBasicSession: recordingBasicSession{result: Result{RowsAffected: 1}}}
	session, err := NewRoutingSession(map[DataSourceKey]Session{
		"read":  read,
		"write": write,
	}, ReadWriteDataSourceResolver("read", "write"), WithRoutingDefaultDataSource("write"))
	if err != nil {
		t.Fatalf("new routing session failed: %v", err)
	}

	var users []baseMapperUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", nil, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	result, err := session.Exec(context.Background(), "system.user.UserMapper.Update", NamedArgs{"id": int64(7)})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
	if !reflect.DeepEqual(read.queries, []string{"system.user.UserMapper.List"}) {
		t.Fatalf("unexpected read queries %#v", read.queries)
	}
	if !reflect.DeepEqual(write.execs, []string{"system.user.UserMapper.Update"}) {
		t.Fatalf("unexpected write execs %#v", write.execs)
	}
}

func TestRoutingSession_whenRegistryProvided_shouldExposeStatementCommandToResolver(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{
			{
				ID:        "List",
				Namespace: "system.user.UserMapper",
				FullName:  "system.user.UserMapper.List",
				Command:   StatementCommandSelect,
			},
		},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}
	read := &recordingRoutingSession{}
	var observed StatementCommand
	session, err := NewRoutingSession(map[DataSourceKey]Session{
		"read": read,
	}, func(ctx context.Context, operation RoutingOperation) (DataSourceKey, error) {
		observed = operation.Command
		return "read", nil
	}, WithRoutingDefaultDataSource("read"), WithRoutingRegistry(registry))
	if err != nil {
		t.Fatalf("new routing session failed: %v", err)
	}

	var users []baseMapperUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", nil, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if observed != StatementCommandSelect {
		t.Fatalf("unexpected command %q", observed)
	}
}

func TestRoutingSession_QueryPage_whenDelegateSupportsPage_shouldRoutePageQuery(t *testing.T) {
	read := &recordingRoutingSession{pageResult: PageQueryResult{Total: 12, Size: 5, Current: 2, Pages: 3}}
	session, err := NewRoutingSession(map[DataSourceKey]Session{
		"read": read,
	}, StaticDataSourceResolver("read"), WithRoutingDefaultDataSource("read"))
	if err != nil {
		t.Fatalf("new routing session failed: %v", err)
	}

	var users []baseMapperUser
	result, err := session.QueryPage(context.Background(), "system.user.UserMapper.ListPage", nil, NewPageRequest(2, 5), &users)
	if err != nil {
		t.Fatalf("query page failed: %v", err)
	}

	if result.Total != 12 || result.Pages != 3 {
		t.Fatalf("unexpected page result %#v", result)
	}
	if !reflect.DeepEqual(read.pages, []string{"system.user.UserMapper.ListPage"}) {
		t.Fatalf("unexpected page calls %#v", read.pages)
	}
}

func TestRoutingSession_QueryPage_whenDelegateDoesNotSupportPageAndCountDisabled_shouldFallbackToQuery(t *testing.T) {
	read := &recordingBasicSession{}
	session, err := NewRoutingSession(map[DataSourceKey]Session{
		"read": read,
	}, StaticDataSourceResolver("read"), WithRoutingDefaultDataSource("read"))
	if err != nil {
		t.Fatalf("new routing session failed: %v", err)
	}

	var users []baseMapperUser
	page := NewPageRequest(2, 5)
	page.SearchCount = false
	result, err := session.QueryPage(context.Background(), "system.user.UserMapper.ListPage", nil, page, &users)
	if err != nil {
		t.Fatalf("query page failed: %v", err)
	}

	if result.Size != 5 || result.Current != 2 {
		t.Fatalf("unexpected page result %#v", result)
	}
	if !reflect.DeepEqual(read.queries, []string{"system.user.UserMapper.ListPage"}) {
		t.Fatalf("unexpected query fallback %#v", read.queries)
	}
}

func TestRoutingSessionFactory_OpenSession_whenOpenersProvided_shouldOpenAndCloseDelegates(t *testing.T) {
	primary := &recordingRoutingSession{}
	replica := &recordingRoutingSession{}
	factory, err := NewRoutingSessionFactory(map[DataSourceKey]SessionOpenFunc{
		DefaultDataSourceKey: func() (Session, error) { return primary, nil },
		"replica":            func() (Session, error) { return replica, nil },
	}, StaticDataSourceResolver("replica"))
	if err != nil {
		t.Fatalf("new routing session factory failed: %v", err)
	}

	session, err := factory.OpenSession()
	if err != nil {
		t.Fatalf("open routing session failed: %v", err)
	}
	var users []baseMapperUser
	if err := session.Query(context.Background(), "system.user.UserMapper.List", nil, &users); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if !reflect.DeepEqual(replica.queries, []string{"system.user.UserMapper.List"}) {
		t.Fatalf("unexpected replica queries %#v", replica.queries)
	}
	if !primary.closed || !replica.closed {
		t.Fatalf("expected delegates to be closed")
	}
}

func TestRoutingSessionFactory_OpenSession_whenOpenerFails_shouldCloseOpenedDelegates(t *testing.T) {
	opened := &recordingRoutingSession{}
	expectedErr := errors.New("open replica failed")
	factory, err := NewRoutingSessionFactory(map[DataSourceKey]SessionOpenFunc{
		DefaultDataSourceKey: func() (Session, error) { return opened, nil },
		"replica":            func() (Session, error) { return nil, expectedErr },
	}, StaticDataSourceResolver(DefaultDataSourceKey))
	if err != nil {
		t.Fatalf("new routing session factory failed: %v", err)
	}

	_, err = factory.OpenSession()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected opener error, got %v", err)
	}
	if !opened.closed {
		t.Fatalf("expected opened delegate to be closed")
	}
}

type recordingBasicSession struct {
	queries  []string
	queryOne []string
	execs    []string
	result   Result
}

func (s *recordingBasicSession) Query(ctx context.Context, statement string, args NamedArgs, dest any) error {
	s.queries = append(s.queries, statement)
	return nil
}

func (s *recordingBasicSession) QueryOne(ctx context.Context, statement string, args NamedArgs, dest any) error {
	s.queryOne = append(s.queryOne, statement)
	return nil
}

func (s *recordingBasicSession) Exec(ctx context.Context, statement string, args NamedArgs) (Result, error) {
	s.execs = append(s.execs, statement)
	return s.result, nil
}

type recordingRoutingSession struct {
	recordingBasicSession
	pages      []string
	pageResult PageQueryResult
	closed     bool
}

func (s *recordingRoutingSession) QueryPage(ctx context.Context, statement string, args NamedArgs, page PageRequest, dest any) (PageQueryResult, error) {
	s.pages = append(s.pages, statement)
	return s.pageResult, nil
}

func (s *recordingRoutingSession) Close() error {
	s.closed = true
	return nil
}
