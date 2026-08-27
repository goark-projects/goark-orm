package orm

import "strings"

// MyBatisGlobalConfigFile 描述 JSON 中可序列化的全局配置。
type MyBatisGlobalConfigFile struct {
	DbConfig MyBatisDbConfigFile `json:"dbConfig,omitempty"`
}

// MyBatisDbConfigFile 描述 JSON 中可序列化的数据库全局配置。
type MyBatisDbConfigFile struct {
	IDType              string `json:"idType,omitempty"`
	TablePrefix         string `json:"tablePrefix,omitempty"`
	Schema              string `json:"schema,omitempty"`
	LogicDeleteField    string `json:"logicDeleteField,omitempty"`
	LogicDeleteValue    any    `json:"logicDeleteValue,omitempty"`
	LogicNotDeleteValue any    `json:"logicNotDeleteValue,omitempty"`
	InsertStrategy      string `json:"insertStrategy,omitempty"`
	UpdateStrategy      string `json:"updateStrategy,omitempty"`
	WhereStrategy       string `json:"whereStrategy,omitempty"`
}

// BuildGlobalConfig 转换 JSON 全局配置声明。
func (f MyBatisConfigFile) BuildGlobalConfig() (GlobalConfig, error) {
	source, err := f.singleGlobalConfigFile()
	if err != nil {
		return GlobalConfig{}, err
	}
	if source == nil {
		return DefaultGlobalConfig(), nil
	}
	return source.Build()
}

func (f MyBatisConfigFile) singleGlobalConfigFile() (*MyBatisGlobalConfigFile, error) {
	if f.Global != nil && f.GlobalConfig != nil {
		return nil, configurationErrorf("global and globalConfig cannot both be provided")
	}
	if f.Global != nil {
		return f.Global, nil
	}
	return f.GlobalConfig, nil
}

// Build 转换全局配置声明。
func (f MyBatisGlobalConfigFile) Build() (GlobalConfig, error) {
	dbConfig, err := f.DbConfig.Build()
	if err != nil {
		return GlobalConfig{}, err
	}
	config := DefaultGlobalConfig()
	config.DbConfig = dbConfig
	return normalizeGlobalConfig(config)
}

// Build 转换数据库全局配置声明。
func (f MyBatisDbConfigFile) Build() (DbConfig, error) {
	config := DefaultDbConfig()
	if strings.TrimSpace(f.IDType) != "" {
		idType, err := ParseIDType(f.IDType)
		if err != nil {
			return DbConfig{}, err
		}
		config.IDType = idType
	}
	config.TablePrefix = strings.TrimSpace(f.TablePrefix)
	config.Schema = strings.TrimSpace(f.Schema)
	config.LogicDeleteField = strings.TrimSpace(f.LogicDeleteField)
	if f.LogicDeleteValue != nil {
		config.LogicDeleteValue = f.LogicDeleteValue
	}
	if f.LogicNotDeleteValue != nil {
		config.LogicNotDeleteValue = f.LogicNotDeleteValue
	}
	if strings.TrimSpace(f.InsertStrategy) != "" {
		strategy, err := ParseFieldStrategy(f.InsertStrategy)
		if err != nil {
			return DbConfig{}, err
		}
		config.InsertStrategy = strategy
	}
	if strings.TrimSpace(f.UpdateStrategy) != "" {
		strategy, err := ParseFieldStrategy(f.UpdateStrategy)
		if err != nil {
			return DbConfig{}, err
		}
		config.UpdateStrategy = strategy
	}
	if strings.TrimSpace(f.WhereStrategy) != "" {
		strategy, err := ParseFieldStrategy(f.WhereStrategy)
		if err != nil {
			return DbConfig{}, err
		}
		config.WhereStrategy = strategy
	}
	return normalizeDbConfig(config)
}
