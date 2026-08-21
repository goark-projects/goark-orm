package ormgen

import "testing"

func TestParseFieldTag_whenValid_shouldParseTypedAttributes(t *testing.T) {
	tag, err := ParseFieldTag("column='id';primary-key=true;auto-increment=true;size=64")
	if err != nil {
		t.Fatalf("parse tag failed: %v", err)
	}
	column, ok, err := tagString(tag, "column")
	if err != nil {
		t.Fatalf("read column failed: %v", err)
	}
	if !ok || column != "id" {
		t.Fatalf("unexpected column: ok=%v value=%q", ok, column)
	}
	primary, ok, err := tagBool(tag, "primary-key")
	if err != nil {
		t.Fatalf("read primary-key failed: %v", err)
	}
	if !ok || !primary {
		t.Fatalf("unexpected primary-key: ok=%v value=%v", ok, primary)
	}
	size, ok, err := tagInt(tag, "size")
	if err != nil {
		t.Fatalf("read size failed: %v", err)
	}
	if !ok || size != 64 {
		t.Fatalf("unexpected size: ok=%v value=%d", ok, size)
	}
}

func TestParseFieldTag_whenInvalid_shouldReject(t *testing.T) {
	cases := []string{
		"",
		"column=id",
		"column='id';primary-key",
		"column='id',primary-key=true",
		"column='id';size='64'",
	}
	for _, item := range cases {
		if _, err := ParseFieldTag(item); err == nil {
			t.Fatalf("expected parse error for %q", item)
		}
	}
}
