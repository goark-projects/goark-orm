package runtime

import "testing"

func TestParseFieldStrategy_whenAliasesProvided_shouldNormalizeStrategy(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected FieldStrategy
	}{
		{name: "default", raw: "", expected: FieldStrategyDefault},
		{name: "always", raw: "always", expected: FieldStrategyAlways},
		{name: "not_null_dash", raw: "not-null", expected: FieldStrategyNotNull},
		{name: "not_empty_upper", raw: "NOT_EMPTY", expected: FieldStrategyNotEmpty},
		{name: "not_zero_dash", raw: "not-zero", expected: FieldStrategyNotZero},
		{name: "never_space", raw: " never ", expected: FieldStrategyNever},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseFieldStrategy(tt.raw)
			if err != nil {
				t.Fatalf("parse field strategy failed: %v", err)
			}
			if actual != tt.expected {
				t.Fatalf("unexpected strategy %q", actual)
			}
		})
	}
}

func TestParseFieldStrategy_whenUnsupportedValueProvided_shouldReject(t *testing.T) {
	_, err := ParseFieldStrategy("ignored")
	if err == nil {
		t.Fatalf("expected unsupported strategy error")
	}
}
