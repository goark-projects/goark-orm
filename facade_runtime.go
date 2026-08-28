package orm

import (
	"context"
	"database/sql"
	"io"
	"time"

	rt "goark.dev/orm/internal/runtime"
)

// 本文件提供根包公共门面，运行时实现位于 internal/runtime。
type (
	AutoMappingBehavior                = rt.AutoMappingBehavior
	AutoMappingUnknownColumnBehavior   = rt.AutoMappingUnknownColumnBehavior
	BaseMapper[T any, ID any]          = rt.BaseMapper[T, ID]
	BaseMapperOption                   = rt.BaseMapperOption
	BatchError                         = rt.BatchError
	BatchResult                        = rt.BatchResult
	BatchSession                       = rt.BatchSession
	BindingError                       = rt.BindingError
	BlockingCache                      = rt.BlockingCache
	Cache                              = rt.Cache
	CacheMeta                          = rt.CacheMeta
	CacheMissReleaser                  = rt.CacheMissReleaser
	CacheStats                         = rt.CacheStats
	CacheStatsProvider                 = rt.CacheStatsProvider
	CallResult                         = rt.CallResult
	CallSession                        = rt.CallSession
	ColumnMeta                         = rt.ColumnMeta
	CompiledSQL                        = rt.CompiledSQL
	ConfigProperties                   = rt.ConfigProperties
	Configuration                      = rt.Configuration
	ConfigurationError                 = rt.ConfigurationError
	Cursor[T any]                      = rt.Cursor[T]
	CursorQuerySession                 = rt.CursorQuerySession
	DataPermissionProvider             = rt.DataPermissionProvider
	DataSourceKey                      = rt.DataSourceKey
	DataSourceResolver                 = rt.DataSourceResolver
	DatabaseIDProvider                 = rt.DatabaseIDProvider
	DatabaseIDProviderFile             = rt.DatabaseIDProviderFile
	Db                                 = rt.Db
	DbConfig                           = rt.DbConfig
	DbType                             = rt.DbType
	DefaultIdentifierGenerator         = rt.DefaultIdentifierGenerator
	DefaultSQLInjector                 = rt.DefaultSQLInjector
	DeleteSQLBuilder                   = rt.DeleteSQLBuilder
	Dialect                            = rt.Dialect
	DialectCapabilities                = rt.DialectCapabilities
	DialectCapabilitiesProvider        = rt.DialectCapabilitiesProvider
	DialectGeneratedKeyStyle           = rt.DialectGeneratedKeyStyle
	DialectIdentifierQuoteStyle        = rt.DialectIdentifierQuoteStyle
	DialectJSONStyle                   = rt.DialectJSONStyle
	DialectPlaceholderStyle            = rt.DialectPlaceholderStyle
	DialectRowLockStyle                = rt.DialectRowLockStyle
	DialectUpsertStyle                 = rt.DialectUpsertStyle
	DynamicSQLNode                     = rt.DynamicSQLNode
	DynamicSQLNodeKind                 = rt.DynamicSQLNodeKind
	DynamicSQLRenderOptions            = rt.DynamicSQLRenderOptions
	EntityMeta                         = rt.EntityMeta
	EntitySemanticInterceptorOption    = rt.EntitySemanticInterceptorOption
	EnumValuer                         = rt.EnumValuer
	EnumValuerContext                  = rt.EnumValuerContext
	ExecutorError                      = rt.ExecutorError
	ExecutorType                       = rt.ExecutorType
	Field[T any]                       = rt.Field[T]
	FieldFill                          = rt.FieldFill
	FieldStrategy                      = rt.FieldStrategy
	GeneratedKeyPlan                   = rt.GeneratedKeyPlan
	GlobalConfig                       = rt.GlobalConfig
	IDType                             = rt.IDType
	IdentifierGenerator                = rt.IdentifierGenerator
	IllegalSQLOption                   = rt.IllegalSQLOption
	InjectNamespaceResolver            = rt.InjectNamespaceResolver
	InjectOption                       = rt.InjectOption
	InsertSQLBuilder                   = rt.InsertSQLBuilder
	Lazy[T any]                        = rt.Lazy[T]
	LazySlice[T any]                   = rt.LazySlice[T]
	LocalCacheScope                    = rt.LocalCacheScope
	LogicDeleteByIDInjector            = rt.LogicDeleteByIDInjector
	ManagedSession                     = rt.ManagedSession
	MapperMeta                         = rt.MapperMeta
	MapperRef                          = rt.MapperRef
	MappingError                       = rt.MappingError
	MemoryCache                        = rt.MemoryCache
	MemoryCacheOption                  = rt.MemoryCacheOption
	MetaObject                         = rt.MetaObject
	MetaObjectHandler                  = rt.MetaObjectHandler
	MetaObjectHandlerFuncs             = rt.MetaObjectHandlerFuncs
	MultiRowInsertSQLBuilder           = rt.MultiRowInsertSQLBuilder
	MyBatisAssembly                    = rt.MyBatisAssembly
	MyBatisAssemblyResult              = rt.MyBatisAssemblyResult
	MyBatisConfig                      = rt.MyBatisConfig
	MyBatisConfigFile                  = rt.MyBatisConfigFile
	MyBatisDbConfigFile                = rt.MyBatisDbConfigFile
	MyBatisEnvironment                 = rt.MyBatisEnvironment
	MyBatisEnvironmentFile             = rt.MyBatisEnvironmentFile
	MyBatisGlobalConfigFile            = rt.MyBatisGlobalConfigFile
	MyBatisSettings                    = rt.MyBatisSettings
	MyBatisSettingsFile                = rt.MyBatisSettingsFile
	NamedArgs                          = rt.NamedArgs
	Page[T any]                        = rt.Page[T]
	PageQueryResult                    = rt.PageQueryResult
	PageQuerySession                   = rt.PageQuerySession
	PageRequest                        = rt.PageRequest
	ParameterHandler                   = rt.ParameterHandler
	ParameterHandlerMiddleware         = rt.ParameterHandlerMiddleware
	ParameterHandlerMiddlewareFunc     = rt.ParameterHandlerMiddlewareFunc
	ParameterMeta                      = rt.ParameterMeta
	ParameterMode                      = rt.ParameterMode
	PluginRef                          = rt.PluginRef
	PluginRegistry                     = rt.PluginRegistry
	PluginRegistryBuilder              = rt.PluginRegistryBuilder
	QueryChain[T any, ID any]          = rt.QueryChain[T, ID]
	QueryWrapper[T any]                = rt.QueryWrapper[T]
	RawIdentifier                      = rt.RawIdentifier
	RawOrderBy                         = rt.RawOrderBy
	RawOrderItem                       = rt.RawOrderItem
	RawSQLToken                        = rt.RawSQLToken
	Registry                           = rt.Registry
	RegistryError                      = rt.RegistryError
	RenderedSQL                        = rt.RenderedSQL
	Result                             = rt.Result
	ResultArgMeta                      = rt.ResultArgMeta
	ResultAssociationMeta              = rt.ResultAssociationMeta
	ResultCollectionMeta               = rt.ResultCollectionMeta
	ResultConstructorMeta              = rt.ResultConstructorMeta
	ResultDiscriminatorCaseMeta        = rt.ResultDiscriminatorCaseMeta
	ResultDiscriminatorMeta            = rt.ResultDiscriminatorMeta
	ResultFieldMeta                    = rt.ResultFieldMeta
	ResultHandler[T any]               = rt.ResultHandler[T]
	ResultMapMeta                      = rt.ResultMapMeta
	ResultSetHandler                   = rt.ResultSetHandler
	ResultSetHandlerMiddleware         = rt.ResultSetHandlerMiddleware
	ResultSetHandlerMiddlewareFunc     = rt.ResultSetHandlerMiddlewareFunc
	ResultSetMeta                      = rt.ResultSetMeta
	ResultSetRows                      = rt.ResultSetRows
	ResultSetType                      = rt.ResultSetType
	RoutingOperation                   = rt.RoutingOperation
	RoutingOperationKind               = rt.RoutingOperationKind
	RoutingSession                     = rt.RoutingSession
	RoutingSessionFactory              = rt.RoutingSessionFactory
	RoutingSessionOption               = rt.RoutingSessionOption
	RowCursor                          = rt.RowCursor
	RowLockOptions                     = rt.RowLockOptions
	RowScanner                         = rt.RowScanner
	RowScannerFunc                     = rt.RowScannerFunc
	RowScannerRow                      = rt.RowScannerRow
	RowScannerTypeHandlers             = rt.RowScannerTypeHandlers
	Rows                               = rt.Rows
	RuntimeAssembly                    = rt.RuntimeAssembly
	RuntimeAssemblyResult              = rt.RuntimeAssemblyResult
	RuntimeConfig                      = rt.RuntimeConfig
	RuntimeConfigFile                  = rt.RuntimeConfigFile
	RuntimeEnvironment                 = rt.RuntimeEnvironment
	RuntimeEnvironmentFile             = rt.RuntimeEnvironmentFile
	RuntimeSettings                    = rt.RuntimeSettings
	RuntimeSettingsFile                = rt.RuntimeSettingsFile
	SQLCondition                       = rt.SQLCondition
	SQLExecutor                        = rt.SQLExecutor
	SQLFetchSizeApplier                = rt.SQLFetchSizeApplier
	SQLGuardRule                       = rt.SQLGuardRule
	SQLGuardRuleFunc                   = rt.SQLGuardRuleFunc
	SQLInjector                        = rt.SQLInjector
	SQLInjectorFunc                    = rt.SQLInjectorFunc
	SQLObservation                     = rt.SQLObservation
	SQLPreparer                        = rt.SQLPreparer
	SQLProvider                        = rt.SQLProvider
	SQLProviderDescriptor              = rt.SQLProviderDescriptor
	SQLProviderOption                  = rt.SQLProviderOption
	SQLSession                         = rt.SQLSession
	SQLSessionFactory                  = rt.SQLSessionFactory
	SQLSessionOption                   = rt.SQLSessionOption
	SQLSource                          = rt.SQLSource
	SQLStatementOptionsApplier         = rt.SQLStatementOptionsApplier
	SQLTransactionFactory              = rt.SQLTransactionFactory
	SelectKeyMeta                      = rt.SelectKeyMeta
	SelectKeyOrder                     = rt.SelectKeyOrder
	SelectSQLBuilder                   = rt.SelectSQLBuilder
	Service[T any, ID any]             = rt.Service[T, ID]
	Session                            = rt.Session
	SessionOpenFunc                    = rt.SessionOpenFunc
	StatementCachePolicy               = rt.StatementCachePolicy
	StatementCallSession               = rt.StatementCallSession
	StatementCommand                   = rt.StatementCommand
	StatementExecutor                  = rt.StatementExecutor
	StatementExecutorMiddleware        = rt.StatementExecutorMiddleware
	StatementExecutorMiddlewareFunc    = rt.StatementExecutorMiddlewareFunc
	StatementHandler                   = rt.StatementHandler
	StatementHandlerMiddleware         = rt.StatementHandlerMiddleware
	StatementHandlerMiddlewareFunc     = rt.StatementHandlerMiddlewareFunc
	StatementInterceptor               = rt.StatementInterceptor
	StatementInterceptorFunc           = rt.StatementInterceptorFunc
	StatementInvocation                = rt.StatementInvocation
	StatementMeta                      = rt.StatementMeta
	StatementNotFoundError             = rt.StatementNotFoundError
	StatementOptions                   = rt.StatementOptions
	StatementRuntime                   = rt.StatementRuntime
	StatementSession                   = rt.StatementSession
	StatementSource                    = rt.StatementSource
	StatementType                      = rt.StatementType
	TooManyResultsError                = rt.TooManyResultsError
	Transaction                        = rt.Transaction
	TransactionFactory                 = rt.TransactionFactory
	TransactionSource                  = rt.TransactionSource
	TxSession                          = rt.TxSession
	TypeAlias                          = rt.TypeAlias
	TypeHandler                        = rt.TypeHandler
	TypeHandlerAdapter                 = rt.TypeHandlerAdapter
	TypeHandlerRef                     = rt.TypeHandlerRef
	TypeHandlerRowScanner              = rt.TypeHandlerRowScanner
	TypeHandlerRowScannerFunc          = rt.TypeHandlerRowScannerFunc
	TypedConditionTarget[T any, W any] = rt.TypedConditionTarget[T, W]
	TypedField[T any, V any]           = rt.TypedField[T, V]
	TypedFieldRef[T any]               = rt.TypedFieldRef[T]
	UpdateChain[T any, ID any]         = rt.UpdateChain[T, ID]
	UpdateSQLBuilder                   = rt.UpdateSQLBuilder
	UpdateWrapper[T any]               = rt.UpdateWrapper[T]
	UpsertSpec                         = rt.UpsertSpec
)

