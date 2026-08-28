package account

import (
	"context"
	"reflect"
	"testing"

	orm "goark.dev/orm"
)

func TestProductionAccountMetadata_shouldValidate(t *testing.T) {
	registry := orm.NewRegistry()
	if err := RegisterGoarkORMMetadata(registry); err != nil {
		t.Fatalf("register metadata failed: %v", err)
	}
	if err := RegisterSQLProviders(registry); err != nil {
		t.Fatalf("register providers failed: %v", err)
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("validate registry failed: %v", err)
	}
	mapper, ok := registry.Mapper(UserMapperNamespace)
	if !ok {
		t.Fatalf("expected mapper %s", UserMapperNamespace)
	}
	if len(mapper.Statements) != 4 {
		t.Fatalf("unexpected statement count %d", len(mapper.Statements))
	}
}

func TestProductionAccountProvider_shouldCompileActiveEmails(t *testing.T) {
	registry := orm.NewRegistry()
	if err := RegisterSQLProviders(registry); err != nil {
		t.Fatalf("register providers failed: %v", err)
	}
	descriptor, ok := registry.SQLProviderDescriptor(ActiveEmailsProviderName)
	if !ok {
		t.Fatalf("expected provider %s", ActiveEmailsProviderName)
	}
	source, err := descriptor.Provider(
		context.Background(),
		orm.StatementMeta{ID: "ActiveEmails", FullName: UserMapperNamespace + ".ActiveEmails", Command: orm.StatementCommandSelect},
		orm.NamedArgs{"tenantID": "tenant-a", "limit": 2},
	)
	if err != nil {
		t.Fatalf("provider failed: %v", err)
	}
	compiled, err := orm.CompileSQLContext(context.Background(), source.SQL, source.Args, orm.NewPostgresDialect())
	if err != nil {
		t.Fatalf("compile SQL failed: %v", err)
	}
	expectedSQL := `SELECT "email" FROM "sys_user" WHERE "tenant_id" = $1 AND "status" = $2 AND "deleted" = $3 ORDER BY "email" ASC LIMIT $4`
	if compiled.SQL != expectedSQL {
		t.Fatalf("unexpected SQL %q", compiled.SQL)
	}
	if !reflect.DeepEqual(compiled.Args, []any{"tenant-a", "ACTIVE", false, 2}) {
		t.Fatalf("unexpected args %#v", compiled.Args)
	}
	if source.CacheKey != "tenant:tenant-a:ActiveEmails" {
		t.Fatalf("unexpected cache key %q", source.CacheKey)
	}
}
