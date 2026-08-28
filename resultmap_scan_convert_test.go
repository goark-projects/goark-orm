package orm

import (
	"reflect"
	"testing"
)

func TestSetReflectField_whenDatabaseReturnsNumericString_shouldAssignIntegerField(t *testing.T) {
	var out struct {
		ID int64
	}
	field := reflect.ValueOf(&out).Elem().FieldByName("ID")

	if err := setReflectField(field, "42"); err != nil {
		t.Fatalf("set numeric string failed: %v", err)
	}
	if out.ID != 42 {
		t.Fatalf("unexpected ID %d", out.ID)
	}
}

func TestSetReflectField_whenDatabaseReturnsInvalidNumericString_shouldReject(t *testing.T) {
	var out struct {
		ID int64
	}
	field := reflect.ValueOf(&out).Elem().FieldByName("ID")

	err := setReflectField(field, "not-number")
	if err == nil {
		t.Fatal("expected invalid numeric string error")
	}
}
