package orm

import (
	"io"
)

// RuntimeSettings 描述 Goark ORM 运行期 settings 配置。
type RuntimeSettings = MyBatisSettings

// RuntimeEnvironment 描述 Goark ORM 数据库环境配置。
type RuntimeEnvironment = MyBatisEnvironment

// RuntimeConfig 是 Goark ORM 运行期配置声明模型。
type RuntimeConfig = MyBatisConfig

// RuntimeConfigFile 描述可提交的 ORM 运行期 JSON 配置文件。
type RuntimeConfigFile = MyBatisConfigFile

// RuntimeSettingsFile 使用字符串承载需要解析的枚举和时间配置。
type RuntimeSettingsFile = MyBatisSettingsFile

// RuntimeEnvironmentFile 描述 JSON 中的数据库环境。
type RuntimeEnvironmentFile = MyBatisEnvironmentFile

// RuntimeAssembly 描述一次显式运行期装配输入。
type RuntimeAssembly = MyBatisAssembly

// RuntimeAssemblyResult 返回配置装配后的稳定运行期对象。
type RuntimeAssemblyResult = MyBatisAssemblyResult

// DefaultRuntimeConfig 返回可直接构建运行期配置的默认声明模型。
func DefaultRuntimeConfig() RuntimeConfig {
	return DefaultMyBatisConfig()
}

// LoadRuntimeConfig 从 JSON 文件读取运行期配置声明。
func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	return LoadMyBatisConfig(path)
}

// DecodeRuntimeConfig 从 Reader 严格解码运行期配置声明。
func DecodeRuntimeConfig(reader io.Reader) (RuntimeConfig, error) {
	return DecodeMyBatisConfig(reader)
}

// AssembleRuntimeConfig 显式装配配置、注册表和可选 SessionFactory。
func AssembleRuntimeConfig(assembly RuntimeAssembly) (RuntimeAssemblyResult, error) {
	return AssembleMyBatisConfig(assembly)
}

// LoadAndAssembleRuntimeConfig 读取 JSON 配置并完成运行期装配。
func LoadAndAssembleRuntimeConfig(path string, assembly RuntimeAssembly) (RuntimeAssemblyResult, error) {
	return LoadAndAssembleMyBatisConfig(path, assembly)
}
