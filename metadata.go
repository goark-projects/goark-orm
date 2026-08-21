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
)

// StatementSource 表示 SQL 语句来源。
type StatementSource string

const (
	// StatementSourceXML 表示语句来源于 Mapper XML。
	StatementSourceXML StatementSource = "xml"
	// StatementSourceAnnotation 表示语句来源于方法注解。
	StatementSourceAnnotation StatementSource = "annotation"
)

// ColumnMeta 描述实体字段与数据库列的静态映射。
type ColumnMeta struct {
	FieldName     string
	FieldType     string
	ColumnName    string
	PrimaryKey    bool
	AutoIncrement bool
	Nullable      *bool
	Size          *int
	DBType        string
	DefaultValue  string
	TypeHandler   string
	Version       bool
	SoftDelete    bool
	CreatedAt     bool
	UpdatedAt     bool
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

// ResultMapMeta 描述 XML resultMap。
type ResultMapMeta struct {
	ID       string
	TypeName string
	Fields   []ResultFieldMeta
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
	Open            string
	Close           string
	Separator       string
	RefID           string
	Children        []DynamicSQLNode
}

// StatementMeta 描述已经编译好的 Mapper 语句元数据。
type StatementMeta struct {
	ID               string
	Namespace        string
	FullName         string
	Command          StatementCommand
	Source           StatementSource
	SQL              string
	ResultMap        string
	ResultType       string
	ParameterType    string
	UseGeneratedKeys bool
	KeyProperty      string
	DynamicSQL       []DynamicSQLNode
}

// MapperMeta 描述一个 Mapper 接口及其语句集合。
type MapperMeta struct {
	TypeName     string
	Namespace    string
	XML          string
	ResultMaps   []ResultMapMeta
	Statements   []StatementMeta
	ImplTypeName string
}
