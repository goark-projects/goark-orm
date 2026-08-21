package ormgen

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"
)

const annotationPrefix = "goark-orm:"

type annotation struct {
	Name string
	Args map[string]string
}

func parseAnnotations(group *ast.CommentGroup) ([]annotation, error) {
	if group == nil {
		return nil, nil
	}
	out := make([]annotation, 0)
	for _, comment := range group.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		if !strings.HasPrefix(text, annotationPrefix) {
			continue
		}
		item, err := parseAnnotation(strings.TrimPrefix(text, annotationPrefix))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func parseAnnotation(raw string) (annotation, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return annotation{}, fmt.Errorf("goark-orm: annotation name is required")
	}
	nameEnd := strings.IndexAny(raw, "( \t")
	if nameEnd < 0 {
		return annotation{Name: raw, Args: map[string]string{}}, nil
	}
	item := annotation{
		Name: strings.TrimSpace(raw[:nameEnd]),
		Args: map[string]string{},
	}
	if item.Name == "" {
		return annotation{}, fmt.Errorf("goark-orm: annotation name is required")
	}
	rest := strings.TrimSpace(raw[nameEnd:])
	if rest == "" {
		return item, nil
	}
	if !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return annotation{}, fmt.Errorf("goark-orm: annotation %q arguments are not closed", item.Name)
	}
	args, err := parseAnnotationArgs(rest[1 : len(rest)-1])
	if err != nil {
		return annotation{}, err
	}
	item.Args = args
	return item, nil
}

func parseAnnotationArgs(raw string) (map[string]string, error) {
	args := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return args, nil
	}
	for _, part := range splitByCommaOutsideQuotes(raw) {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("goark-orm: annotation argument is empty")
		}
		key, value, ok := cutOutsideDoubleQuotes(part, '=')
		if !ok {
			return nil, fmt.Errorf("goark-orm: annotation argument %q must be key=value", part)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return nil, fmt.Errorf("goark-orm: annotation argument %q must be key=value", part)
		}
		if strings.HasPrefix(value, "\"") {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf("goark-orm: annotation argument %q is invalid: %w", key, err)
			}
			value = unquoted
		}
		if _, exists := args[key]; exists {
			return nil, fmt.Errorf("goark-orm: duplicate annotation argument %q", key)
		}
		args[key] = value
	}
	return args, nil
}

func splitByCommaOutsideQuotes(raw string) []string {
	parts := make([]string, 0)
	var builder strings.Builder
	inQuote := false
	escaped := false
	for _, r := range raw {
		if escaped {
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			builder.WriteRune(r)
			escaped = true
			continue
		}
		if r == '"' {
			builder.WriteRune(r)
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			parts = append(parts, builder.String())
			builder.Reset()
			continue
		}
		builder.WriteRune(r)
	}
	parts = append(parts, builder.String())
	return parts
}

func cutOutsideDoubleQuotes(raw string, sep rune) (string, string, bool) {
	inQuote := false
	escaped := false
	for index, r := range raw {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == sep && !inQuote {
			return raw[:index], raw[index+len(string(r)):], true
		}
	}
	return "", "", false
}

func findAnnotation(annotations []annotation, name string) (annotation, bool) {
	for _, item := range annotations {
		if item.Name == name {
			return item, true
		}
	}
	return annotation{}, false
}

func mergeAnnotations(left []annotation, right []annotation) []annotation {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	out := make([]annotation, 0, len(left)+len(right))
	out = append(out, left...)
	out = append(out, right...)
	return out
}
