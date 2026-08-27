package ormgen

import "testing"

func TestToSnakeCase_whenNameContainsAcronymAndDigits_shouldDeriveStableName(t *testing.T) {
	cases := map[string]string{
		"AuditLog":     "audit_log",
		"HTTPServerID": "http_server_id",
		"URLValue2":    "url_value_2",
	}
	for input, expected := range cases {
		if actual := toSnakeCase(input); actual != expected {
			t.Fatalf("toSnakeCase(%q)=%q, want %q", input, actual, expected)
		}
	}
}

func TestNormalizeNamingConfig_whenAliasProvided_shouldNormalize(t *testing.T) {
	config, err := normalizeNamingConfig(NamingConfig{
		Table:  "underline",
		Column: "snake",
	})
	if err != nil {
		t.Fatalf("normalize naming config failed: %v", err)
	}
	if config.Table != NamingStrategySnakeCase || config.Column != NamingStrategySnakeCase {
		t.Fatalf("unexpected naming config %#v", config)
	}
}
