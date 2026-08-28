package runtime

import (
	"fmt"
	"sort"
	"strings"
)

// InjectNamespaceResolver 根据实体元数据解析注入 Mapper 的 namespace。
type InjectNamespaceResolver func(entity EntityMeta) string

// DefaultSQLInjector 生成常用 BaseMapper 风格的显式 Statement。
type DefaultSQLInjector struct{}

// Inspect 生成默认通用方法 Statement。
func (DefaultSQLInjector) Inspect(entity EntityMeta, dialect Dialect, global GlobalConfig) ([]StatementMeta, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	global, err := normalizeGlobalConfig(global)
	if err != nil {
		return nil, err
	}
	builder, err := newDefaultSQLInjectBuilder(entity, dialect, global.DbConfig)
	if err != nil {
		return nil, err
	}
	return builder.statements()
}

// RegisterDefaultInjectedStatementsForRegistry 为 Registry 中的实体注册默认通用方法。
func RegisterDefaultInjectedStatementsForRegistry(registry *Registry, namespaceResolver InjectNamespaceResolver, options ...InjectOption) error {
	if registry == nil {
		return configurationErrorf("registry is nil")
	}
	entities := registry.Entities()
	sort.Slice(entities, func(i int, j int) bool {
		return entities[i].TypeName < entities[j].TypeName
	})
	for _, entity := range entities {
		namespace := ""
		if namespaceResolver != nil {
			namespace = namespaceResolver(copyEntityMeta(entity))
		}
		if err := RegisterInjectedStatements(registry, namespace, entity, DefaultSQLInjector{}, options...); err != nil {
			return err
		}
	}
	return nil
}

type defaultSQLInjectBuilder struct {
	entity   EntityMeta
	dialect  Dialect
	dbConfig DbConfig
	primary  ColumnMeta
	semantic baseMapperSemanticColumns
}

func newDefaultSQLInjectBuilder(entity EntityMeta, dialect Dialect, dbConfig DbConfig) (defaultSQLInjectBuilder, error) {
	copied := copyEntityMeta(entity)
	primary, err := singlePrimaryColumn(copied)
	if err != nil {
		return defaultSQLInjectBuilder{}, err
	}
	semantic, err := collectBaseMapperSemanticColumnsWithDbConfig(copied, dbConfig)
	if err != nil {
		return defaultSQLInjectBuilder{}, err
	}
	if _, err := quoteIdentifierPath(dialect, effectiveTableName(copied.Table, dbConfig)); err != nil {
		return defaultSQLInjectBuilder{}, err
	}
	for _, column := range copied.Columns {
		if _, err := quoteIdentifierPath(dialect, column.ColumnName); err != nil {
			return defaultSQLInjectBuilder{}, err
		}
	}
	return defaultSQLInjectBuilder{
		entity:   copied,
		dialect:  dialect,
		dbConfig: dbConfig,
		primary:  primary,
		semantic: semantic,
	}, nil
}

func (b defaultSQLInjectBuilder) statements() ([]StatementMeta, error) {
	statements := make([]StatementMeta, 0, 4)
	selectByID, err := b.selectByID()
	if err != nil {
		return nil, err
	}
	selectCount, err := b.selectCount()
	if err != nil {
		return nil, err
	}
	physicalDeleteByID, err := b.physicalDeleteByID()
	if err != nil {
		return nil, err
	}
	statements = append(statements, selectByID, selectCount, physicalDeleteByID)
	if b.semantic.hasSoftDelete {
		logicDeleteByID, err := b.logicDeleteByID()
		if err != nil {
			return nil, err
		}
		statements = append(statements, logicDeleteByID)
	}
	return statements, nil
}

func (b defaultSQLInjectBuilder) selectByID() (StatementMeta, error) {
	sqlText, err := b.selectBaseSQL()
	if err != nil {
		return StatementMeta{}, err
	}
	primary, err := quoteIdentifierPath(b.dialect, b.primary.ColumnName)
	if err != nil {
		return StatementMeta{}, err
	}
	parameters := []string{"id"}
	sqlText += " WHERE " + primary + " = #{id}"
	if b.semantic.hasSoftDelete {
		live, err := b.softDeleteLiveCondition("live")
		if err != nil {
			return StatementMeta{}, err
		}
		sqlText += " AND " + live
		parameters = append(parameters, "live")
	}
	return b.statement("SelectByID", StatementCommandSelect, sqlText, b.entity.TypeName, parameters, StatementCacheDefault), nil
}

