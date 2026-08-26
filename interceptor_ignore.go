package orm

import "strings"

const (
	// InterceptorNameAll 表示忽略全部可命名拦截器。
	InterceptorNameAll = "all"
	// InterceptorNameSQLObserver 表示 SQL 模板观察拦截器。
	InterceptorNameSQLObserver = "sql-observer"
	// InterceptorNameBlockAttack 表示全表更新/删除保护拦截器。
	InterceptorNameBlockAttack = "block-attack"
	// InterceptorNameDataPermission 表示数据权限条件拦截器。
	InterceptorNameDataPermission = "data-permission"
	// InterceptorNameTenant 表示租户条件拦截器。
	InterceptorNameTenant = "tenant"
	// InterceptorNameDynamicTable 表示动态表名拦截器。
	InterceptorNameDynamicTable = "dynamic-table"
	// InterceptorNamePagination 表示分页拦截器。
	InterceptorNamePagination = "pagination"
	// InterceptorNameEntitySemantic 表示实体语义拦截器。
	InterceptorNameEntitySemantic = "entity-semantic"
	// InterceptorNameSQLGuard 表示通用 SQL 治理规则拦截器。
	InterceptorNameSQLGuard = "sql-guard"
	// InterceptorNameIllegalSQL 表示非法 SQL 治理拦截器。
	InterceptorNameIllegalSQL = "illegal-sql"
	// InterceptorNameReadOnly 表示只读治理拦截器。
	InterceptorNameReadOnly = "read-only"
)

// StatementInterceptorIgnored 判断语句是否声明跳过指定拦截器。
func StatementInterceptorIgnored(statement StatementMeta, name string) bool {
	target := canonicalInterceptorName(name)
	if target == "" {
		return false
	}
	for _, item := range statement.InterceptorIgnores {
		item = canonicalInterceptorName(item)
		if item == InterceptorNameAll || item == target {
			return true
		}
	}
	return false
}

func canonicalInterceptorName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	switch value {
	case "sqlobserver", "sql-observer":
		return InterceptorNameSQLObserver
	case "blockattack", "block-attack":
		return InterceptorNameBlockAttack
	case "datapermission", "data-permission":
		return InterceptorNameDataPermission
	case "dynamictable", "dynamic-table":
		return InterceptorNameDynamicTable
	case "entitysemantic", "entity-semantic":
		return InterceptorNameEntitySemantic
	case "sqlguard", "sql-guard":
		return InterceptorNameSQLGuard
	case "illegalsql", "illegal-sql":
		return InterceptorNameIllegalSQL
	case "readonly", "read-only":
		return InterceptorNameReadOnly
	default:
		return value
	}
}
