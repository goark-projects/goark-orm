package ormtest_test

import (
	"context"
	"database/sql"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/ormtest"
)

func TestV1ORMTestPublicAPIContract_shouldCompileExternalUsage(t *testing.T) {
	items, err := ormtest.ParseSQLList(`["select 1", " ", "select 2"]`, "")
	if err != nil {
		t.Fatalf("parse SQL list failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected SQL list %#v", items)
	}

	config, configured, err := ormtest.LoadDatabaseSuiteConfigFromEnv("GOARK_ORM_CONTRACT_NOT_SET")
	if err != nil {
		t.Fatalf("load env config failed: %v", err)
	}
	if configured || config.DriverName != "" {
		t.Fatalf("unexpected configured env suite %#v", config)
	}

	compatibility, err := ormtest.NewCompatibilitySuiteConfig(
		orm.DbTypePostgres,
		ormtest.WithCompatibilityTable("goark_orm_contract_users"),
		ormtest.WithCompatibilityNamespace("contract.UserMapper"),
		ormtest.WithCompatibilityEnvPrefix("GOARK_ORM_CONTRACT"),
	)
	if err != nil {
		t.Fatalf("new compatibility suite config failed: %v", err)
	}
	if compatibility.DBType != orm.DbTypePostgres || len(compatibility.Cases) == 0 {
		t.Fatalf("unexpected compatibility config %#v", compatibility)
	}
	if !ormtest.IsCompatibilityDBTypeSupported(orm.DbTypePostgres) ||
		!ormtest.IsCompatibilityDBTypeSupported(orm.DbTypeMariaDB) ||
		!ormtest.IsCompatibilityDBTypeSupported(orm.DbTypeSQLite) ||
		len(ormtest.SupportedCompatibilityDBTypes()) != 4 {
		t.Fatalf("unexpected compatibility support boundary")
	}

	statement := orm.StatementMeta{
		ID:        "Find",
		Namespace: "contract.UserMapper",
		FullName:  "contract.UserMapper.Find",
		Command:   orm.StatementCommandSelect,
		SQL:       "select id from contract_user where id = #{id}",
	}
	_ = ormtest.QueryStatementCase[ormtest.CompatibilityRecord]("query", statement, orm.NamedArgs{"id": int64(1)}, nil)
	_ = ormtest.QueryOneStatementCase[ormtest.CompatibilityRecord]("query-one", statement, orm.NamedArgs{"id": int64(1)}, nil)
	_ = ormtest.ExecStatementCase("exec", statement, orm.NamedArgs{"id": int64(1)}, nil)
	_ = ormtest.PageStatementCase[ormtest.CompatibilityRecord]("page", statement, orm.NamedArgs{"id": int64(1)}, orm.NewPageRequest(1, 10), nil)
	_ = ormtest.CallStatementCase("call", statement, orm.NamedArgs{}, nil, nil)
	_ = ormtest.PingCase()

	var _ = ormtest.DatabaseSuiteConfig{
		DriverName: "contract",
		DBType:     orm.DbTypePostgres,
		Registry:   orm.NewRegistry(),
		Cases: []ormtest.DatabaseCase{{
			Name: "noop",
			Run: func(context.Context, *orm.SQLSession, *sql.DB) error {
				return nil
			},
		}},
	}
}
