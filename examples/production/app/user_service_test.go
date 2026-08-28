package app

import (
	"context"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/examples/production/account"
)

type fakeUserMapper struct {
	tenantID string
	status   account.UserStatus
	page     orm.PageRequest
	limit    int
}

func (m *fakeUserMapper) FindByID(ctx context.Context, tenantID string, id int64) (*account.User, error) {
	return &account.User{ID: id, TenantID: tenantID, Status: account.UserStatusActive}, nil
}

func (m *fakeUserMapper) ListByTenant(ctx context.Context, tenantID string, status account.UserStatus, page orm.PageRequest) (orm.Page[account.User], error) {
	m.tenantID = tenantID
	m.status = status
	m.page = page
	return orm.Page[account.User]{Records: []account.User{{ID: 1, TenantID: tenantID, Status: status}}, Current: page.Current, Size: page.Size}, nil
}

func (m *fakeUserMapper) ArchiveReturning(ctx context.Context, tenantID string, id int64) (*account.User, error) {
	return &account.User{ID: id, TenantID: tenantID, Status: account.UserStatusArchived, Deleted: true}, nil
}

func (m *fakeUserMapper) ActiveEmails(ctx context.Context, tenantID string, limit int) ([]string, error) {
	m.tenantID = tenantID
	m.limit = limit
	return []string{"ops@example.com"}, nil
}

func TestUserApplication_shouldNormalizePagingAndTenant(t *testing.T) {
	mapper := &fakeUserMapper{}
	service, err := NewUserApplication(mapper, UserApplicationOptions{MaxPageSize: 50})
	if err != nil {
		t.Fatalf("new application failed: %v", err)
	}
	page, err := service.ListUsers(context.Background(), " tenant-a ", account.UserStatus(" ACTIVE "), orm.PageRequest{Current: 0, Size: 500})
	if err != nil {
		t.Fatalf("list users failed: %v", err)
	}
	if mapper.tenantID != "tenant-a" || mapper.status != account.UserStatusActive {
		t.Fatalf("unexpected call tenant=%q status=%q", mapper.tenantID, mapper.status)
	}
	if mapper.page.Current != 1 || mapper.page.Size != 50 {
		t.Fatalf("unexpected page %#v", mapper.page)
	}
	if page.Current != 1 || page.Size != 50 {
		t.Fatalf("unexpected result page %#v", page)
	}
}

func TestUserApplication_shouldCapEmailLimit(t *testing.T) {
	mapper := &fakeUserMapper{}
	service, err := NewUserApplication(mapper, UserApplicationOptions{DefaultEmailLimit: 10, MaxEmailLimit: 25})
	if err != nil {
		t.Fatalf("new application failed: %v", err)
	}
	emails, err := service.ActiveEmails(context.Background(), "tenant-a", 1000)
	if err != nil {
		t.Fatalf("active emails failed: %v", err)
	}
	if len(emails) != 1 || mapper.limit != 25 {
		t.Fatalf("unexpected emails=%#v limit=%d", emails, mapper.limit)
	}
}

func TestUserApplication_shouldRejectInvalidInput(t *testing.T) {
	service, err := NewUserApplication(&fakeUserMapper{}, UserApplicationOptions{})
	if err != nil {
		t.Fatalf("new application failed: %v", err)
	}
	if _, err := service.GetUser(context.Background(), " ", 1); err == nil {
		t.Fatalf("expected tenant validation error")
	}
	if _, err := service.GetUser(context.Background(), "tenant-a", 0); err == nil {
		t.Fatalf("expected id validation error")
	}
	if _, err := service.ActiveEmails(nil, "tenant-a", 1); err == nil {
		t.Fatalf("expected context validation error")
	}
}
