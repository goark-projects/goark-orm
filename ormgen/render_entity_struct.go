package ormgen

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"goark.dev/orm"
)

func writeDeclaredEntityStruct(builder *bytes.Buffer, entity EntityModel) error {
	if !entity.DeclareStruct {
		return nil
	}
	builder.WriteString("//goark-orm:entity(table=")
	builder.WriteString(strconv.Quote(entity.Table))
	if entity.KeySequence != "" {
		builder.WriteString(", keySequence=")
		builder.WriteString(strconv.Quote(entity.KeySequence))
	}
	builder.WriteString(")\n")
	builder.WriteString("type ")
	builder.WriteString(entity.TypeName)
	builder.WriteString(" struct {\n")
	for _, column := range entity.Columns {
		tag, err := columnStructTag(column)
		if err != nil {
			return err
		}
		builder.WriteString(column.FieldName)
		builder.WriteByte(' ')
		builder.WriteString(column.FieldType)
		builder.WriteByte(' ')
		builder.WriteString(strconv.Quote(tag))
		builder.WriteByte('\n')
	}
	builder.WriteString("}\n\n")
	return nil
}

func columnStructTag(column ColumnModel) (string, error) {
	items := make([]string, 0, 16)
	if err := appendReverseTagString(&items, "column", column.ColumnName); err != nil {
		return "", err
	}
	if column.PrimaryKey {
		appendReverseTagBool(&items, "primary-key", true)
	}
	if column.AutoIncrement {
		appendReverseTagBool(&items, "auto-increment", true)
	}
	if column.IDType != orm.IDTypeNone {
		if err := appendReverseTagString(&items, "id-type", string(column.IDType)); err != nil {
			return "", err
		}
	}
	if column.Nullable != nil {
		appendReverseTagBool(&items, "nullable", *column.Nullable)
	}
	if column.Size != nil {
		appendReverseTagInt(&items, "size", *column.Size)
	}
	if column.NumericScale != nil {
		appendReverseTagInt(&items, "numeric-scale", *column.NumericScale)
	}
	for _, item := range []struct {
		key   string
		value string
	}{
		{key: "type", value: column.DBType},
		{key: "default", value: column.DefaultValue},
		{key: "type-handler", value: column.TypeHandler},
		{key: "key-column", value: column.KeyColumn},
		{key: "update", value: column.UpdateExpression},
		{key: "condition", value: column.Condition},
	} {
		if err := appendReverseTagString(&items, item.key, item.value); err != nil {
			return "", err
		}
	}
	if column.SelectDisabled {
		appendReverseTagBool(&items, "select", false)
	}
	if column.InsertStrategy != orm.FieldStrategyDefault {
		if err := appendReverseTagString(&items, "insert-strategy", reverseFieldStrategyTagValue(column.InsertStrategy)); err != nil {
			return "", err
		}
	}
	if column.UpdateStrategy != orm.FieldStrategyDefault {
		if err := appendReverseTagString(&items, "update-strategy", reverseFieldStrategyTagValue(column.UpdateStrategy)); err != nil {
			return "", err
		}
	}
	if column.WhereStrategy != orm.FieldStrategyDefault {
		if err := appendReverseTagString(&items, "where-strategy", reverseFieldStrategyTagValue(column.WhereStrategy)); err != nil {
			return "", err
		}
	}
	if column.OrderBy {
		appendReverseTagBool(&items, "order-by", true)
	}
	if column.OrderDesc {
		appendReverseTagBool(&items, "order-desc", true)
	}
	if column.OrderPriority != 0 {
		appendReverseTagInt(&items, "order-priority", column.OrderPriority)
	}
	if column.Version {
		appendReverseTagBool(&items, "version", true)
	}
	if column.SoftDelete {
		appendReverseTagBool(&items, "soft-delete", true)
	}
	if column.CreatedAt {
		appendReverseTagBool(&items, "created-at", true)
	}
	if column.UpdatedAt {
		appendReverseTagBool(&items, "updated-at", true)
	}
	if column.Fill != orm.FieldFillDefault {
		if err := appendReverseTagString(&items, "fill", reverseFieldFillTagValue(column.Fill)); err != nil {
			return "", err
		}
	}
	return `goark-orm:"` + strings.Join(items, ";") + `"`, nil
}

func reverseFieldStrategyTagValue(value orm.FieldStrategy) string {
	return strings.ToLower(strings.ReplaceAll(string(value), "_", "-"))
}

func reverseFieldFillTagValue(value orm.FieldFill) string {
	return strings.ToLower(string(value))
}

func appendReverseTagString(items *[]string, key string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "';\n\r\t") {
		return fmt.Errorf("goark-orm: generated tag %s value %q contains unsupported character", key, value)
	}
	*items = append(*items, key+"='"+value+"'")
	return nil
}

func appendReverseTagBool(items *[]string, key string, value bool) {
	*items = append(*items, key+"="+strconv.FormatBool(value))
}

func appendReverseTagInt(items *[]string, key string, value int) {
	*items = append(*items, key+"="+strconv.Itoa(value))
}

func modelKnownImportPaths(model *PackageModel) []string {
	if model == nil {
		return nil
	}
	imports := make(map[string]struct{})
	for _, entity := range model.Entities {
		if !entity.DeclareStruct {
			continue
		}
		for _, column := range entity.Columns {
			for _, importPath := range knownImportPathsForFieldType(column.FieldType) {
				imports[importPath] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(imports))
	for importPath := range imports {
		out = append(out, importPath)
	}
	sort.Strings(out)
	return out
}

func knownImportPathsForFieldType(fieldType string) []string {
	fieldType = strings.TrimSpace(fieldType)
	if fieldType == "" {
		return nil
	}
	candidates := []struct {
		qualifier string
		path      string
	}{
		{qualifier: "json", path: "encoding/json"},
		{qualifier: "sql", path: "database/sql"},
		{qualifier: "time", path: "time"},
		{qualifier: "net", path: "net"},
		{qualifier: "netip", path: "net/netip"},
		{qualifier: "url", path: "net/url"},
	}
	out := make([]string, 0, 1)
	for _, candidate := range candidates {
		if fieldTypeContainsQualifier(fieldType, candidate.qualifier) {
			out = append(out, candidate.path)
		}
	}
	return out
}

func fieldTypeContainsQualifier(fieldType string, qualifier string) bool {
	token := qualifier + "."
	index := strings.Index(fieldType, token)
	for index >= 0 {
		if index == 0 || !isGoIdentifierPart(rune(fieldType[index-1])) {
			return true
		}
		next := index + len(token)
		index = strings.Index(fieldType[next:], token)
		if index >= 0 {
			index += next
		}
	}
	return false
}
