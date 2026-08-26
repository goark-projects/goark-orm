package jsoncodec

import (
	"strings"
	"testing"
)

func TestMarshalAndUnmarshal_whenStructProvided_shouldRoundTripWithSonic(t *testing.T) {
	input := struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}{
		Name: "Alice",
		Age:  30,
	}

	data, err := Marshal(input)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !Valid(data) {
		t.Fatalf("expected valid json, got %q", data)
	}

	var output struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := Unmarshal(data, &output); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if output != input {
		t.Fatalf("unexpected output %#v", output)
	}
}

func TestDecodeStrict_whenUnknownFieldProvided_shouldReject(t *testing.T) {
	var output struct {
		Name string `json:"name"`
	}

	err := DecodeStrict(strings.NewReader(`{"name":"Alice","unknown":true}`), &output)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}
