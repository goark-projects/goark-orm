package orm

import "testing"

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