const (
	AutoMappingBehaviorFull                 = rt.AutoMappingBehaviorFull
	AutoMappingBehaviorNone                 = rt.AutoMappingBehaviorNone
	AutoMappingBehaviorPartial              = rt.AutoMappingBehaviorPartial
	AutoMappingUnknownColumnBehaviorFailing = rt.AutoMappingUnknownColumnBehaviorFailing
	AutoMappingUnknownColumnBehaviorNone    = rt.AutoMappingUnknownColumnBehaviorNone
	AutoMappingUnknownColumnBehaviorWarning = rt.AutoMappingUnknownColumnBehaviorWarning
	DatabaseIDProviderVendor                = rt.DatabaseIDProviderVendor
	DbTypeMariaDB                           = rt.DbTypeMariaDB
	DbTypeMySQL                             = rt.DbTypeMySQL
	DbTypeOracle                            = rt.DbTypeOracle
	DbTypePostgres                          = rt.DbTypePostgres
	DbTypeQuestion                          = rt.DbTypeQuestion
	DbTypeSQLServer                         = rt.DbTypeSQLServer
	DbTypeSQLite                            = rt.DbTypeSQLite
	DefaultBatchSize                        = rt.DefaultBatchSize
	DefaultDataSourceKey                    = rt.DefaultDataSourceKey
	DefaultPreparedStatementCacheSize       = rt.DefaultPreparedStatementCacheSize
	DialectGeneratedKeyLastInsertID         = rt.DialectGeneratedKeyLastInsertID
	DialectGeneratedKeyNone                 = rt.DialectGeneratedKeyNone
	DialectGeneratedKeyOutput               = rt.DialectGeneratedKeyOutput
	DialectGeneratedKeyReturning            = rt.DialectGeneratedKeyReturning
	DialectGeneratedKeyReturningInto        = rt.DialectGeneratedKeyReturningInto
	DialectIdentifierQuoteBacktick          = rt.DialectIdentifierQuoteBacktick
	DialectIdentifierQuoteBracket           = rt.DialectIdentifierQuoteBracket
	DialectIdentifierQuoteDouble            = rt.DialectIdentifierQuoteDouble
	DialectJSONExtension                    = rt.DialectJSONExtension
	DialectJSONNative                       = rt.DialectJSONNative
	DialectJSONNone                         = rt.DialectJSONNone
	DialectPlaceholderAtPNumber             = rt.DialectPlaceholderAtPNumber
	DialectPlaceholderColonNumber           = rt.DialectPlaceholderColonNumber
	DialectPlaceholderDollarNumber          = rt.DialectPlaceholderDollarNumber
	DialectPlaceholderQuestion              = rt.DialectPlaceholderQuestion
	DialectRowLockForUpdate                 = rt.DialectRowLockForUpdate
	DialectRowLockHints                     = rt.DialectRowLockHints
	DialectRowLockNone                      = rt.DialectRowLockNone
	DialectUpsertMerge                      = rt.DialectUpsertMerge
	DialectUpsertNone                       = rt.DialectUpsertNone
	DialectUpsertOnConflict                 = rt.DialectUpsertOnConflict
	DialectUpsertOnDuplicateKey             = rt.DialectUpsertOnDuplicateKey
	DynamicSQLNodeBind                      = rt.DynamicSQLNodeBind
	DynamicSQLNodeChoose                    = rt.DynamicSQLNodeChoose
	DynamicSQLNodeForeach                   = rt.DynamicSQLNodeForeach
	DynamicSQLNodeIf                        = rt.DynamicSQLNodeIf
	DynamicSQLNodeInclude                   = rt.DynamicSQLNodeInclude
	DynamicSQLNodeOtherwise                 = rt.DynamicSQLNodeOtherwise
	DynamicSQLNodeSet                       = rt.DynamicSQLNodeSet
	DynamicSQLNodeText                      = rt.DynamicSQLNodeText
	DynamicSQLNodeTrim                      = rt.DynamicSQLNodeTrim
	DynamicSQLNodeWhen                      = rt.DynamicSQLNodeWhen
	DynamicSQLNodeWhere                     = rt.DynamicSQLNodeWhere
	ExecutorTypeBatch                       = rt.ExecutorTypeBatch
	ExecutorTypeReuse                       = rt.ExecutorTypeReuse
	ExecutorTypeSimple                      = rt.ExecutorTypeSimple
	FieldFillDefault                        = rt.FieldFillDefault
	FieldFillInsert                         = rt.FieldFillInsert
	FieldFillInsertUpdate                   = rt.FieldFillInsertUpdate
	FieldFillUpdate                         = rt.FieldFillUpdate
	FieldStrategyAlways                     = rt.FieldStrategyAlways
	FieldStrategyDefault                    = rt.FieldStrategyDefault
	FieldStrategyNever                      = rt.FieldStrategyNever
	FieldStrategyNotEmpty                   = rt.FieldStrategyNotEmpty
	FieldStrategyNotNull                    = rt.FieldStrategyNotNull
	FieldStrategyNotZero                    = rt.FieldStrategyNotZero
	IDTypeAssignID                          = rt.IDTypeAssignID
	IDTypeAssignUUID                        = rt.IDTypeAssignUUID
	IDTypeAuto                              = rt.IDTypeAuto
	IDTypeInput                             = rt.IDTypeInput
	IDTypeNone                              = rt.IDTypeNone
	InterceptorNameAll                      = rt.InterceptorNameAll
	InterceptorNameBlockAttack              = rt.InterceptorNameBlockAttack
	InterceptorNameDataPermission           = rt.InterceptorNameDataPermission
	InterceptorNameDynamicTable             = rt.InterceptorNameDynamicTable
	InterceptorNameEntitySemantic           = rt.InterceptorNameEntitySemantic
	InterceptorNameIllegalSQL               = rt.InterceptorNameIllegalSQL
	InterceptorNamePagination               = rt.InterceptorNamePagination
	InterceptorNameReadOnly                 = rt.InterceptorNameReadOnly
	InterceptorNameSQLGuard                 = rt.InterceptorNameSQLGuard
	InterceptorNameSQLObserver              = rt.InterceptorNameSQLObserver
	InterceptorNameTenant                   = rt.InterceptorNameTenant
	LocalCacheScopeSession                  = rt.LocalCacheScopeSession
	LocalCacheScopeStatement                = rt.LocalCacheScopeStatement
	ParameterModeIn                         = rt.ParameterModeIn
	ParameterModeInOut                      = rt.ParameterModeInOut
	ParameterModeOut                        = rt.ParameterModeOut
	ResultSetTypeForwardOnly                = rt.ResultSetTypeForwardOnly
	ResultSetTypeScrollInsensitive          = rt.ResultSetTypeScrollInsensitive
	ResultSetTypeScrollSensitive            = rt.ResultSetTypeScrollSensitive
	RoutingOperationCall                    = rt.RoutingOperationCall
	RoutingOperationExec                    = rt.RoutingOperationExec
	RoutingOperationPage                    = rt.RoutingOperationPage
	RoutingOperationQuery                   = rt.RoutingOperationQuery
	RoutingOperationQueryOne                = rt.RoutingOperationQueryOne
	SelectKeyOrderAfter                     = rt.SelectKeyOrderAfter
	SelectKeyOrderBefore                    = rt.SelectKeyOrderBefore
	StatementCacheDefault                   = rt.StatementCacheDefault
	StatementCacheDisabled                  = rt.StatementCacheDisabled
	StatementCacheEnabled                   = rt.StatementCacheEnabled
	StatementCommandCall                    = rt.StatementCommandCall
	StatementCommandDelete                  = rt.StatementCommandDelete
	StatementCommandInsert                  = rt.StatementCommandInsert
	StatementCommandSelect                  = rt.StatementCommandSelect
	StatementCommandUpdate                  = rt.StatementCommandUpdate
	StatementSourceAnnotation               = rt.StatementSourceAnnotation
	StatementSourceBase                     = rt.StatementSourceBase
	StatementSourceXML                      = rt.StatementSourceXML
	StatementTypeCallable                   = rt.StatementTypeCallable
	StatementTypePrepared                   = rt.StatementTypePrepared
)

