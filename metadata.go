package orm

// StatementCommand 表示 SQL 语句命令类型。
type StatementCommand string

const (
	// StatementCommandSelect 表示查询语句。
	StatementCommandSelect StatementCommand = "select"
	// StatementCommandInsert 表示插入语句。
	StatementCommandInsert StatementCommand = "insert"
	// StatementCommandUpdate 表示更新语句。
	StatementCommandUpdate StatementCommand = "update"
	// StatementCommandDelete 表示删除语句。
	StatementCommandDelete StatementCommand = "delete"
	// StatementCommandCall 表示存储过程或可调用语句。
	StatementCommandCall StatementCommand = "call"
)

// StatementType 表示 Statement 的底层执行形态。
type StatementType string

const (
	// StatementTypePrepared 表示普通预编译 SQL 语句。
	StatementTypePrepared StatementType = "PREPARED"
	// StatementTypeCallable 表示数据库存储过程或 callable statement。
	StatementTypeCallable StatementType = "CALLABLE"
)

// ParameterMode 表示可调用语句参数方向。
type ParameterMode string

const (
	// ParameterModeIn 表示普通输入参数。
	ParameterModeIn ParameterMode = "IN"
	// ParameterModeOut 表示仅输出参数。
	ParameterModeOut ParameterMode = "OUT"
	// ParameterModeInOut 表示既输入又输出的参数。
	ParameterModeInOut ParameterMode = "INOUT"
)

// ParameterMeta 描述可调用语句中的单个参数。
type ParameterMeta struct {
	Name        string
	Mode        ParameterMode
	JDBCType    string
	TypeHandler string
}

// ResultSetMeta 描述存储过程返回的一个结果集。
type ResultSetMeta struct {
	Name       string
	ResultMap  string
	ResultType string
}

// StatementSource 表示 SQL 语句来源。
type StatementSource string

const (
	// StatementSourceXML 表示语句来源于 Mapper XML。
	StatementSourceXML StatementSource = "xml"
	// StatementSourceAnnotation 表示语句来源于方法注解。
	StatementSourceAnnotation StatementSource = "annotation"
	// StatementSourceBase 表示语句来源于通用 BaseMapper。
	StatementSourceBase StatementSource = "base"
)

// StatementCachePolicy 描述 Statement 级缓存策略。
type StatementCachePolicy string

const (
	// StatementCacheDefault 表示使用 MyBatis 风格默认缓存策略。
	StatementCacheDefault StatementCachePolicy = ""
	// StatementCacheEnabled 表示显式启用缓存行为。
	StatementCacheEnabled StatementCachePolicy = "enabled"
	// StatementCacheDisabled 表示显式禁用缓存行为。
	StatementCacheDisabled StatementCachePolicy = "disabled"
)

// SelectKeyOrder 表示 selectKey 的执行时机。
type SelectKeyOrder string

const (
	// SelectKeyOrderBefore 表示先生成主键再执行主写入语句。
	SelectKeyOrderBefore SelectKeyOrder = "BEFORE"
	// SelectKeyOrderAfter 表示主写入语句执行后再查询生成主键。
	SelectKeyOrderAfter SelectKeyOrder = "AFTER"
)

// FieldFill 描述 MyBatis-Plus 风格字段自动填充时机。
type FieldFill string

const (
	// FieldFillDefault 表示字段不参与严格自动填充。
	FieldFillDefault FieldFill = ""
	// FieldFillInsert 表示字段仅在 INSERT 时参与自动填充。
	FieldFillInsert FieldFill = "INSERT"
	// FieldFillUpdate 表示字段仅在 UPDATE 时参与自动填充。
	FieldFillUpdate FieldFill = "UPDATE"
	// FieldFillInsertUpdate 表示字段在 INSERT 和 UPDATE 时都参与自动填充。
	FieldFillInsertUpdate FieldFill = "INSERT_UPDATE"
)

// ColumnMeta 描述实体字段与数据库列的静态映射。
type ColumnMeta struct {
	FieldName      string
	FieldType      string
	ColumnName     string
	KeyColumn      string
	PrimaryKey     bool
	AutoIncrement  bool
	IDType         IDType
	Nullable       *bool
	Size           *int
	NumericScale   *int
	DBType         string
	DefaultValue   string
	TypeHandler    string
	Condition      string
	SelectDisabled bool
	InsertStrategy FieldStrategy
	UpdateStrategy FieldStrategy
	WhereStrategy  FieldStrategy
	Version        bool
	SoftDelete     bool
	CreatedAt      bool
	UpdatedAt      bool
	Fill           FieldFill
}

// EntityMeta 描述一个实体类型的 ORM 映射。
type EntityMeta struct {
	TypeName string
	Table    string
	Columns  []ColumnMeta
}

// ResultFieldMeta 描述 XML resultMap 中的一项结果映射。
type ResultFieldMeta struct {
	Property    string
	Column      string
	ID          bool
	TypeHandler string
}

// ResultConstructorMeta 描述 resultMap 的构造参数映射。
type ResultConstructorMeta struct {
	Args []ResultArgMeta
}

// ResultArgMeta 描述 constructor 中的 arg 或 idArg。
type ResultArgMeta struct {
	Name        string
	Property    string
	Column      string
	ID          bool
	TypeHandler string
}

