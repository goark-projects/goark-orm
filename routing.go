package orm

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// DataSourceKey 表示一个逻辑数据源名称。
type DataSourceKey string

const (
	// DefaultDataSourceKey 表示默认逻辑数据源。
	DefaultDataSourceKey DataSourceKey = "default"
)

type dataSourceContextKey struct{}

// WithDataSource 在 context 中声明本次 ORM 调用使用的数据源。
func WithDataSource(ctx context.Context, key DataSourceKey) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, dataSourceContextKey{}, key)
}

// DataSourceFromContext 从 context 读取数据源声明。
func DataSourceFromContext(ctx context.Context) (DataSourceKey, bool) {
	if ctx == nil {
		return "", false
	}
	key, ok := ctx.Value(dataSourceContextKey{}).(DataSourceKey)
	key = normalizeDataSourceKey(key)
	return key, ok && key != ""
}

// RoutingOperationKind 表示路由时的 ORM 操作类型。
type RoutingOperationKind string

const (
	// RoutingOperationQuery 表示多行查询。
	RoutingOperationQuery RoutingOperationKind = "query"
	// RoutingOperationQueryOne 表示单行查询。
	RoutingOperationQueryOne RoutingOperationKind = "query-one"
	// RoutingOperationExec 表示写语句执行。
	RoutingOperationExec RoutingOperationKind = "exec"
	// RoutingOperationCall 表示存储过程调用。
	RoutingOperationCall RoutingOperationKind = "call"
	// RoutingOperationPage 表示分页查询。
	RoutingOperationPage RoutingOperationKind = "page"
)

// RoutingOperation 描述一次待路由的 ORM 操作。
type RoutingOperation struct {
	Kind      RoutingOperationKind
	Statement string
	Command   StatementCommand
	Args      NamedArgs
	Page      PageRequest
}

// DataSourceResolver 根据 ORM 操作选择逻辑数据源。
type DataSourceResolver func(ctx context.Context, operation RoutingOperation) (DataSourceKey, error)

// StaticDataSourceResolver 始终返回固定数据源。
func StaticDataSourceResolver(key DataSourceKey) DataSourceResolver {
	key = normalizeDataSourceKey(key)
	return func(ctx context.Context, operation RoutingOperation) (DataSourceKey, error) {
		return key, nil
	}
}

// ReadWriteDataSourceResolver 按读写操作拆分数据源。
func ReadWriteDataSourceResolver(read DataSourceKey, write DataSourceKey) DataSourceResolver {
	read = normalizeDataSourceKey(read)
	write = normalizeDataSourceKey(write)
	return func(ctx context.Context, operation RoutingOperation) (DataSourceKey, error) {
		if operation.Kind == RoutingOperationExec || operation.Kind == RoutingOperationCall || statementIsWrite(operation.Command) {
			return write, nil
		}
		return read, nil
	}
}

// StatementDataSourceResolver 按 Statement 全名优先路由，未命中时回退到 fallback。
func StatementDataSourceResolver(routes map[string]DataSourceKey, fallback DataSourceResolver) DataSourceResolver {
	copied := make(map[string]DataSourceKey, len(routes))
	for statement, key := range routes {
		statement = strings.TrimSpace(statement)
		key = normalizeDataSourceKey(key)
		if statement != "" && key != "" {
			copied[statement] = key
		}
	}
	return func(ctx context.Context, operation RoutingOperation) (DataSourceKey, error) {
		if key, ok := copied[operation.Statement]; ok {
			return key, nil
		}
		if fallback != nil {
			return fallback(ctx, operation)
		}
		return "", nil
	}
}

// RoutingSessionOption 配置多数据源路由 Session。
type RoutingSessionOption func(*RoutingSession) error

// WithRoutingRegistry 配置用于读取 StatementCommand 的 Registry。
func WithRoutingRegistry(registry *Registry) RoutingSessionOption {
	return func(session *RoutingSession) error {
		if registry == nil {
			return configurationErrorf("routing registry is nil")
		}
		session.registry = registry
		return nil
	}
}

// WithRoutingDefaultDataSource 配置默认数据源。
func WithRoutingDefaultDataSource(key DataSourceKey) RoutingSessionOption {
	return func(session *RoutingSession) error {
		key = normalizeDataSourceKey(key)
		if key == "" {
			return configurationErrorf("routing default data source is empty")
		}
		session.defaultKey = key
		return nil
	}
}

// RoutingSession 按 context 和路由规则把 ORM 操作转发到具体 Session。
type RoutingSession struct {
	sessions   map[DataSourceKey]Session
	resolver   DataSourceResolver
	defaultKey DataSourceKey
	registry   *Registry
}

