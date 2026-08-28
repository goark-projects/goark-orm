package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	orm "goark.dev/orm"
	"goark.dev/orm/examples/production/account"
)

// UserApplication 暴露账号场景的业务入口。
type UserApplication struct {
	mapper            account.UserMapper
	timeout           time.Duration
	defaultPageSize   int64
	maxPageSize       int64
	defaultEmailLimit int
	maxEmailLimit     int
}

// NewUserApplication 创建账号业务服务。
func NewUserApplication(mapper account.UserMapper, options UserApplicationOptions) (*UserApplication, error) {
	if mapper == nil {
		return nil, fmt.Errorf("goark-orm example: user mapper is nil")
	}
	app := &UserApplication{
		mapper:            mapper,
		timeout:           firstDuration(options.Timeout, defaultQueryTimeout),
		defaultPageSize:   firstInt64(options.DefaultPageSize, defaultPageSize),
		maxPageSize:       firstInt64(options.MaxPageSize, defaultMaxPageSize),
		defaultEmailLimit: firstInt(options.DefaultEmailLimit, defaultEmailLimit),
		maxEmailLimit:     firstInt(options.MaxEmailLimit, defaultMaxEmailLimit),
	}
	if app.maxPageSize < 1 {
		return nil, fmt.Errorf("goark-orm example: max page size must be positive")
	}
	if app.maxEmailLimit < 1 {
		return nil, fmt.Errorf("goark-orm example: max email limit must be positive")
	}
	return app, nil
}

// GetUser 按租户和主键读取账号。
func (a *UserApplication) GetUser(ctx context.Context, tenantID string, id int64) (*account.User, error) {
	tenantID, err := normalizeTenantID(tenantID)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveID(id); err != nil {
		return nil, err
	}
	ctx, cancel, err := a.callContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return a.mapper.FindByID(ctx, tenantID, id)
}

// ListUsers 分页读取租户账号。
func (a *UserApplication) ListUsers(ctx context.Context, tenantID string, status account.UserStatus, page orm.PageRequest) (orm.Page[account.User], error) {
	tenantID, err := normalizeTenantID(tenantID)
	if err != nil {
		return orm.Page[account.User]{}, err
	}
	page = a.normalizePage(page)
	ctx, cancel, err := a.callContext(ctx)
	if err != nil {
		return orm.Page[account.User]{}, err
	}
	defer cancel()
	return a.mapper.ListByTenant(ctx, tenantID, normalizeStatus(status), page)
}

// ActiveEmails 读取租户内可用账号邮箱。
func (a *UserApplication) ActiveEmails(ctx context.Context, tenantID string, limit int) ([]string, error) {
	tenantID, err := normalizeTenantID(tenantID)
	if err != nil {
		return nil, err
	}
	limit = a.normalizeEmailLimit(limit)
	ctx, cancel, err := a.callContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return a.mapper.ActiveEmails(ctx, tenantID, limit)
}

// ArchiveUser 归档单个账号并返回数据库返回的最新行。
func (a *UserApplication) ArchiveUser(ctx context.Context, tenantID string, id int64) (*account.User, error) {
	tenantID, err := normalizeTenantID(tenantID)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveID(id); err != nil {
		return nil, err
	}
	ctx, cancel, err := a.callContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return a.mapper.ArchiveReturning(ctx, tenantID, id)
}

func (a *UserApplication) callContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if a == nil {
		return nil, nil, fmt.Errorf("goark-orm example: user application is nil")
	}
	if ctx == nil {
		return nil, nil, fmt.Errorf("goark-orm example: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if a.timeout <= 0 {
		return ctx, func() {}, nil
	}
	child, cancel := context.WithTimeout(ctx, a.timeout)
	return child, cancel, nil
}

func (a *UserApplication) normalizePage(page orm.PageRequest) orm.PageRequest {
	if page.Current < 1 {
		page.Current = 1
	}
	if page.Size <= 0 {
		page.Size = a.defaultPageSize
	}
	if page.Size > a.maxPageSize {
		page.Size = a.maxPageSize
	}
	return page
}

func (a *UserApplication) normalizeEmailLimit(limit int) int {
	if limit <= 0 {
		return a.defaultEmailLimit
	}
	if limit > a.maxEmailLimit {
		return a.maxEmailLimit
	}
	return limit
}

func normalizeTenantID(tenantID string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "", fmt.Errorf("goark-orm example: tenant id is required")
	}
	return tenantID, nil
}

func normalizeStatus(status account.UserStatus) account.UserStatus {
	return account.UserStatus(strings.TrimSpace(string(status)))
}

func requirePositiveID(id int64) error {
	if id <= 0 {
		return fmt.Errorf("goark-orm example: id must be positive")
	}
	return nil
}

func firstDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func firstInt64(value int64, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func firstInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
