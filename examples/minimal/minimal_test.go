package minimal

import (
	"testing"

	orm "goark.dev/orm"
)

func TestMinimalExampleGeneratedMetadata_shouldRegister(t *testing.T) {
	registry := orm.NewRegistry()
	if err := RegisterGoarkORMMetadata(registry); err != nil {
		t.Fatalf("register metadata failed: %v", err)
	}
	mapper, ok := registry.Mapper("example.minimal.UserMapper")
	if !ok {
		t.Fatalf("expected generated mapper metadata")
	}
	if len(mapper.Statements) != 2 {
		t.Fatalf("unexpected statement count %d", len(mapper.Statements))
	}
}