var (
	ErrBinding              = rt.ErrBinding
	ErrConfiguration        = rt.ErrConfiguration
	ErrExecutor             = rt.ErrExecutor
	ErrMapping              = rt.ErrMapping
	ErrORM                  = rt.ErrORM
	ErrRegistry             = rt.ErrRegistry
	ErrStatementNotFound    = rt.ErrStatementNotFound
	ErrTooManyResults       = rt.ErrTooManyResults
	ErrTransactionCompleted = rt.ErrTransactionCompleted
)

func AssembleMyBatisConfig(assembly MyBatisAssembly) (MyBatisAssemblyResult, error) {
	return rt.AssembleMyBatisConfig(assembly)
}

func AssembleRuntimeConfig(assembly RuntimeAssembly) (RuntimeAssemblyResult, error) {
	return rt.AssembleRuntimeConfig(assembly)
}

func BetweenTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], left V, right V) W {
	return rt.BetweenTypedValues[T, V, W](w, field, left, right)
}

func BetweenTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], left V, right V) W {
	return rt.BetweenTypedValuesIf[T, V, W](condition, w, field, left, right)
}

func BuildUpsertSQL(dialect Dialect, spec UpsertSpec) (SQLSource, error) {
	return rt.BuildUpsertSQL(dialect, spec)
}

