package orm

import (
	"strings"
)

// GlobalConfig 描述 ORM 全局配置。
type GlobalConfig struct {
	DbConfig            DbConfig
	IdentifierGenerator IdentifierGenerator
	MetaObjectHandler   MetaObjectHandler
}

// DbConfig 描述数据库命名、主键和逻辑删除默认策略。
type DbConfig struct {
	IDType              IDType
	TablePrefix         string
	Schema              string
	LogicDeleteField    string
	LogicDeleteValue    any
	LogicNotDeleteValue any
	InsertStrategy      FieldStrategy
	UpdateStrategy      FieldStrategy
	WhereStrategy       FieldStrategy
}

// DefaultGlobalConfig 返回独立 ORM 的全局默认配置。
func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{DbConfig: DefaultDbConfig()}
}

// DefaultDbConfig 返回数据库层默认配置。
func DefaultDbConfig() DbConfig {
	return DbConfig{
		LogicDeleteValue:    true,
		LogicNotDeleteValue: false,
	}
}

func normalizeGlobalConfig(config GlobalConfig) (GlobalConfig, error) {
	dbConfig, err := normalizeDbConfig(config.DbConfig)
	if err != nil {
		return GlobalConfig{}, err
	}
	config.DbConfig = dbConfig
	return config, nil
}

func normalizeDbConfig(config DbConfig) (DbConfig, error) {
	config.TablePrefix = strings.TrimSpace(config.TablePrefix)
	config.Schema = strings.TrimSpace(config.Schema)
	config.LogicDeleteField = strings.TrimSpace(config.LogicDeleteField)
	if err := validateDbConfigIDType(config.IDType); err != nil {
		return DbConfig{}, err
	}
	if err := validateDbConfigFieldStrategies(config); err != nil {
		return DbConfig{}, err
	}
	if config.Schema != "" && !validIdentifierPart(config.Schema) {
		return DbConfig{}, configurationErrorf("dbConfig schema %q is invalid", config.Schema)
	}
	if config.TablePrefix != "" && !validIdentifierPart(config.TablePrefix+"x") {
		return DbConfig{}, configurationErrorf("dbConfig tablePrefix %q is invalid", config.TablePrefix)
	}
	if config.LogicDeleteField != "" && !validIdentifierPart(config.LogicDeleteField) {
		return DbConfig{}, configurationErrorf("dbConfig logicDeleteField %q is invalid", config.LogicDeleteField)
	}
	defaults := DefaultDbConfig()
	if config.LogicDeleteValue == nil {
		config.LogicDeleteValue = defaults.LogicDeleteValue
	}
	if config.LogicNotDeleteValue == nil {
		config.LogicNotDeleteValue = defaults.LogicNotDeleteValue
	}
	return config, nil
}

func validateDbConfigFieldStrategies(config DbConfig) error {
	for name, strategy := range map[string]FieldStrategy{
		"insertStrategy": config.InsertStrategy,
		"updateStrategy": config.UpdateStrategy,
		"whereStrategy":  config.WhereStrategy,
	} {
		if _, err := ParseFieldStrategy(string(strategy)); err != nil {
			return configurationErrorf("dbConfig %s %q is invalid", name, strategy)
		}
	}
	return nil
}

func validateDbConfigIDType(value IDType) error {
	switch value {
	case IDTypeNone, IDTypeAuto, IDTypeInput, IDTypeAssignID, IDTypeAssignUUID:
		return nil
	default:
		return configurationErrorf("dbConfig idType %q is invalid", value)
	}
}

func effectiveTableName(table string, config DbConfig) string {
	table = strings.TrimSpace(table)
	if table == "" {
		return ""
	}
	table = applyTablePrefix(table, config.TablePrefix)
	if config.Schema != "" && !strings.Contains(table, ".") {
		table = config.Schema + "." + table
	}
	return table
}

func applyTablePrefix(table string, prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || table == "" {
		return table
	}
	qualifier := ""
	name := table
	if index := strings.LastIndex(table, "."); index >= 0 {
		qualifier = table[:index+1]
		name = table[index+1:]
	}
	if strings.HasPrefix(name, prefix) {
		return table
	}
	return qualifier + prefix + name
}

func effectiveColumnIDTypeWithDbConfig(column ColumnMeta, config DbConfig) IDType {
	if column.IDType != "" {
		return column.IDType
	}
	if column.AutoIncrement {
		return IDTypeAuto
	}
	if column.PrimaryKey && config.IDType != IDTypeNone {
		return config.IDType
	}
	return IDTypeNone
}

func logicDeleteValue(config DbConfig) any {
	if config.LogicDeleteValue == nil {
		return true
	}
	return config.LogicDeleteValue
}

func logicNotDeleteValue(config DbConfig) any {
	if config.LogicNotDeleteValue == nil {
		return false
	}
	return config.LogicNotDeleteValue
}

func columnMatchesConfiguredLogicDelete(column ColumnMeta, field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	return column.FieldName == field ||
		column.ColumnName == field ||
		parameterPropertyAlias(column.FieldName) == field ||
		normalizeColumnKey(column.FieldName) == normalizeColumnKey(field) ||
		normalizeColumnKey(column.ColumnName) == normalizeColumnKey(field)
}

func firstIdentifierGenerator(primary IdentifierGenerator, fallback IdentifierGenerator) IdentifierGenerator {
	if primary != nil {
		return primary
	}
	return fallback
}

func firstMetaObjectHandler(primary MetaObjectHandler, fallback MetaObjectHandler) MetaObjectHandler {
	if primary != nil {
		return primary
	}
	return fallback
}
