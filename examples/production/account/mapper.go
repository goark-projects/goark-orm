package account

import (
	"context"

	orm "goark.dev/orm"
)

const (
	// UserMapperNamespace 是账号 Mapper 的稳定 namespace。
	UserMapperNamespace = "examples.production.account.UserMapper"
)

// goark-orm:mapper(namespace="examples.production.account.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, tenantID string, id int64) (*User, error)
	ListByTenant(ctx context.Context, tenantID string, status UserStatus, page orm.PageRequest) (orm.Page[User], error)
	ArchiveReturning(ctx context.Context, tenantID string, id int64) (*User, error)

	//goark-orm:select(provider="examples.production.account.UserSQL.ActiveEmails", timeout="1s", fetchSize=128, resultSetType="FORWARD_ONLY")
	ActiveEmails(ctx context.Context, tenantID string, limit int) ([]string, error)
}