func Call(ctx context.Context, session Session, statement string, args NamedArgs, resultSets ...any) (CallResult, error) {
	return rt.Call(ctx, session, statement, args, resultSets...)
}

func CompileSQL(query string, args NamedArgs, dialect Dialect) (CompiledSQL, error) {
	return rt.CompileSQL(query, args, dialect)
}

func CompileSQLContext(ctx context.Context, query string, args NamedArgs, dialect Dialect) (CompiledSQL, error) {
	return rt.CompileSQLContext(ctx, query, args, dialect)
}

func DataSourceFromContext(ctx context.Context) (DataSourceKey, bool) {
	return rt.DataSourceFromContext(ctx)
}

func DecodeMyBatisConfig(reader io.Reader) (MyBatisConfig, error) {
	return rt.DecodeMyBatisConfig(reader)
}

func DecodeRuntimeConfig(reader io.Reader) (RuntimeConfig, error) {
	return rt.DecodeRuntimeConfig(reader)
}

func DefaultConfiguration() Configuration {
	return rt.DefaultConfiguration()
}

func DefaultDbConfig() DbConfig {
	return rt.DefaultDbConfig()
}

func DefaultGlobalConfig() GlobalConfig {
	return rt.DefaultGlobalConfig()
}

func DefaultMyBatisConfig() MyBatisConfig {
	return rt.DefaultMyBatisConfig()
}

