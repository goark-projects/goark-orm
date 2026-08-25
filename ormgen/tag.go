package ormgen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const structTagKey = "goark-orm"

type fieldTag struct {
	Values map[string]tagValue
}

type tagValue struct {
	Raw    string
	Kind   tagValueKind
	String string
	Bool   bool
	Int    int
}

type tagValueKind string

const (
	tagValueString tagValueKind = "string"
	tagValueBool   tagValueKind = "bool"
	tagValueInt    tagValueKind = "int"
)

func parseGoarkORMStructTag(raw string) (fieldTag, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return fieldTag{}, false, nil
	}
	unquoted, err := strconv.Unquote(raw)
	if err != nil {
		return fieldTag{}, false, fmt.Errorf("goark-orm: struct tag %s is invalid: %w", raw, err)
	}
	value, ok := reflect.StructTag(unquoted).Lookup(structTagKey)
	if !ok {
		return fieldTag{}, false, nil
	}
	parsed, err := ParseFieldTag(value)
	if err != nil {
		return fieldTag{}, true, err
	}
	return parsed, true, nil
}

// ParseFieldTag 解析 goark-orm struct tag 的 value。
func ParseFieldTag(raw string) (fieldTag, error) {
	tag := fieldTag{Values: make(map[string]tagValue)}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return tag, fmt.Errorf("goark-orm: field tag is empty")
	}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			return tag, fmt.Errorf("goark-orm: field tag %q has empty attribute", raw)
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return tag, fmt.Errorf("goark-orm: field tag attribute %q must be key=value", part)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return tag, fmt.Errorf("goark-orm: field tag attribute %q must be key=value", part)
		}
		if _, exists := tag.Values[key]; exists {
			return tag, fmt.Errorf("goark-orm: duplicate field tag attribute %q", key)
		}
		parsed, err := parseTagValue(key, value)
		if err != nil {
			return tag, err
		}
		tag.Values[key] = parsed
	}
	return tag, nil
}

func parseTagValue(key string, raw string) (tagValue, error) {
	expected, ok := tagValueTypes[key]
	if !ok {
		return tagValue{}, fmt.Errorf("goark-orm: unsupported field tag attribute %q", key)
	}
	if strings.HasPrefix(raw, "'") || strings.HasSuffix(raw, "'") {
		if len(raw) < 2 || !strings.HasPrefix(raw, "'") || !strings.HasSuffix(raw, "'") {
			return tagValue{}, fmt.Errorf("goark-orm: field tag attribute %q has invalid string value %q", key, raw)
		}
		if expected != tagValueString {
			return tagValue{}, fmt.Errorf("goark-orm: field tag attribute %q requires %s value", key, expected)
		}
		return tagValue{
			Raw:    raw,
			Kind:   tagValueString,
			String: strings.TrimSuffix(strings.TrimPrefix(raw, "'"), "'"),
		}, nil
	}
	if raw == "true" || raw == "false" {
		if expected != tagValueBool {
			return tagValue{}, fmt.Errorf("goark-orm: field tag attribute %q requires %s value", key, expected)
		}
		value, _ := strconv.ParseBool(raw)
		return tagValue{Raw: raw, Kind: tagValueBool, Bool: value}, nil
	}
	value, err := strconv.Atoi(raw)
	if err == nil {
		if expected != tagValueInt {
			return tagValue{}, fmt.Errorf("goark-orm: field tag attribute %q requires %s value", key, expected)
		}
		return tagValue{Raw: raw, Kind: tagValueInt, Int: value}, nil
	}
	return tagValue{}, fmt.Errorf("goark-orm: field tag attribute %q string value must use single quotes", key)
}

var tagValueTypes = map[string]tagValueKind{
	"column":         tagValueString,
	"type":           tagValueString,
	"default":        tagValueString,
	"id-type":        tagValueString,
	"fill":           tagValueString,
	"type-handler":   tagValueString,
	"primary-key":    tagValueBool,
	"auto-increment": tagValueBool,
	"nullable":       tagValueBool,
	"version":        tagValueBool,
	"soft-delete":    tagValueBool,
	"created-at":     tagValueBool,
	"updated-at":     tagValueBool,
	"transient":      tagValueBool,
	"size":           tagValueInt,
}

func tagString(tag fieldTag, key string) (string, bool, error) {
	value, ok := tag.Values[key]
	if !ok {
		return "", false, nil
	}
	if value.Kind != tagValueString {
		return "", true, fmt.Errorf("goark-orm: field tag attribute %q requires string value", key)
	}
	return value.String, true, nil
}

func tagBool(tag fieldTag, key string) (bool, bool, error) {
	value, ok := tag.Values[key]
	if !ok {
		return false, false, nil
	}
	if value.Kind != tagValueBool {
		return false, true, fmt.Errorf("goark-orm: field tag attribute %q requires boolean value", key)
	}
	return value.Bool, true, nil
}

func tagInt(tag fieldTag, key string) (int, bool, error) {
	value, ok := tag.Values[key]
	if !ok {
		return 0, false, nil
	}
	if value.Kind != tagValueInt {
		return 0, true, fmt.Errorf("goark-orm: field tag attribute %q requires integer value", key)
	}
	return value.Int, true, nil
}
