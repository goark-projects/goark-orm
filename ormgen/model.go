package ormgen

import "goark.dev/orm"

// GenerateSpec 描述 ORM 代码生成输入。
type GenerateSpec struct {
	Dir          string
	PackageName  string
	DatabaseID   string
	TypeHandlers []string
	BuildTags    []string
	Naming       NamingConfig
}

// PackageModel 是扫描后的包级 ORM 模型。
type PackageModel struct {
	Dir         string
	PackageName string
	Entities    []EntityModel
	Mappers     []MapperModel
}

// EntityModel 描述一个实体类型。
type EntityModel struct {
	TypeName      string
	Table         string
	KeySequence   string
	Columns       []ColumnModel
	DeclareStruct bool
}

// ColumnModel 描述实体字段映射。
type ColumnModel struct {
	FieldName        string
	FieldType        string
	ColumnName       string
	KeyColumn        string
	UpdateExpression string
	PrimaryKey       bool
	AutoIncrement    bool
	IDType           orm.IDType
	Nullable         *bool
	Size             *int
	NumericScale     *int
	DBType           string
	DefaultValue     string
	TypeHandler      string
	Condition        string
	SelectDisabled   bool
	InsertStrategy   orm.FieldStrategy
	UpdateStrategy   orm.FieldStrategy
	WhereStrategy    orm.FieldStrategy
	OrderBy          bool
	OrderDesc        bool
	OrderPriority    int
	Version          bool
	SoftDelete       bool
	CreatedAt        bool
	UpdatedAt        bool
	Fill             orm.FieldFill
	Transient        bool
}

// MapperModel 描述 Mapper 接口和语句。
type MapperModel struct {
	TypeName     string
	Namespace    string
	XML          string
	Cache        orm.CacheMeta
	ImplTypeName string
	Methods      []MethodModel
	ResultMaps   []orm.ResultMapMeta
	Statements   []StatementModel
}

// MethodModel 描述 Mapper 方法。
type MethodModel struct {
	Name       string
	Params     []ParamModel
	ResultType string
	Command    orm.StatementCommand
	Statement  StatementModel
}

// ParamModel 描述 Mapper 方法参数。
type ParamModel struct {
	Name string
	Type string
}

// StatementModel 描述生成器阶段的语句元数据。
type StatementModel struct {
	ID                 string
	Namespace          string
	FullName           string
	Command            orm.StatementCommand
	StatementType      orm.StatementType
	Source             orm.StatementSource
	SQL                string
	Provider           string
	ResultMap          string
	ResultType         string
	ParameterType      string
	DatabaseID         string
	AffectData         bool
	UseGeneratedKeys   bool
	KeyProperty        string
	Options            orm.StatementOptions
	SelectKey          orm.SelectKeyMeta
	UseCache           orm.StatementCachePolicy
	FlushCache         orm.StatementCachePolicy
	Parameters         []string
	ParameterModes     []orm.ParameterMeta
	ResultSets         []orm.ResultSetMeta
	DynamicSQL         []orm.DynamicSQLNode
	InterceptorIgnores []string
}

// GeneratedPackage 表示一个已生成源码的包。
type GeneratedPackage struct {
	Dir         string
	PackageName string
	Source      []byte
}