func DefaultRuntimeConfig() RuntimeConfig {
	return rt.DefaultRuntimeConfig()
}

func DialectCapabilitiesOf(dialect Dialect) DialectCapabilities {
	return rt.DialectCapabilitiesOf(dialect)
}

func EqTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.EqTypedValue[T, V, W](w, field, value)
}

func EqTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.EqTypedValueIf[T, V, W](condition, w, field, value)
}

func GeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.GeTypedValue[T, V, W](w, field, value)
}

func GeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.GeTypedValueIf[T, V, W](condition, w, field, value)
}

func GetFieldValue[T any, ID any, V any](ctx context.Context, service *Service[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	return rt.GetFieldValue[T, ID, V](ctx, service, field, wrapper)
}

func GetFirstFieldValue[T any, ID any, V any](ctx context.Context, service *Service[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	return rt.GetFirstFieldValue[T, ID, V](ctx, service, field, wrapper)
}

func GtTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.GtTypedValue[T, V, W](w, field, value)
}

func GtTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.GtTypedValueIf[T, V, W](condition, w, field, value)
}

func InTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], values ...V) W {
	return rt.InTypedValues[T, V, W](w, field, values...)
}

func InTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], values ...V) W {
	return rt.InTypedValuesIf[T, V, W](condition, w, field, values...)
}

func LeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.LeTypedValue[T, V, W](w, field, value)
}

func LeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.LeTypedValueIf[T, V, W](condition, w, field, value)
}

func LikeLeftTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.LikeLeftTypedValue[T, V, W](w, field, value)
}

func LikeLeftTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.LikeLeftTypedValueIf[T, V, W](condition, w, field, value)
}

func LikeRightTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.LikeRightTypedValue[T, V, W](w, field, value)
}

func LikeRightTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.LikeRightTypedValueIf[T, V, W](condition, w, field, value)
}

func LikeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.LikeTypedValue[T, V, W](w, field, value)
}

func LikeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.LikeTypedValueIf[T, V, W](condition, w, field, value)
}

func ListFieldValues[T any, ID any, V any](ctx context.Context, service *Service[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) ([]V, error) {
	return rt.ListFieldValues[T, ID, V](ctx, service, field, wrapper)
}

func LoadAndAssembleMyBatisConfig(path string, assembly MyBatisAssembly) (MyBatisAssemblyResult, error) {
	return rt.LoadAndAssembleMyBatisConfig(path, assembly)
}

func LoadAndAssembleRuntimeConfig(path string, assembly RuntimeAssembly) (RuntimeAssemblyResult, error) {
	return rt.LoadAndAssembleRuntimeConfig(path, assembly)
}

func LoadMyBatisConfig(path string) (MyBatisConfig, error) {
	return rt.LoadMyBatisConfig(path)
}

func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	return rt.LoadRuntimeConfig(path)
}

func LtTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.LtTypedValue[T, V, W](w, field, value)
}

func LtTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.LtTypedValueIf[T, V, W](condition, w, field, value)
}

func NeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.NeTypedValue[T, V, W](w, field, value)
}

func NeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.NeTypedValueIf[T, V, W](condition, w, field, value)
}

func NewBaseMapper[T any, ID any](session StatementSession, entity EntityMeta, options ...BaseMapperOption) (*BaseMapper[T, ID], error) {
	return rt.NewBaseMapper[T, ID](session, entity, options...)
}

func NewBatchSession(session Session) (*BatchSession, error) {
	return rt.NewBatchSession(session)
}

func NewBlockAttackInterceptor() StatementInterceptor {
	return rt.NewBlockAttackInterceptor()
}

func NewBlockingCache(delegate Cache) *BlockingCache {
	return rt.NewBlockingCache(delegate)
}

func NewBoolTypeHandler() TypeHandler {
	return rt.NewBoolTypeHandler()
}

func NewBytesTypeHandler() TypeHandler {
	return rt.NewBytesTypeHandler()
}

func NewDataPermissionInterceptor(provider DataPermissionProvider) StatementInterceptor {
	return rt.NewDataPermissionInterceptor(provider)
}

func NewDb(session Session) (Db, error) {
	return rt.NewDb(session)
}

func NewDecimalTypeHandler() TypeHandler {
	return rt.NewDecimalTypeHandler()
}

func NewDefaultIdentifierGenerator() *DefaultIdentifierGenerator {
	return rt.NewDefaultIdentifierGenerator()
}

func NewDeleteSQLBuilder() *DeleteSQLBuilder {
	return rt.NewDeleteSQLBuilder()
}

func NewDialect(dbType DbType) (Dialect, error) {
	return rt.NewDialect(dbType)
}

func NewDialectCapabilities(dbType DbType) (DialectCapabilities, error) {
	return rt.NewDialectCapabilities(dbType)
}

func NewDynamicTableInterceptor(tables map[string]string) StatementInterceptor {
	return rt.NewDynamicTableInterceptor(tables)
}

func NewEntitySemanticInterceptor(registry *Registry, options ...EntitySemanticInterceptorOption) StatementInterceptor {
	return rt.NewEntitySemanticInterceptor(registry, options...)
}

func NewField[T any](column string) Field[T] {
	return rt.NewField[T](column)
}

