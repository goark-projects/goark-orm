package ormgen

import (
	"fmt"
	"strings"

	orm "goark.dev/orm"
)

func parseXMLResultSetsAttribute(raw string) ([]orm.ResultSetMeta, error) {
	names := splitXMLColumnList(raw)
	if len(names) == 0 {
		return nil, nil
	}
	out := make([]orm.ResultSetMeta, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("goark-orm: XML resultSets contains empty name")
		}
		out = append(out, orm.ResultSetMeta{Name: name})
	}
	return out, nil
}
