package ormtest

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	orm "goark.dev/orm"
)

const compatibilityJSONNativeCaseName = "compatibility-json-native"

func compatibilityProfileColumnDDL(dbType orm.DbType) (string, error) {
	switch dbType {
	case orm.DbTypePostgres:
		return "profile JSONB NOT NULL", nil
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		return "profile JSON NOT NULL", nil
	case orm.DbTypeSQLite:
		return "profile TEXT NOT NULL CHECK (json_valid(profile))", nil
	case orm.DbTypeSQLServer:
		return "profile NVARCHAR(MAX) NOT NULL CHECK (ISJSON(profile) = 1)", nil
	case orm.DbTypeOracle:
		return "profile CLOB NOT NULL CHECK (profile IS JSON)", nil
	default:
		return "", unsupportedCompatibilityDBTypeError(dbType)
	}
}

func compatibilityJSONNativeCase(table string) DatabaseCase {
	return DatabaseCase{
		Name: compatibilityJSONNativeCaseName,
		Run: func(ctx context.Context, session *orm.SQLSession, db *sql.DB) error {
			sqlText, err := compatibilityJSONProbeSQL(session.Dialect(), table)
			if err != nil {
				return err
			}
			compiled, err := orm.CompileSQLContext(ctx, sqlText, orm.NamedArgs{"id": int64(1)}, session.Dialect())
			if err != nil {
				return err
			}
			var role any
			var level any
			if err := db.QueryRowContext(ctx, compiled.SQL, compiled.Args...).Scan(&role, &level); err != nil {
				return err
			}
			if compatibilityScalarString(role) != "admin" || compatibilityScalarString(level) != "7" {
				return fmt.Errorf("unexpected native json values role=%#v level=%#v", role, level)
			}
			return nil
		},
	}
}

func compatibilityJSONProbeSQL(dialect orm.Dialect, table string) (string, error) {
	capabilities := orm.DialectCapabilitiesOf(dialect)
	var role string
	var level string
	switch capabilities.DBType {
	case orm.DbTypePostgres:
		role = "profile ->> 'role'"
		level = "profile ->> 'level'"
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		role = "JSON_UNQUOTE(JSON_EXTRACT(profile, '$.role'))"
		level = "JSON_UNQUOTE(JSON_EXTRACT(profile, '$.level'))"
	case orm.DbTypeSQLite:
		role = "json_extract(profile, '$.role')"
		level = "json_extract(profile, '$.level')"
	case orm.DbTypeSQLServer:
		role = "JSON_VALUE(profile, '$.role')"
		level = "JSON_VALUE(profile, '$.level')"
	case orm.DbTypeOracle:
		role = "JSON_VALUE(profile, '$.role')"
		level = "JSON_VALUE(profile, '$.level')"
	default:
		return "", unsupportedCompatibilityDBTypeError(capabilities.DBType)
	}
	return "SELECT " + role + ", " + level + " FROM " + table + " WHERE " + compatibilityIDColumn(dialect) + " = #{id}", nil
}

func compatibilityScalarString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
