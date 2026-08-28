package ormtest

import (
	"fmt"
	"sync/atomic"

	orm "goark.dev/orm"
)

const (
	benchmarkInsertIDBase = 10_000_000
	benchmarkBatchIDBase  = 20_000_000
	benchmarkUpsertID     = 30_000_000
)

func benchmarkCases(namespace string, rawTable string, table string, dbType orm.DbType) []DatabaseBenchmarkCase {
	return []DatabaseBenchmarkCase{
		benchmarkPreparedQueryCase(namespace, table, dbType),
		benchmarkResultMapJSONCase(namespace, table, dbType),
		benchmarkInsertCase(namespace, table, dbType),
		benchmarkMultiRowInsertCase(rawTable, dbType),
		benchmarkBatchInsertCase(namespace, table, dbType),
		benchmarkUpsertCase(namespace, table, dbType),
	}
}

func benchmarkPreparedQueryCase(namespace string, table string, dbType orm.DbType) DatabaseBenchmarkCase {
	statement := benchmarkCaseStatement(namespace, table, dbType, "SelectGenerated")
	return DatabaseBenchmarkCase{
		Name: "prepared-query-generated-scan",
		Run: func(scope DatabaseBenchmarkScope) error {
			scope.B.ReportAllocs()
			scope.B.ResetTimer()
			for i := 0; i < scope.B.N; i++ {
				var record CompatibilityRecord
				if err := scope.Session.QueryOneStatement(scope.Context, statement, orm.NamedArgs{"id": int64(1)}, &record); err != nil {
					return err
				}
				if record.ID != 1 || record.Profile.Level != 7 {
					return fmt.Errorf("unexpected generated scan record %#v", record)
				}
			}
			return nil
		},
	}
}

func benchmarkResultMapJSONCase(namespace string, table string, dbType orm.DbType) DatabaseBenchmarkCase {
	statement := benchmarkCaseStatement(namespace, table, dbType, "SelectResultMap")
	return DatabaseBenchmarkCase{
		Name: "resultmap-json-native",
		Run: func(scope DatabaseBenchmarkScope) error {
			scope.B.ReportAllocs()
			scope.B.ResetTimer()
			for i := 0; i < scope.B.N; i++ {
				var record CompatibilityRecord
				if err := scope.Session.QueryOneStatement(scope.Context, statement, orm.NamedArgs{"id": int64(1)}, &record); err != nil {
					return err
				}
				if record.ID != 1 || record.Profile.Role != "admin" {
					return fmt.Errorf("unexpected resultmap record %#v", record)
				}
			}
			return nil
		},
	}
}

func benchmarkInsertCase(namespace string, table string, dbType orm.DbType) DatabaseBenchmarkCase {
	statement := benchmarkCaseStatement(namespace, table, dbType, "Insert")
	var seq atomic.Int64
	return DatabaseBenchmarkCase{
		Name: "insert-typehandler",
		Run: func(scope DatabaseBenchmarkScope) error {
			scope.B.ReportAllocs()
			scope.B.ResetTimer()
			for i := 0; i < scope.B.N; i++ {
				id := benchmarkInsertIDBase + seq.Add(1)
				record := benchmarkRecord(id, "Insert")
				result, err := scope.Session.ExecStatement(scope.Context, statement, orm.NamedArgs{"record": record})
				if err != nil {
					return err
				}
				if result.RowsAffected != 1 {
					return fmt.Errorf("insert rows affected = %d", result.RowsAffected)
				}
			}
			return nil
		},
	}
}

func benchmarkBatchInsertCase(namespace string, table string, dbType orm.DbType) DatabaseBenchmarkCase {
	const batchSize = 16
	statement := benchmarkCaseStatement(namespace, table, dbType, "Insert")
	var seq atomic.Int64
	return DatabaseBenchmarkCase{
		Name: "batch-insert-16",
		Run: func(scope DatabaseBenchmarkScope) error {
			scope.B.ReportAllocs()
			scope.B.ResetTimer()
			for i := 0; i < scope.B.N; i++ {
				batch, err := orm.NewBatchSession(scope.Session)
				if err != nil {
					return err
				}
				for item := 0; item < batchSize; item++ {
					id := benchmarkBatchIDBase + seq.Add(1)
					record := benchmarkRecord(id, "Batch")
					if _, err := batch.ExecStatement(scope.Context, statement, orm.NamedArgs{"record": record}); err != nil {
						return err
					}
				}
				results, err := batch.Flush(scope.Context)
				if err != nil {
					return err
				}
				if len(results) != batchSize {
					return fmt.Errorf("batch result length = %d", len(results))
				}
			}
			return nil
		},
	}
}

