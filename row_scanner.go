package orm

import (
	"context"
	"strings"
)

// RowScannerRow 是生成式行扫描器需要的最小行读取契约。
type RowScannerRow interface {
	Scan(dest ...any) error
}

// RowScanner 描述由生成器输出的实体行扫描器。
type RowScanner interface {
	ScanRow(ctx context.Context, columns []string, row RowScannerRow, dest any) error
}

// RowScannerTypeHandlers 为生成式行扫描器提供 TypeHandler 查找能力。
type RowScannerTypeHandlers interface {
	TypeHandler(name string) (TypeHandler, bool)
}

// TypeHandlerRowScanner 描述需要 TypeHandler 参与字段转换的生成式行扫描器。
type TypeHandlerRowScanner interface {
	RowScanner
	ScanRowWithTypeHandlers(ctx context.Context, columns []string, row RowScannerRow, dest any, handlers RowScannerTypeHandlers) error
}

// RowScannerFunc 将函数适配为 RowScanner。
type RowScannerFunc func(ctx context.Context, columns []string, row RowScannerRow, dest any) error

// ScanRow 执行函数式行扫描器。
func (f RowScannerFunc) ScanRow(ctx context.Context, columns []string, row RowScannerRow, dest any) error {
	if f == nil {
		return mappingErrorf("row scanner function is nil")
	}
	return f(ctx, columns, row, dest)
}

// TypeHandlerRowScannerFunc 将函数适配为 TypeHandlerRowScanner。
type TypeHandlerRowScannerFunc func(ctx context.Context, columns []string, row RowScannerRow, dest any, handlers RowScannerTypeHandlers) error

// ScanRow 执行 TypeHandler 感知的行扫描器，未提供处理器时由扫描器返回确定性错误。
func (f TypeHandlerRowScannerFunc) ScanRow(ctx context.Context, columns []string, row RowScannerRow, dest any) error {
	return f.ScanRowWithTypeHandlers(ctx, columns, row, dest, nil)
}

// ScanRowWithTypeHandlers 执行函数式 TypeHandler 行扫描器。
func (f TypeHandlerRowScannerFunc) ScanRowWithTypeHandlers(ctx context.Context, columns []string, row RowScannerRow, dest any, handlers RowScannerTypeHandlers) error {
	if f == nil {
		return mappingErrorf("row scanner function is nil")
	}
	return f(ctx, columns, row, dest, handlers)
}

type rowScannerTypeHandlerMap map[string]TypeHandler

func (m rowScannerTypeHandlerMap) TypeHandler(name string) (TypeHandler, bool) {
	if m == nil {
		return nil, false
	}
	handler, ok := m[strings.TrimSpace(name)]
	return handler, ok
}
