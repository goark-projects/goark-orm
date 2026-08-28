package runtime

import (
	"context"
	"fmt"
)

const (
	rowScanStackTargetCount     = 16
	rowScanStackConversionCount = 8
)

type scanPostConversion struct {
	statement   string
	column      string
	field       string
	handlerName string
	handler     TypeHandler
	value       *any
	target      any
}

func rowScanTargets(count int, stack *[rowScanStackTargetCount]any) []any {
	if count <= 0 {
		return nil
	}
	if count <= len(stack) {
		return stack[:count]
	}
	return make([]any, count)
}

func applyScanPostConversions(ctx context.Context, postScan []scanPostConversion) error {
	for _, conversion := range postScan {
		if conversion.handler == nil {
			return &MappingError{
				Statement: conversion.statement,
				Column:    conversion.column,
				Field:     conversion.field,
				Message:   fmt.Sprintf("type-handler %q is not registered", conversion.handlerName),
			}
		}
		if conversion.value == nil {
			continue
		}
		if err := conversion.handler.FromDB(ctx, *conversion.value, conversion.target); err != nil {
			return &MappingError{
				Statement: conversion.statement,
				Column:    conversion.column,
				Field:     conversion.field,
				Message:   fmt.Sprintf("type-handler %q failed", conversion.handlerName),
				Err:       err,
			}
		}
	}
	return nil
}
