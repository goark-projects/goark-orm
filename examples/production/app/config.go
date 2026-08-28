package app

import "time"

const (
	// DefaultRuntimeConfigPath 是生产示例默认运行期配置文件名。
	DefaultRuntimeConfigPath = "goark-orm-runtime.json"

	defaultQueryTimeout  = 3 * time.Second
	defaultPageSize      = int64(20)
	defaultMaxPageSize   = int64(200)
	defaultEmailLimit    = 100
	defaultMaxEmailLimit = 1000
)
