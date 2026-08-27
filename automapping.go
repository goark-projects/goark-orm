package orm

import (
	"fmt"
	"strings"
)

func (s *SQLSession) autoMappingEnabled(resultMap ResultMapMeta, hasResultMap bool) bool {
	if hasResultMap && resultMap.AutoMapping != nil {
		return *resultMap.AutoMapping
	}
	behavior := AutoMappingBehaviorFull
	if s != nil && s.configuration.AutoMappingBehavior != "" {
		behavior = s.configuration.AutoMappingBehavior
	}
	switch behavior {
	case AutoMappingBehaviorNone:
		return false
	case AutoMappingBehaviorPartial:
		return !hasResultMap || !resultMapHasNestedResultObjects(resultMap)
	default:
		return true
	}
}

func resultMapHasNestedResultObjects(resultMap ResultMapMeta) bool {
	if len(resultMap.Associations) > 0 || len(resultMap.Collections) > 0 {
		return true
	}
	for _, item := range resultMap.Discriminator.Cases {
		if len(item.Associations) > 0 || len(item.Collections) > 0 {
			return true
		}
	}
	return false
}

func (s *SQLSession) shouldFailUnknownAutoMappingColumn(bindings map[string]columnBinding) bool {
	if s == nil || s.configuration.AutoMappingUnknownColumnBehavior != AutoMappingUnknownColumnBehaviorFailing {
		return false
	}
	for _, binding := range bindings {
		if binding.autoMapping {
			return true
		}
	}
	return false
}

func unknownAutoMappingColumnError(statement StatementMeta, column string) error {
	return &MappingError{
		Statement: statement.FullName,
		Column:    column,
		Message:   fmt.Sprintf("unknown auto-mapping column %q", strings.TrimSpace(column)),
	}
}