func (b defaultSQLInjectBuilder) selectCount() (StatementMeta, error) {
	table, err := b.quotedTable()
	if err != nil {
		return StatementMeta{}, err
	}
	parameters := []string(nil)
	sqlText := "SELECT COUNT(1) FROM " + table
	if b.semantic.hasSoftDelete {
		live, err := b.softDeleteLiveCondition("live")
		if err != nil {
			return StatementMeta{}, err
		}
		sqlText += " WHERE " + live
		parameters = []string{"live"}
	}
	return b.statement("SelectCount", StatementCommandSelect, sqlText, "int64", parameters, StatementCacheDefault), nil
}

func (b defaultSQLInjectBuilder) physicalDeleteByID() (StatementMeta, error) {
	table, err := b.quotedTable()
	if err != nil {
		return StatementMeta{}, err
	}
	primary, err := quoteIdentifierPath(b.dialect, b.primary.ColumnName)
	if err != nil {
		return StatementMeta{}, err
	}
	sqlText := "DELETE FROM " + table + " WHERE " + primary + " = #{id}"
	return b.statement("PhysicalDeleteByID", StatementCommandDelete, sqlText, "int64", []string{"id"}, StatementCacheEnabled), nil
}

func (b defaultSQLInjectBuilder) logicDeleteByID() (StatementMeta, error) {
	table, err := b.quotedTable()
	if err != nil {
		return StatementMeta{}, err
	}
	primary, err := quoteIdentifierPath(b.dialect, b.primary.ColumnName)
	if err != nil {
		return StatementMeta{}, err
	}
	deleted, err := quoteIdentifierPath(b.dialect, b.semantic.softDeleteColumn.ColumnName)
	if err != nil {
		return StatementMeta{}, err
	}
	live, err := b.softDeleteLiveCondition("live")
	if err != nil {
		return StatementMeta{}, err
	}
	sqlText := fmt.Sprintf("UPDATE %s SET %s = #{deleted} WHERE %s = #{id} AND %s", table, deleted, primary, live)
	return b.statement("LogicDeleteByID", StatementCommandUpdate, sqlText, "int64", []string{"deleted", "id", "live"}, StatementCacheEnabled), nil
}

func (b defaultSQLInjectBuilder) statement(id string, command StatementCommand, sqlText string, resultType string, parameters []string, flushCache StatementCachePolicy) StatementMeta {
	return StatementMeta{
		ID:         id,
		Command:    command,
		Source:     StatementSourceBase,
		SQL:        sqlText,
		ResultType: resultType,
		Parameters: append([]string(nil), parameters...),
		FlushCache: flushCache,
	}
}

func (b defaultSQLInjectBuilder) selectBaseSQL() (string, error) {
	table, err := b.quotedTable()
	if err != nil {
		return "", err
	}
	columns := make([]string, 0, len(b.entity.Columns))
	for _, column := range b.entity.Columns {
		if column.SelectDisabled {
			continue
		}
		quoted, err := quoteIdentifierPath(b.dialect, column.ColumnName)
		if err != nil {
			return "", err
		}
		columns = append(columns, quoted)
	}
	if len(columns) == 0 {
		return "", fmt.Errorf("goark-orm: entity %s has no selectable columns", b.entity.TypeName)
	}
	return "SELECT " + strings.Join(columns, ", ") + " FROM " + table, nil
}

func (b defaultSQLInjectBuilder) quotedTable() (string, error) {
	return quoteIdentifierPath(b.dialect, effectiveTableName(b.entity.Table, b.dbConfig))
}

func (b defaultSQLInjectBuilder) softDeleteLiveCondition(argName string) (string, error) {
	column, err := quoteIdentifierPath(b.dialect, b.semantic.softDeleteColumn.ColumnName)
	if err != nil {
		return "", err
	}
	return column + " = #{" + argName + "}", nil
}
