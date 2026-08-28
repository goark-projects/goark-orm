package app

import (
	"context"
	"database/sql"
	"time"

	orm "goark.dev/orm"
	"goark.dev/orm/audit"
)

// RuntimeOptions 描述生产示例启动所需的外部资源。
type RuntimeOptions struct {
	DB             *sql.DB
	ConfigPath     string
	AuditRecorder  audit.Recorder
	SQLObserver    func(context.Context, orm.SQLObservation) error
	Clock          func() time.Time
	SessionOptions []orm.SQLSessionOption
}

// UserApplicationOptions 描述业务服务层的资源保护参数。
type UserApplicationOptions struct {
	Timeout           time.Duration
	DefaultPageSize   int64
	MaxPageSize       int64
	DefaultEmailLimit int
	MaxEmailLimit     int
}
