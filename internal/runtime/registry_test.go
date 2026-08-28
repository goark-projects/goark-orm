package runtime

import (
	"context"
	"testing"
)

func TestRegistry_whenRegisterMapper_shouldIndexStatements(t *testing.T) {
	registry := NewRegistry()

	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Statements: []StatementMeta{
			{
				ID:        "FindByID",
				Namespace: "system.user.UserMapper",
				FullName:  "system.user.UserMapper.FindByID",
				Command:   StatementCommandSelect,
				Source:    StatementSourceAnnotation,
				SQL:       "select id from sys_user where id = #{id}",
			},
		},
	})
	if err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}

	statement, ok := registry.Statement("system.user.UserMapper.FindByID")
	if !ok {
		t.Fatal("expected statement to be indexed")
	}
	if statement.Command != StatementCommandSelect || statement.SQL == "" {
		t.Fatalf("unexpected statement metadata: %#v", statement)
	}
}

func TestRegistry_whenDuplicateMapperNamespace_shouldReject(t *testing.T) {
	registry := NewRegistry()
	meta := MapperMeta{TypeName: "UserMapper", Namespace: "system.user.UserMapper"}
	if err := registry.RegisterMapper(meta); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}

	if err := registry.RegisterMapper(meta); err == nil {
		t.Fatal("expected duplicate namespace error")
	}
}

func TestRegistry_whenReturnedEntityMutated_shouldKeepStoredMetadata(t *testing.T) {
	registry := NewRegistry()
	err := registry.RegisterEntity(EntityMeta{
		TypeName: "User",
		Table:    "sys_user",
		Columns:  []ColumnMeta{{FieldName: "ID", ColumnName: "id", PrimaryKey: true}},
	})
	if err != nil {
		t.Fatalf("register entity failed: %v", err)
	}

	entity, ok := registry.Entity("User")
	if !ok {
		t.Fatal("expected entity metadata")
	}
	entity.Columns[0].ColumnName = "broken"

	entity, ok = registry.Entity("User")
	if !ok {
		t.Fatal("expected entity metadata")
	}
	if entity.Columns[0].ColumnName != "id" {
		t.Fatalf("registry returned mutable metadata: %#v", entity.Columns[0])
	}
}

func TestRegistry_whenRegisterRowScanner_shouldExposeScanner(t *testing.T) {
	registry := NewRegistry()
	scanner := RowScannerFunc(func(ctx context.Context, columns []string, row RowScannerRow, dest any) error {
		return nil
	})

	if err := registry.RegisterRowScanner("User", scanner); err != nil {
		t.Fatalf("register row scanner failed: %v", err)
	}

	actual, ok := registry.RowScanner("User")
	if !ok || actual == nil {
		t.Fatal("expected row scanner")
	}
}

func TestRegistry_whenRegisterRowScannerInvalid_shouldReject(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterRowScanner("", RowScannerFunc(func(ctx context.Context, columns []string, row RowScannerRow, dest any) error {
		return nil
	})); err == nil {
		t.Fatal("expected empty type name error")
	}
	if err := registry.RegisterRowScanner("User", nil); err == nil {
		t.Fatal("expected nil scanner error")
	}
	if err := registry.RegisterRowScanner("User", RowScannerFunc(nil)); err == nil {
		t.Fatal("expected nil scanner function error")
	}
}

func TestRegistry_whenRegisterSQLProviderDescriptor_shouldExposeValidatedDescriptor(t *testing.T) {
	registry := NewRegistry()
	provider := func(context.Context, StatementMeta, NamedArgs) (SQLSource, error) {
		return SQLSource{SQL: "select id from sys_user"}, nil
	}

	err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor(
		"UserSQL.List",
		provider,
		WithSQLProviderCommands(StatementCommandSelect),
		WithSQLProviderStatements("system.user.UserMapper.List"),
	))
	if err != nil {
		t.Fatalf("register provider descriptor failed: %v", err)
	}

	descriptor, ok := registry.SQLProviderDescriptor("UserSQL.List")
	if !ok {
		t.Fatal("expected provider descriptor")
	}
	if descriptor.Name != "UserSQL.List" || descriptor.Provider == nil {
		t.Fatalf("unexpected provider descriptor %#v", descriptor)
	}
	if len(descriptor.Commands) != 1 || descriptor.Commands[0] != StatementCommandSelect {
		t.Fatalf("unexpected provider commands %#v", descriptor.Commands)
	}
	if len(descriptor.Statements) != 1 || descriptor.Statements[0] != "system.user.UserMapper.List" {
		t.Fatalf("unexpected provider statements %#v", descriptor.Statements)
	}

	descriptor.Commands[0] = StatementCommandDelete
	actual, ok := registry.SQLProviderDescriptor("UserSQL.List")
	if !ok {
		t.Fatal("expected provider descriptor")
	}
	if actual.Commands[0] != StatementCommandSelect {
		t.Fatalf("registry returned mutable provider descriptor %#v", actual.Commands)
	}
}

func TestRegistry_whenRegisterSQLProviderDescriptorInvalid_shouldReject(t *testing.T) {
	registry := NewRegistry()
	provider := func(context.Context, StatementMeta, NamedArgs) (SQLSource, error) {
		return SQLSource{SQL: "select id from sys_user"}, nil
	}

	if err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor("", provider)); err == nil {
		t.Fatal("expected empty provider name error")
	}
	if err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor("UserSQL.List", nil)); err == nil {
		t.Fatal("expected nil provider error")
	}
	if err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor("UserSQL.List", provider, WithSQLProviderCommands(""))); err == nil {
		t.Fatal("expected empty provider command error")
	}
	if err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor("UserSQL.List", provider, WithSQLProviderStatements(""))); err == nil {
		t.Fatal("expected empty provider statement error")
	}
}
