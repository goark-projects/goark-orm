package orm

import "context"

// RowScannerRow 是生成式行扫描器需要的最小行读取契约。
type RowScannerRow interface {
	Scan(dest ...any) error
}

// RowScanner 描述由生成器输出的实体行扫描器。
type RowScanner interface {
	ScanRow(ctx context.Context, columns []string, row RowScannerRow, dest any) error
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