func NewGeneratedKeyPlan(dialect Dialect, keyColumn string) (GeneratedKeyPlan, error) {
	return rt.NewGeneratedKeyPlan(dialect, keyColumn)
}

func NewIllegalSQLInterceptor(options ...IllegalSQLOption) StatementInterceptor {
	return rt.NewIllegalSQLInterceptor(options...)
}

func NewInsertSQLBuilder() *InsertSQLBuilder {
	return rt.NewInsertSQLBuilder()
}

func NewJSONTypeHandler() TypeHandler {
	return rt.NewJSONTypeHandler()
}

func NewLazy[T any](loader func(context.Context) (T, error)) Lazy[T] {
	return rt.NewLazy[T](loader)
}

func NewLazySlice[T any](loader func(context.Context) ([]T, error)) LazySlice[T] {
	return rt.NewLazySlice[T](loader)
}

func NewMariaDBDialect() Dialect {
	return rt.NewMariaDBDialect()
}

func NewMemoryCache(id string, options ...MemoryCacheOption) *MemoryCache {
	return rt.NewMemoryCache(id, options...)
}

func NewMetaObject(entity EntityMeta, target any) (*MetaObject, error) {
	return rt.NewMetaObject(entity, target)
}

func NewMultiRowInsertSQLBuilder() *MultiRowInsertSQLBuilder {
	return rt.NewMultiRowInsertSQLBuilder()
}

func NewMySQLDialect() Dialect {
	return rt.NewMySQLDialect()
}

func NewOracleDialect() Dialect {
	return rt.NewOracleDialect()
}

func NewPageRequest(current int64, size int64) PageRequest {
	return rt.NewPageRequest(current, size)
}

func NewPaginationInterceptor() StatementInterceptor {
	return rt.NewPaginationInterceptor()
}

func NewPluginRegistryBuilder() *PluginRegistryBuilder {
	return rt.NewPluginRegistryBuilder()
}

func NewPostgresDialect() Dialect {
	return rt.NewPostgresDialect()
}

func NewQueryWrapper[T any]() *QueryWrapper[T] {
	return rt.NewQueryWrapper[T]()
}

func NewQuestionDialect() Dialect {
	return rt.NewQuestionDialect()
}

func NewRawIdentifier(name string) (RawIdentifier, error) {
	return rt.NewRawIdentifier(name)
}

func NewRawOrderBy(items ...RawOrderItem) RawOrderBy {
	return rt.NewRawOrderBy(items...)
}

func NewRawOrderItem(column string, desc bool) (RawOrderItem, error) {
	return rt.NewRawOrderItem(column, desc)
}

func NewReadOnlyInterceptor() StatementInterceptor {
	return rt.NewReadOnlyInterceptor()
}

func NewRegistry() *Registry {
	return rt.NewRegistry()
}

func NewRoutingSession(sessions map[DataSourceKey]Session, resolver DataSourceResolver, options ...RoutingSessionOption) (*RoutingSession, error) {
	return rt.NewRoutingSession(sessions, resolver, options...)
}

func NewRoutingSessionFactory(openers map[DataSourceKey]SessionOpenFunc, resolver DataSourceResolver, options ...RoutingSessionOption) (*RoutingSessionFactory, error) {
	return rt.NewRoutingSessionFactory(openers, resolver, options...)
}

func NewSQLGuardInterceptor(rules ...SQLGuardRule) StatementInterceptor {
	return rt.NewSQLGuardInterceptor(rules...)
}

func NewSQLObserverInterceptor(observe func(context.Context, SQLObservation) error) StatementInterceptor {
	return rt.NewSQLObserverInterceptor(observe)
}

func NewSQLProviderDescriptor(name string, provider SQLProvider, options ...SQLProviderOption) SQLProviderDescriptor {
	return rt.NewSQLProviderDescriptor(name, provider, options...)
}

func NewSQLServerDialect() Dialect {
	return rt.NewSQLServerDialect()
}

func NewSQLSession(registry *Registry, executor SQLExecutor, dialect Dialect, options ...SQLSessionOption) (*SQLSession, error) {
	return rt.NewSQLSession(registry, executor, dialect, options...)
}

func NewSQLSessionFactory(registry *Registry, db *sql.DB, dialect Dialect, options ...SQLSessionOption) (*SQLSessionFactory, error) {
	return rt.NewSQLSessionFactory(registry, db, dialect, options...)
}

func NewSQLTransactionFactory() SQLTransactionFactory {
	return rt.NewSQLTransactionFactory()
}

func NewSQLiteDialect() Dialect {
	return rt.NewSQLiteDialect()
}

func NewSelectSQLBuilder() *SelectSQLBuilder {
	return rt.NewSelectSQLBuilder()
}

func NewService[T any, ID any](mapper *BaseMapper[T, ID]) (*Service[T, ID], error) {
	return rt.NewService[T, ID](mapper)
}

func NewStringTypeHandler() TypeHandler {
	return rt.NewStringTypeHandler()
}

func NewTenantInterceptor(column string, value any) StatementInterceptor {
	return rt.NewTenantInterceptor(column, value)
}

func NewTimeTypeHandler() TypeHandler {
	return rt.NewTimeTypeHandler()
}

func NewTypeHandler(toDB func(context.Context, any) (any, error), fromDB func(context.Context, any, any) error) TypeHandler {
	return rt.NewTypeHandler(toDB, fromDB)
}

func NewTypedField[T any, V any](column string) TypedField[T, V] {
	return rt.NewTypedField[T, V](column)
}

func NewUpdateSQLBuilder() *UpdateSQLBuilder {
	return rt.NewUpdateSQLBuilder()
}

func NewUpdateWrapper[T any]() *UpdateWrapper[T] {
	return rt.NewUpdateWrapper[T]()
}

func NormalizeParameterMode(mode ParameterMode) ParameterMode {
	return rt.NormalizeParameterMode(mode)
}

func NotBetweenTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], left V, right V) W {
	return rt.NotBetweenTypedValues[T, V, W](w, field, left, right)
}

func NotBetweenTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], left V, right V) W {
	return rt.NotBetweenTypedValuesIf[T, V, W](condition, w, field, left, right)
}

func NotInTypedValues[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], values ...V) W {
	return rt.NotInTypedValues[T, V, W](w, field, values...)
}

func NotInTypedValuesIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], values ...V) W {
	return rt.NotInTypedValuesIf[T, V, W](condition, w, field, values...)
}

func NotLikeTypedValue[T any, V any, W TypedConditionTarget[T, W]](w W, field TypedField[T, V], value V) W {
	return rt.NotLikeTypedValue[T, V, W](w, field, value)
}

func NotLikeTypedValueIf[T any, V any, W TypedConditionTarget[T, W]](condition bool, w W, field TypedField[T, V], value V) W {
	return rt.NotLikeTypedValueIf[T, V, W](condition, w, field, value)
}

func PageRequestFromContext(ctx context.Context) (PageRequest, bool) {
	return rt.PageRequestFromContext(ctx)
}

func ParseAutoMappingBehavior(value string) (AutoMappingBehavior, error) {
	return rt.ParseAutoMappingBehavior(value)
}

func ParseAutoMappingUnknownColumnBehavior(value string) (AutoMappingUnknownColumnBehavior, error) {
	return rt.ParseAutoMappingUnknownColumnBehavior(value)
}

func ParseDbType(value string) (DbType, error) {
	return rt.ParseDbType(value)
}

func ParseExecutorType(value string) (ExecutorType, error) {
	return rt.ParseExecutorType(value)
}

func ParseFieldFill(value string) (FieldFill, error) {
	return rt.ParseFieldFill(value)
}

func ParseFieldStrategy(value string) (FieldStrategy, error) {
	return rt.ParseFieldStrategy(value)
}

func ParseIDType(value string) (IDType, error) {
	return rt.ParseIDType(value)
}

func ParseLocalCacheScope(value string) (LocalCacheScope, error) {
	return rt.ParseLocalCacheScope(value)
}

func ParseParameterMode(value string) (ParameterMode, error) {
	return rt.ParseParameterMode(value)
}

func ParseResultSetType(value string) (ResultSetType, error) {
	return rt.ParseResultSetType(value)
}

func QueryCursor[T any](ctx context.Context, session Session, statement string, args NamedArgs) (*Cursor[T], error) {
	return rt.QueryCursor[T](ctx, session, statement, args)
}

func QueryEach[T any](ctx context.Context, session Session, statement string, args NamedArgs, handler ResultHandler[T]) error {
	return rt.QueryEach[T](ctx, session, statement, args, handler)
}

func QueryPage[T any](ctx context.Context, session Session, statement string, args NamedArgs, page PageRequest) (Page[T], error) {
	return rt.QueryPage[T](ctx, session, statement, args, page)
}

func ReadWriteDataSourceResolver(read DataSourceKey, write DataSourceKey) DataSourceResolver {
	return rt.ReadWriteDataSourceResolver(read, write)
}

func RegisterDefaultInjectedStatementsForRegistry(registry *Registry, namespaceResolver InjectNamespaceResolver, options ...InjectOption) error {
	return rt.RegisterDefaultInjectedStatementsForRegistry(registry, namespaceResolver, options...)
}

func RegisterInjectedStatements(registry *Registry, namespace string, entity EntityMeta, injector SQLInjector, options ...InjectOption) error {
	return rt.RegisterInjectedStatements(registry, namespace, entity, injector, options...)
}

func RenderDynamicSQL(nodes []DynamicSQLNode, args NamedArgs) (RenderedSQL, error) {
	return rt.RenderDynamicSQL(nodes, args)
}

func RenderDynamicSQLWithOptions(nodes []DynamicSQLNode, args NamedArgs, options DynamicSQLRenderOptions) (RenderedSQL, error) {
	return rt.RenderDynamicSQLWithOptions(nodes, args, options)
}

func RowLockClause(dialect Dialect, options RowLockOptions) (string, error) {
	return rt.RowLockClause(dialect, options)
}

