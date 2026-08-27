package orm

import "strings"

// ShrinkSQLWhitespaces 将连续空白压缩为单个空格。
func ShrinkSQLWhitespaces(sqlText string) string {
	return strings.Join(strings.Fields(sqlText), " ")
}