// ResultAssociationMeta 描述 resultMap 中的嵌套对象映射。
type ResultAssociationMeta struct {
	Property       string
	TypeName       string
	Column         string
	ColumnPrefix   string
	NotNullColumns []string
	Select         string
	FetchType      string
	Fields         []ResultFieldMeta
	Associations   []ResultAssociationMeta
	Collections    []ResultCollectionMeta
}

// ResultCollectionMeta 描述 resultMap 中的嵌套集合映射。
type ResultCollectionMeta struct {
	Property       string
	TypeName       string
	Column         string
	ColumnPrefix   string
	NotNullColumns []string
	Select         string
	FetchType      string
	Fields         []ResultFieldMeta
	Associations   []ResultAssociationMeta
	Collections    []ResultCollectionMeta
}

// ResultMapMeta 描述 XML resultMap。
type ResultMapMeta struct {
	ID            string
	TypeName      string
	Extends       string
	AutoMapping   *bool
	Constructor   ResultConstructorMeta
	Fields        []ResultFieldMeta
	Associations  []ResultAssociationMeta
	Collections   []ResultCollectionMeta
	Discriminator ResultDiscriminatorMeta
}

// ResultDiscriminatorMeta 描述 MyBatis resultMap discriminator。
type ResultDiscriminatorMeta struct {
	Column      string
	TypeName    string
	TypeHandler string
	Cases       []ResultDiscriminatorCaseMeta
}

// ResultDiscriminatorCaseMeta 描述 discriminator 的单个 case。
type ResultDiscriminatorCaseMeta struct {
	Value        string
	ResultMap    string
	ResultType   string
	Fields       []ResultFieldMeta
	Associations []ResultAssociationMeta
	Collections  []ResultCollectionMeta
}

// DynamicSQLNodeKind 表示动态 SQL 节点类型。
type DynamicSQLNodeKind string

const (
	// DynamicSQLNodeText 表示普通 SQL 文本。
	DynamicSQLNodeText DynamicSQLNodeKind = "text"
	// DynamicSQLNodeIf 表示条件 SQL 节点。
	DynamicSQLNodeIf DynamicSQLNodeKind = "if"
	// DynamicSQLNodeWhere 表示 WHERE 包装节点。
	DynamicSQLNodeWhere DynamicSQLNodeKind = "where"
	// DynamicSQLNodeSet 表示 SET 包装节点。
	DynamicSQLNodeSet DynamicSQLNodeKind = "set"
	// DynamicSQLNodeTrim 表示通用前后缀修剪节点。
	DynamicSQLNodeTrim DynamicSQLNodeKind = "trim"
	// DynamicSQLNodeForeach 表示集合展开节点。
	DynamicSQLNodeForeach DynamicSQLNodeKind = "foreach"
	// DynamicSQLNodeChoose 表示分支选择节点。
	DynamicSQLNodeChoose DynamicSQLNodeKind = "choose"
	// DynamicSQLNodeWhen 表示 choose 的条件分支。
	DynamicSQLNodeWhen DynamicSQLNodeKind = "when"
	// DynamicSQLNodeOtherwise 表示 choose 的默认分支。
	DynamicSQLNodeOtherwise DynamicSQLNodeKind = "otherwise"
	// DynamicSQLNodeInclude 表示 XML sql 片段引用，生成期应展开。
	DynamicSQLNodeInclude DynamicSQLNodeKind = "include"
	// DynamicSQLNodeBind 表示 XML bind 变量绑定节点。
	DynamicSQLNodeBind DynamicSQLNodeKind = "bind"
)

// DynamicSQLNode 描述 XML 动态 SQL 的静态节点树。
type DynamicSQLNode struct {
	Kind            DynamicSQLNodeKind
	Text            string
	Test            string
	Prefix          string
	Suffix          string
	PrefixOverrides string
	SuffixOverrides string
	Collection      string
	Item            string
	Index           string
	Name            string
	Value           string
	Open            string
	Close           string
	Separator       string
	RefID           string
	Children        []DynamicSQLNode
}

// SelectKeyMeta 描述 MyBatis 风格 selectKey 主键生成语句。
type SelectKeyMeta struct {
	Enabled     bool
	KeyProperty string
	ResultType  string
	Order       SelectKeyOrder
	SQL         string
	DynamicSQL  []DynamicSQLNode
}

// StatementMeta 描述已经编译好的 Mapper 语句元数据。
type StatementMeta struct {
	ID                 string
	Namespace          string
	FullName           string
	Command            StatementCommand
	StatementType      StatementType
	Source             StatementSource
	SQL                string
	Provider           string
	ResultMap          string
	ResultType         string
	ParameterType      string
	DatabaseID         string
	UseGeneratedKeys   bool
	KeyProperty        string
	Options            StatementOptions
	SelectKey          SelectKeyMeta
	UseCache           StatementCachePolicy
	FlushCache         StatementCachePolicy
	Parameters         []string
	ParameterModes     []ParameterMeta
	ResultSets         []ResultSetMeta
	DynamicSQL         []DynamicSQLNode
	InterceptorIgnores []string
}

// CacheMeta 描述 Mapper namespace 级二级缓存配置。
type CacheMeta struct {
	Enabled             bool
	RefNamespace        string
	Eviction            string
	Size                int
	FlushIntervalMillis int64
	ReadOnly            bool
	Blocking            bool
}

// MapperMeta 描述一个 Mapper 接口及其语句集合。
type MapperMeta struct {
	TypeName     string
	Namespace    string
	XML          string
	Cache        CacheMeta
	ResultMaps   []ResultMapMeta
	Statements   []StatementMeta
	ImplTypeName string
}