func benchmarkMultiRowInsertCase(table string, dbType orm.DbType) DatabaseBenchmarkCase {
	const batchSize = 16
	var seq atomic.Int64
	columns := benchmarkMultiRowColumns(dbType)
	return DatabaseBenchmarkCase{
		Name: "multirow-insert-16",
		Run: func(scope DatabaseBenchmarkScope) error {
			scope.B.ReportAllocs()
			scope.B.ResetTimer()
			for i := 0; i < scope.B.N; i++ {
				rows := make([]orm.NamedArgs, 0, batchSize)
				for item := 0; item < batchSize; item++ {
					id := benchmarkBatchIDBase + 100_000_000 + seq.Add(1)
					rows = append(rows, benchmarkMultiRowArgs(id, dbType))
				}
				source, err := orm.NewMultiRowInsertSQLBuilder().
					Into(table).
					Columns(columns...).
					Rows(rows...).
					Build(scope.Session.Dialect())
				if err != nil {
					return err
				}
				compiled, err := orm.CompileSQLContext(scope.Context, source.SQL, source.Args, scope.Session.Dialect())
				if err != nil {
					return err
				}
				result, err := scope.DB.ExecContext(scope.Context, compiled.SQL, compiled.Args...)
				if err != nil {
					return err
				}
				rowsAffected, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if rowsAffected <= 0 {
					return fmt.Errorf("multi-row insert rows affected = %d", rowsAffected)
				}
			}
			return nil
		},
	}
}

func benchmarkUpsertCase(namespace string, table string, dbType orm.DbType) DatabaseBenchmarkCase {
	statement := benchmarkCaseStatement(namespace, table, dbType, "Upsert")
	var seq atomic.Int64
	return DatabaseBenchmarkCase{
		Name: "upsert-native",
		Run: func(scope DatabaseBenchmarkScope) error {
			scope.B.ReportAllocs()
			scope.B.ResetTimer()
			for i := 0; i < scope.B.N; i++ {
				version := seq.Add(1)
				result, err := scope.Session.ExecStatement(scope.Context, statement, benchmarkUpsertArgs(version))
				if err != nil {
					return err
				}
				if result.RowsAffected <= 0 {
					return fmt.Errorf("upsert rows affected = %d", result.RowsAffected)
				}
			}
			return nil
		},
	}
}

func benchmarkRecord(id int64, namePrefix string) CompatibilityRecord {
	return CompatibilityRecord{
		ID:        id,
		Name:      fmt.Sprintf("%s-%d", namePrefix, id),
		Age:       18,
		Profile:   CompatibilityProfile{Role: "bench", Level: 1},
		CreatedAt: "2026-08-26T00:00:00Z",
	}
}

func benchmarkUpsertArgs(version int64) orm.NamedArgs {
	return orm.NamedArgs{
		"id":         int64(benchmarkUpsertID),
		"name":       fmt.Sprintf("Upsert-%d", version),
		"age":        19,
		"profile":    CompatibilityProfile{Role: "upsert", Level: int(version%100) + 1},
		"created_at": "2026-08-26T00:00:00Z",
	}
}

func benchmarkMultiRowColumns(dbType orm.DbType) []string {
	if dbType == orm.DbTypeOracle {
		return []string{"ID", "NAME", "AGE", "PROFILE", "CREATED_AT"}
	}
	return []string{"id", "name", "age", "profile", "created_at"}
}

func benchmarkMultiRowArgs(id int64, dbType orm.DbType) orm.NamedArgs {
	if dbType == orm.DbTypeOracle {
		return orm.NamedArgs{
			"ID":         id,
			"NAME":       fmt.Sprintf("MultiRow-%d", id),
			"AGE":        20,
			"PROFILE":    `{"role":"multirow","level":1}`,
			"CREATED_AT": "2026-08-26T00:00:00Z",
		}
	}
	return orm.NamedArgs{
		"id":         id,
		"name":       fmt.Sprintf("MultiRow-%d", id),
		"age":        20,
		"profile":    `{"role":"multirow","level":1}`,
		"created_at": "2026-08-26T00:00:00Z",
	}
}

func benchmarkCaseStatement(namespace string, table string, dbType orm.DbType, id string) orm.StatementMeta {
	for _, statement := range benchmarkMapperMeta(namespace, table, dbType).Statements {
		if statement.ID == id {
			return statement
		}
	}
	return orm.StatementMeta{}
}
