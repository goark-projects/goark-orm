package runtime

import (
	"fmt"
	"strings"
)

// SQLInjector 为实体生成可显式注册的通用方法语句。
type SQLInjector interface {
	Inspect(entity EntityMeta, dialect Dialect, global GlobalConfig) ([]StatementMeta, error)
}

// SQLInjectorFunc 允许用函数声明轻量注入器。
type SQLInjectorFunc func(entity EntityMeta, dialect Dialect, global GlobalConfig) ([]StatementMeta, error)

// Inspect 执行函数式 SQL 注入器。
func (f SQLInjectorFunc) Inspect(entity EntityMeta, dialect Dialect, global GlobalConfig) ([]StatementMeta, error) {
	if f == nil {
		return nil, configurationErrorf("SQL injector function is nil")
	}
	return f(entity, dialect, global)
}

// RegisterInjectedStatements 将注入器生成的通用 Statement 注册到 Registry。
func RegisterInjectedStatements(registry *Registry, namespace string, entity EntityMeta, injector SQLInjector, options ...InjectOption) error {
	if registry == nil {
		return configurationErrorf("registry is nil")
	}
	if injector == nil {
		return configurationErrorf("SQL injector is nil")
	}
	opts := injectOptions{dialect: NewQuestionDialect(), global: DefaultGlobalConfig()}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = entity.TypeName + "Mapper"
	}
	statements, err := injector.Inspect(copyEntityMeta(entity), opts.dialect, opts.global)
	if err != nil {
		return err
	}
	mapper := MapperMeta{
		TypeName:   namespace,
		Namespace:  namespace,
		Statements: make([]StatementMeta, 0, len(statements)),
	}
	for _, statement := range statements {
		statement.ID = strings.TrimSpace(statement.ID)
		if statement.ID == "" {
			return configurationErrorf("injected statement id is required")
		}
		statement.Namespace = namespace
		statement.FullName = namespace + "." + statement.ID
		if statement.Source == "" {
			statement.Source = StatementSourceBase
		}
		mapper.Statements = append(mapper.Statements, statement)
	}
	return registry.RegisterMapper(mapper)
}

type injectOptions struct {
	dialect Dialect
	global  GlobalConfig
}

// InjectOption 配置 SQL 注入行为。
type InjectOption func(*injectOptions)

// WithInjectDialect 指定注入器使用的数据库方言。
func WithInjectDialect(dialect Dialect) InjectOption {
	return func(options *injectOptions) {
		if dialect != nil {
			options.dialect = dialect
		}
	}
}

// WithInjectGlobalConfig 指定注入器使用的全局配置。
func WithInjectGlobalConfig(global GlobalConfig) InjectOption {
	return func(options *injectOptions) {
		options.global = global
	}
}

// LogicDeleteByIDInjector 生成按主键逻辑删除语句。
type LogicDeleteByIDInjector struct{}

// Inspect 生成 LogicDeleteByID Statement。
func (LogicDeleteByIDInjector) Inspect(entity EntityMeta, dialect Dialect, global GlobalConfig) ([]StatementMeta, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	global, err := normalizeGlobalConfig(global)
	if err != nil {
		return nil, err
	}
	primary, err := singlePrimaryColumn(entity)
	if err != nil {
		return nil, err
	}
	columns, err := collectBaseMapperSemanticColumnsWithDbConfig(entity, global.DbConfig)
	if err != nil {
		return nil, err
	}
	if !columns.hasSoftDelete {
		return nil, configurationErrorf("entity %s missing soft-delete field", entity.TypeName)
	}
	table, err := quoteIdentifierPath(dialect, effectiveTableName(entity.Table, global.DbConfig))
	if err != nil {
		return nil, err
	}
	primaryColumn, err := quoteIdentifierPath(dialect, primary.ColumnName)
	if err != nil {
		return nil, err
	}
	deleteColumn, err := quoteIdentifierPath(dialect, columns.softDeleteColumn.ColumnName)
	if err != nil {
		return nil, err
	}
	sqlText := fmt.Sprintf(
		"UPDATE %s SET %s = #{deleted} WHERE %s = #{id} AND %s = #{live}",
		table,
		deleteColumn,
		primaryColumn,
		deleteColumn,
	)
	return []StatementMeta{{
		ID:         "LogicDeleteByID",
		Command:    StatementCommandUpdate,
		Source:     StatementSourceBase,
		SQL:        sqlText,
		Parameters: []string{"deleted", "id", "live"},
		ResultType: "int64",
		FlushCache: StatementCacheEnabled,
	}}, nil
}
