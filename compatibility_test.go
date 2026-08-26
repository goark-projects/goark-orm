package orm

import "testing"

func TestPublicCompatibilityConstants_whenRead_shouldExposeStableV1Contract(t *testing.T) {
	if ModulePath != "goark.dev/orm" {
		t.Fatalf("unexpected module path %q", ModulePath)
	}
	if APIVersion != "v1" {
		t.Fatalf("unexpected API version %q", APIVersion)
	}
}
