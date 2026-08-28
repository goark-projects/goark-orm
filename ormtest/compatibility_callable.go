package ormtest

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	orm "goark.dev/orm"
)

// compatibilityCallSQL 返回标准套件用于验证 callable 查询路径的 SQL。
func compatibilityCallSQL(dbType orm.DbType, quotedTable string, quotedRoutine string) string {
	switch dbType {
	case orm.DbTypePostgres:
		return "select id, name, age, profile, created_at from " + quotedRoutine + "(#{minAge})"
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		return "call " + quotedRoutine + "(#{minAge})"
	case orm.DbTypeSQLServer:
		return "exec " + quotedRoutine + " @in_min_age = #{minAge}"
	case orm.DbTypeOracle:
		return "select id, name, age, profile, created_at from " + quotedTable + " where age >= #{minAge} order by id"
	default:
		return ""
	}
}

// compatibilityPostgresCallRoutineDDL 创建 PostgreSQL 结果集函数。
func compatibilityPostgresCallRoutineDDL(quotedTable string, quotedRoutine string) string {
	return strings.Join([]string{
		"CREATE FUNCTION " + quotedRoutine + "(min_age INTEGER)",
		"RETURNS TABLE(id BIGINT, name VARCHAR(64), age INTEGER, profile TEXT, created_at VARCHAR(40))",
		"LANGUAGE SQL",
		"AS 'SELECT t.id, t.name, t.age, t.profile, t.created_at FROM " + quotedTable + " AS t WHERE t.age >= min_age ORDER BY t.id'",
	}, " ")
}

// compatibilityMySQLCallRoutineDDL 创建 MySQL 结果集过程。
func compatibilityMySQLCallRoutineDDL(quotedTable string, quotedRoutine string) string {
	return strings.Join([]string{
		"CREATE PROCEDURE " + quotedRoutine + "(IN in_min_age INTEGER)",
		"BEGIN",
		"SELECT id, name, age, profile, created_at FROM " + quotedTable + " WHERE age >= in_min_age ORDER BY id;",
		"END",
	}, " ")
}

// compatibilitySQLServerCallRoutineDDL 创建 SQL Server 结果集过程。
func compatibilitySQLServerCallRoutineDDL(quotedTable string, quotedRoutine string) string {
	return strings.Join([]string{
		"CREATE PROCEDURE " + quotedRoutine + " @in_min_age INT AS",
		"BEGIN",
		"SET NOCOUNT ON;",
		"SELECT id, name, age, profile, created_at FROM " + quotedTable + " WHERE age >= @in_min_age ORDER BY id;",
		"END",
	}, " ")
}

func compatibilityCallStatement(namespace string, sqlText string) orm.StatementMeta {
	return orm.StatementMeta{
		ID:            "CallReport",
		Namespace:     namespace,
		FullName:      namespace + ".CallReport",
		Command:       orm.StatementCommandCall,
		StatementType: orm.StatementTypeCallable,
		Source:        orm.StatementSourceAnnotation,
		SQL:           sqlText,
		ParameterType: "CompatibilityRecord",
		ResultSets: []orm.ResultSetMeta{{
			Name:      "records",
			ResultMap: defaultCompatibilityResultMapID,
		}},
	}
}

func compatibilityCallableCase(namespace string, callSQL string) DatabaseCase {
	return DatabaseCase{
		Name: "compatibility-callable",
		Run: func(ctx context.Context, session *orm.SQLSession, _ *sql.DB) error {
			var records []CompatibilityRecord
			_, err := session.CallStatement(
				ctx,
				compatibilityCallStatement(namespace, callSQL),
				orm.NamedArgs{"minAge": 30},
				&records,
			)
			if err != nil {
				return err
			}
			if len(records) != 1 {
				return fmt.Errorf("callable records length = %d", len(records))
			}
			record := records[0]
			if record.ID != 1 || record.Name != "Alice Updated" || record.Age != 32 {
				return fmt.Errorf("unexpected callable record %#v", record)
			}
			if record.Profile.Role != "admin" || record.Profile.Level != 7 {
				return fmt.Errorf("unexpected callable profile %#v", record.Profile)
			}
			return nil
		},
	}
}
