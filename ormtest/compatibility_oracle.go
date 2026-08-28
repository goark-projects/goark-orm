package ormtest

import "strings"

// compatibilityOracleDropTableSQL 生成可重复执行的 Oracle 清理语句。
func compatibilityOracleDropTableSQL(quotedTable string) string {
	return strings.Join([]string{
		"BEGIN",
		"EXECUTE IMMEDIATE 'DROP TABLE " + quotedTable + " PURGE';",
		"EXCEPTION WHEN OTHERS THEN",
		"IF SQLCODE != -942 THEN RAISE; END IF;",
		"END;",
	}, " ")
}
