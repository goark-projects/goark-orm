#!/usr/bin/env sh
set -eu

# 本地发布门禁只验证当前工作树，不负责创建远程流水线。
GO_BIN="${GO:-go}"
GOFMT_BIN="${GOFMT:-gofmt}"
BENCHTIME="${GOARK_ORM_BENCHTIME:-100x}"
BENCHMARKS='Benchmark(CompileSQL_Postgres|RenderDynamicSQL|QueryWrapperBuild|SQLSessionQueryOne(GeneratedRowScanner|TypeHandlerRowScanner|ResultMapTypeHandler)|AppendSQLCondition_GroupedQuery|CountSQLBase_NestedOrderBy|JSONTypeHandler_(ToDB|FromDB))$'

export GOWORK="${GOWORK:-off}"

go_files="$(git ls-files --cached --others --exclude-standard '*.go')"
if [ -n "$go_files" ]; then
	unformatted="$($GOFMT_BIN -l $go_files)"
	if [ -n "$unformatted" ]; then
		printf '%s\n' "$unformatted"
		exit 1
	fi
fi

$GO_BIN run ./cmd/goark-orm generate orm --dir examples/minimal --check
$GO_BIN test -count=1 ./...
$GO_BIN vet ./...
git diff --check
$GO_BIN test -run '^$' -bench "$BENCHMARKS" -benchtime="$BENCHTIME" -benchmem ./internal/runtime
