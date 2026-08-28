package account

import "time"

// UserStatus 描述生产示例中的账号状态。
type UserStatus string

const (
	// UserStatusActive 表示账号可正常访问业务。
	UserStatusActive UserStatus = "ACTIVE"
	// UserStatusLocked 表示账号被安全策略锁定。
	UserStatusLocked UserStatus = "LOCKED"
	// UserStatusArchived 表示账号已归档。
	UserStatusArchived UserStatus = "ARCHIVED"
)

// UserProfile 描述 JSON 字段中的非关系型扩展信息。
type UserProfile struct {
	Tier string   `json:"tier,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

// goark-orm:entity(table="sys_user")
type User struct {
	ID          int64       `goark-orm:"column='id';primary-key=true;id-type='ASSIGN_ID'"`
	TenantID    string      `goark-orm:"column='tenant_id';size=64;nullable=false;insert-strategy='not-empty';where-strategy='not-empty'"`
	Email       string      `goark-orm:"column='email';size=128;nullable=false;insert-strategy='not-empty';update-strategy='not-empty'"`
	DisplayName string      `goark-orm:"column='display_name';size=128;nullable=false;insert-strategy='not-empty';update-strategy='not-empty'"`
	Status      UserStatus  `goark-orm:"column='status';size=32;nullable=false;insert-strategy='not-empty';update-strategy='not-empty'"`
	Profile     UserProfile `goark-orm:"column='profile';type='json';type-handler='json'"`
	Version     int64       `goark-orm:"column='version';version=true"`
	Deleted     bool        `goark-orm:"column='deleted';soft-delete=true"`
	CreatedAt   time.Time   `goark-orm:"column='created_at';fill='insert'"`
	UpdatedAt   time.Time   `goark-orm:"column='updated_at';fill='insert_update'"`
}
