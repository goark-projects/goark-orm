package ormtest

import (
	"fmt"

	orm "goark.dev/orm"
)

func benchmarkDDL(dbType orm.DbType, table string) ([]string, []string, error) {
	cleanup := []string{"DROP TABLE IF EXISTS " + table}
	switch dbType {
	case orm.DbTypePostgres:
		return []string{
			"DROP TABLE IF EXISTS " + table,
			"CREATE TABLE " + table + " (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile JSONB NOT NULL, created_at VARCHAR(40) NOT NULL)",
			"INSERT INTO " + table + " (id, name, age, profile, created_at) VALUES (1, 'Alice', 31, '{\"role\":\"admin\",\"level\":7}', '2026-08-26T00:00:00Z')",
		}, cleanup, nil
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		return []string{
			"DROP TABLE IF EXISTS " + table,
			"CREATE TABLE " + table + " (id BIGINT PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile JSON NOT NULL, created_at VARCHAR(40) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			"INSERT INTO " + table + " (id, name, age, profile, created_at) VALUES (1, 'Alice', 31, '{\"role\":\"admin\",\"level\":7}', '2026-08-26T00:00:00Z')",
		}, cleanup, nil
	case orm.DbTypeSQLite:
		return []string{
			"DROP TABLE IF EXISTS " + table,
			"CREATE TABLE " + table + " (id INTEGER PRIMARY KEY, name VARCHAR(64) NOT NULL, age INTEGER NOT NULL, profile JSON NOT NULL, created_at VARCHAR(40) NOT NULL)",
			"INSERT INTO " + table + " (id, name, age, profile, created_at) VALUES (1, 'Alice', 31, '{\"role\":\"admin\",\"level\":7}', '2026-08-26T00:00:00Z')",
		}, cleanup, nil
	case orm.DbTypeSQLServer:
		return []string{
			"DROP TABLE IF EXISTS " + table,
			"CREATE TABLE " + table + " (id BIGINT PRIMARY KEY, name NVARCHAR(64) NOT NULL, age INT NOT NULL, profile NVARCHAR(MAX) NOT NULL CHECK (ISJSON(profile) = 1), created_at NVARCHAR(40) NOT NULL)",
			"INSERT INTO " + table + " (id, name, age, profile, created_at) VALUES (1, 'Alice', 31, '{\"role\":\"admin\",\"level\":7}', '2026-08-26T00:00:00Z')",
		}, cleanup, nil
	case orm.DbTypeOracle:
		drop := compatibilityOracleDropTableSQL(table)
		return []string{
			drop,
			"CREATE TABLE " + table + " (id NUMBER(19) PRIMARY KEY, name VARCHAR2(64 CHAR) NOT NULL, age NUMBER(10) NOT NULL, profile CLOB NOT NULL CHECK (profile IS JSON), created_at VARCHAR2(40 CHAR) NOT NULL)",
			"INSERT INTO " + table + " (id, name, age, profile, created_at) VALUES (1, 'Alice', 31, '{\"role\":\"admin\",\"level\":7}', '2026-08-26T00:00:00Z')",
		}, []string{drop}, nil
	default:
		return nil, nil, unsupportedBenchmarkDBTypeError(dbType)
	}
}

func benchmarkUpsertSQL(table string, dbType orm.DbType) string {
	switch dbType {
	case orm.DbTypePostgres:
		return "insert into " + table + " (id, name, age, profile, created_at) values (#{id}, #{name}, #{age}, #{profile}, #{created_at}) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, age = EXCLUDED.age, profile = EXCLUDED.profile, created_at = EXCLUDED.created_at"
	case orm.DbTypeMySQL, orm.DbTypeMariaDB:
		return "insert into " + table + " (id, name, age, profile, created_at) values (#{id}, #{name}, #{age}, #{profile}, #{created_at}) ON DUPLICATE KEY UPDATE name = VALUES(name), age = VALUES(age), profile = VALUES(profile), created_at = VALUES(created_at)"
	case orm.DbTypeSQLServer:
		return "MERGE INTO " + table + " goark_orm_target USING (SELECT #{id} AS id, #{name} AS name, #{age} AS age, #{profile} AS profile, #{created_at} AS created_at) goark_orm_source ON (goark_orm_target.id = goark_orm_source.id) WHEN MATCHED THEN UPDATE SET goark_orm_target.name = goark_orm_source.name, goark_orm_target.age = goark_orm_source.age, goark_orm_target.profile = goark_orm_source.profile, goark_orm_target.created_at = goark_orm_source.created_at WHEN NOT MATCHED THEN INSERT (id, name, age, profile, created_at) VALUES (goark_orm_source.id, goark_orm_source.name, goark_orm_source.age, goark_orm_source.profile, goark_orm_source.created_at);"
	case orm.DbTypeOracle:
		return "MERGE INTO " + table + " goark_orm_target USING (SELECT #{id} AS id, #{name} AS name, #{age} AS age, #{profile} AS profile, #{created_at} AS created_at FROM dual) goark_orm_source ON (goark_orm_target.id = goark_orm_source.id) WHEN MATCHED THEN UPDATE SET goark_orm_target.name = goark_orm_source.name, goark_orm_target.age = goark_orm_source.age, goark_orm_target.profile = goark_orm_source.profile, goark_orm_target.created_at = goark_orm_source.created_at WHEN NOT MATCHED THEN INSERT (id, name, age, profile, created_at) VALUES (goark_orm_source.id, goark_orm_source.name, goark_orm_source.age, goark_orm_source.profile, goark_orm_source.created_at)"
	default:
		return fmt.Sprintf("unsupported upsert database %q", dbType)
	}
}