func SelectFieldValue[T any, ID any, V any](ctx context.Context, mapper *BaseMapper[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	return rt.SelectFieldValue[T, ID, V](ctx, mapper, field, wrapper)
}

func SelectFieldValues[T any, ID any, V any](ctx context.Context, mapper *BaseMapper[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) ([]V, error) {
	return rt.SelectFieldValues[T, ID, V](ctx, mapper, field, wrapper)
}

func SelectFirstFieldValue[T any, ID any, V any](ctx context.Context, mapper *BaseMapper[T, ID], field TypedField[T, V], wrapper *QueryWrapper[T]) (V, error) {
	return rt.SelectFirstFieldValue[T, ID, V](ctx, mapper, field, wrapper)
}

func SetDecrByTypedValue[T any, V any](w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	return rt.SetDecrByTypedValue[T, V](w, field, value)
}

func SetDecrByTypedValueIf[T any, V any](condition bool, w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	return rt.SetDecrByTypedValueIf[T, V](condition, w, field, value)
}

func SetIncrByTypedValue[T any, V any](w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	return rt.SetIncrByTypedValue[T, V](w, field, value)
}

func SetIncrByTypedValueIf[T any, V any](condition bool, w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	return rt.SetIncrByTypedValueIf[T, V](condition, w, field, value)
}

func SetTypedValue[T any, V any](w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	return rt.SetTypedValue[T, V](w, field, value)
}

func SetTypedValueIf[T any, V any](condition bool, w *UpdateWrapper[T], field TypedField[T, V], value V) *UpdateWrapper[T] {
	return rt.SetTypedValueIf[T, V](condition, w, field, value)
}

func ShrinkSQLWhitespaces(sqlText string) string {
	return rt.ShrinkSQLWhitespaces(sqlText)
}

func StatementDataSourceResolver(routes map[string]DataSourceKey, fallback DataSourceResolver) DataSourceResolver {
	return rt.StatementDataSourceResolver(routes, fallback)
}

func StatementInterceptorIgnored(statement StatementMeta, name string) bool {
	return rt.StatementInterceptorIgnored(statement, name)
}

func StaticDataSourceResolver(key DataSourceKey) DataSourceResolver {
	return rt.StaticDataSourceResolver(key)
}

func ValidateRegistry(registry *Registry) error {
	return rt.ValidateRegistry(registry)
}

func WithBaseMapperClock(clock func() time.Time) BaseMapperOption {
	return rt.WithBaseMapperClock(clock)
}

func WithBaseMapperDialect(dialect Dialect) BaseMapperOption {
	return rt.WithBaseMapperDialect(dialect)
}

func WithBaseMapperGlobalConfig(global GlobalConfig) BaseMapperOption {
	return rt.WithBaseMapperGlobalConfig(global)
}

func WithBaseMapperIdentifierGenerator(generator IdentifierGenerator) BaseMapperOption {
	return rt.WithBaseMapperIdentifierGenerator(generator)
}

func WithBaseMapperMetaObjectHandler(handler MetaObjectHandler) BaseMapperOption {
	return rt.WithBaseMapperMetaObjectHandler(handler)
}

func WithConfiguration(config Configuration) SQLSessionOption {
	return rt.WithConfiguration(config)
}

func WithDataSource(ctx context.Context, key DataSourceKey) context.Context {
	return rt.WithDataSource(ctx, key)
}

func WithEntitySemanticClock(clock func() time.Time) EntitySemanticInterceptorOption {
	return rt.WithEntitySemanticClock(clock)
}

func WithIdentifierGenerator(generator IdentifierGenerator) SQLSessionOption {
	return rt.WithIdentifierGenerator(generator)
}

func WithIllegalSQLDenyMultipleStatements(enabled bool) IllegalSQLOption {
	return rt.WithIllegalSQLDenyMultipleStatements(enabled)
}

func WithIllegalSQLDenySelectWildcard(enabled bool) IllegalSQLOption {
	return rt.WithIllegalSQLDenySelectWildcard(enabled)
}

func WithIllegalSQLDenyWriteWithoutWhere(enabled bool) IllegalSQLOption {
	return rt.WithIllegalSQLDenyWriteWithoutWhere(enabled)
}

func WithInjectDialect(dialect Dialect) InjectOption {
	return rt.WithInjectDialect(dialect)
}

func WithInjectGlobalConfig(global GlobalConfig) InjectOption {
	return rt.WithInjectGlobalConfig(global)
}

func WithInterceptors(interceptors ...StatementInterceptor) SQLSessionOption {
	return rt.WithInterceptors(interceptors...)
}

func WithLocalCache(enabled bool) SQLSessionOption {
	return rt.WithLocalCache(enabled)
}

func WithMemoryCacheMaxEntries(maxEntries int) MemoryCacheOption {
	return rt.WithMemoryCacheMaxEntries(maxEntries)
}

func WithMemoryCacheTTL(ttl time.Duration) MemoryCacheOption {
	return rt.WithMemoryCacheTTL(ttl)
}

func WithMetaObjectHandler(handler MetaObjectHandler) SQLSessionOption {
	return rt.WithMetaObjectHandler(handler)
}

func WithPageRequest(ctx context.Context, page PageRequest) context.Context {
	return rt.WithPageRequest(ctx, page)
}

func WithParameterHandler(handler ParameterHandler) SQLSessionOption {
	return rt.WithParameterHandler(handler)
}

func WithParameterHandlerMiddleware(middleware ...ParameterHandlerMiddleware) SQLSessionOption {
	return rt.WithParameterHandlerMiddleware(middleware...)
}

func WithPreparedStatementCacheSize(maxEntries int) SQLSessionOption {
	return rt.WithPreparedStatementCacheSize(maxEntries)
}

func WithResultSetHandler(handler ResultSetHandler) SQLSessionOption {
	return rt.WithResultSetHandler(handler)
}

func WithResultSetHandlerMiddleware(middleware ...ResultSetHandlerMiddleware) SQLSessionOption {
	return rt.WithResultSetHandlerMiddleware(middleware...)
}

func WithRoutingDefaultDataSource(key DataSourceKey) RoutingSessionOption {
	return rt.WithRoutingDefaultDataSource(key)
}

func WithRoutingRegistry(registry *Registry) RoutingSessionOption {
	return rt.WithRoutingRegistry(registry)
}

func WithSQLProviderCommands(commands ...StatementCommand) SQLProviderOption {
	return rt.WithSQLProviderCommands(commands...)
}

func WithSQLProviderStatements(statements ...string) SQLProviderOption {
	return rt.WithSQLProviderStatements(statements...)
}

func WithStatementExecutor(executor StatementExecutor) SQLSessionOption {
	return rt.WithStatementExecutor(executor)
}

func WithStatementExecutorMiddleware(middleware ...StatementExecutorMiddleware) SQLSessionOption {
	return rt.WithStatementExecutorMiddleware(middleware...)
}

func WithStatementHandler(handler StatementHandler) SQLSessionOption {
	return rt.WithStatementHandler(handler)
}

func WithStatementHandlerMiddleware(middleware ...StatementHandlerMiddleware) SQLSessionOption {
	return rt.WithStatementHandlerMiddleware(middleware...)
}

func WithTypeHandler(name string, handler TypeHandler) SQLSessionOption {
	return rt.WithTypeHandler(name, handler)
}

func WithTypeHandlers(handlers map[string]TypeHandler) SQLSessionOption {
	return rt.WithTypeHandlers(handlers)
}
