package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRegistryValidate_whenMetadataConsistent_shouldPass(t *testing.T) {
	registry := NewRegistry()
	handler := NewTypeHandler(nil, nil)
	if err := registry.RegisterTypeHandler("profile", handler); err != nil {
		t.Fatalf("register type handler failed: %v", err)
	}
	if err := registry.RegisterCache("system.cache.SharedMapper", NewMemoryCache("system.cache.SharedMapper")); err != nil {
		t.Fatalf("register cache failed: %v", err)
	}
	provider := func(context.Context, StatementMeta, NamedArgs) (SQLSource, error) {
		return SQLSource{SQL: "select id, name, profile from sys_user"}, nil
	}
	if err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor(
		"UserSQL.List",
		provider,
		WithSQLProviderCommands(StatementCommandSelect),
		WithSQLProviderStatements("system.user.UserMapper.List"),
	)); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	if err := registry.RegisterEntity(EntityMeta{
		TypeName: "User",
		Table:    "sys_user",
		Columns: []ColumnMeta{
			{FieldName: "ID", ColumnName: "id", PrimaryKey: true},
			{FieldName: "Profile", ColumnName: "profile", TypeHandler: "profile"},
		},
	}); err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	namespace := "system.user.UserMapper"
	err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: namespace,
		Cache:     CacheMeta{Enabled: true, RefNamespace: "system.cache.SharedMapper"},
		ResultMaps: []ResultMapMeta{
			{
				ID:       "UserResult",
				TypeName: "User",
				Fields: []ResultFieldMeta{
					{Property: "ID", Column: "id", ID: true},
					{Property: "Profile", Column: "profile", TypeHandler: "profile"},
				},
				Associations: []ResultAssociationMeta{
					{Property: "ProfileRef", Select: "FindProfile", Column: "id"},
				},
				Discriminator: ResultDiscriminatorMeta{
					Column:      "kind",
					TypeHandler: "profile",
					Cases: []ResultDiscriminatorCaseMeta{
						{Value: "vip", ResultMap: "VipUserResult"},
					},
				},
			},
			{
				ID:       "VipUserResult",
				TypeName: "User",
				Fields:   []ResultFieldMeta{{Property: "ID", Column: "id", ID: true}},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "List",
				Namespace: namespace,
				FullName:  namespace + ".List",
				Command:   StatementCommandSelect,
				Provider:  "UserSQL.List",
				ResultMap: "UserResult",
			},
			{
				ID:        "FindProfile",
				Namespace: namespace,
				FullName:  namespace + ".FindProfile",
				Command:   StatementCommandSelect,
				SQL:       "select profile from sys_profile where user_id = #{id}",
				ResultMap: "UserResult",
			},
			{
				ID:          "Insert",
				Namespace:   namespace,
				FullName:    namespace + ".Insert",
				Command:     StatementCommandInsert,
				SQL:         "insert into sys_user(id, profile) values(#{user.ID}, #{user.Profile})",
				KeyProperty: "user.ID",
				SelectKey: SelectKeyMeta{
					Enabled:     true,
					KeyProperty: "user.ID",
					ResultType:  "int64",
					Order:       SelectKeyOrderBefore,
					SQL:         "select 1",
				},
				ParameterModes: []ParameterMeta{{Name: "profile", Mode: ParameterModeIn, TypeHandler: "profile"}},
				ResultSets:     []ResultSetMeta{{Name: "users", ResultMap: "UserResult"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}

	if err := registry.Validate(); err != nil {
		t.Fatalf("validate registry failed: %v", err)
	}
	if err := ValidateRegistry(registry); err != nil {
		t.Fatalf("validate registry function failed: %v", err)
	}
}

func TestRegistryValidate_whenReferencesMissing_shouldReportRegistryErrors(t *testing.T) {
	registry := NewRegistry()
	namespace := "system.user.UserMapper"
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: namespace,
		ResultMaps: []ResultMapMeta{
			{
				ID:       "UserResult",
				TypeName: "User",
				Fields:   []ResultFieldMeta{{Property: "Profile", Column: "profile", TypeHandler: "missingHandler"}},
				Associations: []ResultAssociationMeta{
					{Property: "Role", Select: "MissingRole", Column: "{id=user_id"},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "List",
				Namespace: namespace,
				FullName:  namespace + ".List",
				Command:   StatementCommandSelect,
				SQL:       "select id from sys_user",
				ResultMap: "MissingResult",
			},
			{
				ID:        "Insert",
				Namespace: namespace,
				FullName:  namespace + ".Insert",
				Command:   StatementCommandInsert,
				SQL:       "insert into sys_user(name) values(#{name})",
				SelectKey: SelectKeyMeta{
					Enabled: true,
				},
				ParameterModes: []ParameterMeta{{Name: "profile", TypeHandler: "missingHandler"}},
				ResultSets:     []ResultSetMeta{{Name: "users", ResultMap: "MissingResult"}},
			},
		},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}

	err := registry.Validate()
	if !errors.Is(err, ErrRegistry) || !errors.Is(err, ErrORM) {
		t.Fatalf("expected registry classification, got %v", err)
	}
	message := err.Error()
	for _, expected := range []string{
		"MissingResult",
		"missingHandler",
		"MissingRole",
		"selectKey",
		"composite column",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected validation error to contain %q, got %v", expected, err)
		}
	}
}

func TestRegistryValidate_whenResultSetMappingInvalid_shouldReportRegistryErrors(t *testing.T) {
	registry := NewRegistry()
	namespace := "system.order.OrderMapper"
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "OrderMapper",
		Namespace: namespace,
		ResultMaps: []ResultMapMeta{
			{
				ID:       "OrderResult",
				TypeName: "Order",
				Fields:   []ResultFieldMeta{{Property: "ID", Column: "order_id", ID: true}},
				Associations: []ResultAssociationMeta{
					{Property: "User", TypeName: "User", Column: "user_id", ResultSet: "missingUsers"},
				},
				Collections: []ResultCollectionMeta{
					{Property: "Items", TypeName: "OrderItem", Column: "order_id", ResultSet: "items", ForeignColumn: "order_id"},
				},
			},
		},
		Statements: []StatementMeta{
			{
				ID:        "LoadReport",
				Namespace: namespace,
				FullName:  namespace + ".LoadReport",
				Command:   StatementCommandSelect,
				SQL:       "select order_id from orders",
				ResultMap: "OrderResult",
				ResultSets: []ResultSetMeta{
					{Name: "orders"},
					{Name: "items"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}

	err := registry.Validate()
	if !errors.Is(err, ErrRegistry) {
		t.Fatalf("expected registry error, got %v", err)
	}
	message := err.Error()
	for _, expected := range []string{"missingUsers", "foreignColumn"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected validation error to contain %q, got %v", expected, err)
		}
	}
}

func TestRegistryValidate_whenProviderDisallowsStatement_shouldExposeBindingCause(t *testing.T) {
	registry := NewRegistry()
	provider := func(context.Context, StatementMeta, NamedArgs) (SQLSource, error) {
		return SQLSource{SQL: "update sys_user set name = #{name}"}, nil
	}
	if err := registry.RegisterSQLProviderDescriptor(NewSQLProviderDescriptor(
		"UserSQL.Update",
		provider,
		WithSQLProviderCommands(StatementCommandSelect),
	)); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	namespace := "system.user.UserMapper"
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: namespace,
		Statements: []StatementMeta{
			{
				ID:        "Update",
				Namespace: namespace,
				FullName:  namespace + ".Update",
				Command:   StatementCommandUpdate,
				Provider:  "UserSQL.Update",
			},
		},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}

	err := registry.Validate()
	if !errors.Is(err, ErrRegistry) || !errors.Is(err, ErrBinding) {
		t.Fatalf("expected registry error with binding cause, got %v", err)
	}
}

func TestRegistryValidate_whenCacheRefCycleExists_shouldReject(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Cache:     CacheMeta{Enabled: true, RefNamespace: "system.role.RoleMapper"},
	}); err != nil {
		t.Fatalf("register user mapper failed: %v", err)
	}
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "RoleMapper",
		Namespace: "system.role.RoleMapper",
		Cache:     CacheMeta{Enabled: true, RefNamespace: "system.user.UserMapper"},
	}); err != nil {
		t.Fatalf("register role mapper failed: %v", err)
	}

	err := registry.Validate()
	if !errors.Is(err, ErrRegistry) {
		t.Fatalf("expected registry error, got %v", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cache-ref cycle error, got %v", err)
	}
}