var _ Session = (*RoutingSession)(nil)
var _ PageQuerySession = (*RoutingSession)(nil)
var _ CallSession = (*RoutingSession)(nil)

// NewRoutingSession 创建多数据源路由 Session。
func NewRoutingSession(sessions map[DataSourceKey]Session, resolver DataSourceResolver, options ...RoutingSessionOption) (*RoutingSession, error) {
	return newRoutingSession(sessions, resolver, options...)
}

func newRoutingSession(sessions map[DataSourceKey]Session, resolver DataSourceResolver, options ...RoutingSessionOption) (*RoutingSession, error) {
	copied, defaultKey, err := normalizeRoutingSessions(sessions)
	if err != nil {
		return nil, err
	}
	session := &RoutingSession{
		sessions:   copied,
		resolver:   resolver,
		defaultKey: defaultKey,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(session); err != nil {
			return nil, err
		}
	}
	if _, ok := session.sessions[session.defaultKey]; !ok {
		return nil, configurationErrorf("routing default data source %q is not registered", session.defaultKey)
	}
	return session, nil
}

// Query 路由并执行多行查询。
func (s *RoutingSession) Query(ctx context.Context, statement string, args NamedArgs, dest any) error {
	delegate, _, err := s.delegate(ctx, RoutingOperation{Kind: RoutingOperationQuery, Statement: statement, Args: args})
	if err != nil {
		return err
	}
	return delegate.Query(ctx, statement, args, dest)
}

// QueryOne 路由并执行单行查询。
func (s *RoutingSession) QueryOne(ctx context.Context, statement string, args NamedArgs, dest any) error {
	delegate, _, err := s.delegate(ctx, RoutingOperation{Kind: RoutingOperationQueryOne, Statement: statement, Args: args})
	if err != nil {
		return err
	}
	return delegate.QueryOne(ctx, statement, args, dest)
}

// Exec 路由并执行写语句。
func (s *RoutingSession) Exec(ctx context.Context, statement string, args NamedArgs) (Result, error) {
	delegate, _, err := s.delegate(ctx, RoutingOperation{Kind: RoutingOperationExec, Statement: statement, Args: args})
	if err != nil {
		return Result{}, err
	}
	return delegate.Exec(ctx, statement, args)
}

// Call 路由并执行存储过程调用。
func (s *RoutingSession) Call(ctx context.Context, statement string, args NamedArgs, resultSets ...any) (CallResult, error) {
	delegate, key, err := s.delegate(ctx, RoutingOperation{Kind: RoutingOperationCall, Statement: statement, Args: args})
	if err != nil {
		return CallResult{}, err
	}
	callSession, ok := delegate.(CallSession)
	if !ok {
		return CallResult{}, configurationErrorf("data source %q session does not support stored procedure call", key)
	}
	return callSession.Call(ctx, statement, args, resultSets...)
}

// QueryPage 路由并执行分页查询。
func (s *RoutingSession) QueryPage(ctx context.Context, statement string, args NamedArgs, page PageRequest, dest any) (PageQueryResult, error) {
	delegate, key, err := s.delegate(ctx, RoutingOperation{Kind: RoutingOperationPage, Statement: statement, Args: args, Page: page})
	if err != nil {
		return PageQueryResult{}, err
	}
	paged, ok := delegate.(PageQuerySession)
	if !ok {
		page = page.normalized()
		if page.SearchCount {
			return PageQueryResult{}, fmt.Errorf("goark-orm: data source %q session does not support page query", key)
		}
		if err := delegate.Query(WithPageRequest(ctx, page), statement, args, dest); err != nil {
			return PageQueryResult{}, err
		}
		return PageQueryResult{Size: page.Size, Current: page.Current}, nil
	}
	return paged.QueryPage(ctx, statement, args, page, dest)
}

// Close 关闭所有实现 Close 方法的委托 Session。
func (s *RoutingSession) Close() error {
	if s == nil {
		return configurationErrorf("routing session is nil")
	}
	var joined error
	for _, session := range s.sessions {
		if closer, ok := session.(interface{ Close() error }); ok {
			joined = errors.Join(joined, closer.Close())
		}
	}
	return joined
}

func (s *RoutingSession) delegate(ctx context.Context, operation RoutingOperation) (Session, DataSourceKey, error) {
	if s == nil {
		return nil, "", configurationErrorf("routing session is nil")
	}
	operation.Command = s.statementCommand(operation.Statement)
	key, err := s.resolve(ctx, operation)
	if err != nil {
		return nil, "", err
	}
	session, ok := s.sessions[key]
	if !ok || session == nil {
		return nil, "", configurationErrorf("routing data source %q is not registered", key)
	}
	return session, key, nil
}

