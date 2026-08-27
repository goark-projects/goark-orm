package ormgen

import "testing"

func TestParseFieldTag_whenValid_shouldParseTypedAttributes(t *testing.T) {
	tag, err := ParseFieldTag("column='id';primary-key=true;auto-increment=true;size=64;fill='insert_update';update='%s + 1';order-by=true;order-desc=true;order-priority=2")
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
	fill, ok, err := tagString(tag, "fill")
	if err != nil {
		t.Fatalf("read fill failed: %v", err)
	}
	if !ok || fill != "insert_update" {
		t.Fatalf("unexpected fill: ok=%v value=%q", ok, fill)
	}
	update, ok, err := tagString(tag, "update")
	if err != nil {
		t.Fatalf("read update failed: %v", err)
	}
	if !ok || update != "%s + 1" {
		t.Fatalf("unexpected update: ok=%v value=%q", ok, update)
	}
	order, ok, err := tagBool(tag, "order-by")
	if err != nil {
		t.Fatalf("read order-by failed: %v", err)
	}
	if !ok || !order {
		t.Fatalf("unexpected order-by: ok=%v value=%v", ok, order)
	}
	priority, ok, err := tagInt(tag, "order-priority")
	if err != nil {
		t.Fatalf("read order-priority failed: %v", err)
	}
	if !ok || priority != 2 {
		t.Fatalf("unexpected order-priority: ok=%v value=%d", ok, priority)
	}
}

func TestParseFieldTag_whenInvalid_shouldReject(t *testing.T) {
	cases := []string{
		"",
		"column=id",
		"column='id';primary-key",
		"column='id',primary-key=true",
		"column='id';size='64'",
		"column='id';fill=true",
	}
	for _, item := range cases {
		if _, err := ParseFieldTag(item); err == nil {
			t.Fatalf("expected parse error for %q", item)
		}
	}
}