func (s *RoutingSession) resolve(ctx context.Context, operation RoutingOperation) (DataSourceKey, error) {
	if key, ok := DataSourceFromContext(ctx); ok {
		return key, nil
	}
	key := ""
	if s.resolver != nil {
		resolved, err := s.resolver(ctx, operation)
		if err != nil {
			return "", err
		}
		key = string(normalizeDataSourceKey(resolved))
	}
	if key == "" {
		key = string(s.defaultKey)
	}
	return DataSourceKey(key), nil
}

func (s *RoutingSession) statementCommand(statement string) StatementCommand {
	if s == nil || s.registry == nil {
		return ""
	}
	meta, ok := s.registry.Statement(statement)
	if !ok {
		return ""
	}
	return meta.Command
}

// SessionOpenFunc 打开一个 ORM Session。
type SessionOpenFunc func() (Session, error)

// OpenSession 执行函数式 Session 打开器。
func (f SessionOpenFunc) OpenSession() (Session, error) {
	if f == nil {
		return nil, configurationErrorf("session opener is nil")
	}
	return f()
}

// RoutingSessionFactory 创建多数据源路由 Session。
type RoutingSessionFactory struct {
	openers  map[DataSourceKey]SessionOpenFunc
	resolver DataSourceResolver
	options  []RoutingSessionOption
}

// NewRoutingSessionFactory 创建多数据源路由 SessionFactory。
func NewRoutingSessionFactory(openers map[DataSourceKey]SessionOpenFunc, resolver DataSourceResolver, options ...RoutingSessionOption) (*RoutingSessionFactory, error) {
	copied := make(map[DataSourceKey]SessionOpenFunc, len(openers))
	for key, opener := range openers {
		key = normalizeDataSourceKey(key)
		if key == "" {
			return nil, configurationErrorf("routing data source key is empty")
		}
		if opener == nil {
			return nil, configurationErrorf("routing data source %q opener is nil", key)
		}
		copied[key] = opener
	}
	if len(copied) == 0 {
		return nil, configurationErrorf("routing data source openers are empty")
	}
	return &RoutingSessionFactory{
		openers:  copied,
		resolver: resolver,
		options:  append([]RoutingSessionOption(nil), options...),
	}, nil
}

// OpenSession 打开所有委托 Session，并返回路由 Session。
func (f *RoutingSessionFactory) OpenSession() (*RoutingSession, error) {
	if f == nil {
		return nil, configurationErrorf("routing session factory is nil")
	}
	sessions := make(map[DataSourceKey]Session, len(f.openers))
	for _, key := range sortedDataSourceKeys(f.openers) {
		opener := f.openers[key]
		session, err := opener.OpenSession()
		if err != nil {
			return nil, errors.Join(err, closeRoutingSessions(sessions))
		}
		if session == nil {
			return nil, errors.Join(configurationErrorf("routing data source %q session is nil", key), closeRoutingSessions(sessions))
		}
		sessions[key] = session
	}
	session, err := NewRoutingSession(sessions, f.resolver, f.options...)
	if err != nil {
		return nil, errors.Join(err, closeRoutingSessions(sessions))
	}
	return session, nil
}

func normalizeRoutingSessions(sessions map[DataSourceKey]Session) (map[DataSourceKey]Session, DataSourceKey, error) {
	if len(sessions) == 0 {
		return nil, "", configurationErrorf("routing sessions are empty")
	}
	copied := make(map[DataSourceKey]Session, len(sessions))
	defaultKey := DataSourceKey("")
	for key, session := range sessions {
		key = normalizeDataSourceKey(key)
		if key == "" {
			return nil, "", configurationErrorf("routing data source key is empty")
		}
		if session == nil {
			return nil, "", configurationErrorf("routing data source %q session is nil", key)
		}
		if defaultKey == "" || key == DefaultDataSourceKey {
			defaultKey = key
		}
		copied[key] = session
	}
	return copied, defaultKey, nil
}

func normalizeDataSourceKey(key DataSourceKey) DataSourceKey {
	return DataSourceKey(strings.TrimSpace(string(key)))
}

func closeRoutingSessions(sessions map[DataSourceKey]Session) error {
	var joined error
	for _, session := range sessions {
		if closer, ok := session.(interface{ Close() error }); ok {
			joined = errors.Join(joined, closer.Close())
		}
	}
	return joined
}

func sortedDataSourceKeys(openers map[DataSourceKey]SessionOpenFunc) []DataSourceKey {
	keys := make([]DataSourceKey, 0, len(openers))
	for key := range openers {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left int, right int) bool {
		return keys[left] < keys[right]
	})
	return keys
}
