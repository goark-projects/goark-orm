package runtime

import "testing"

func TestNewDialectCapabilities_whenDbTypeProvided_shouldReturnFeatureMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dbType        DbType
		placeholder   DialectPlaceholderStyle
		quote         DialectIdentifierQuoteStyle
		generatedKey  DialectGeneratedKeyStyle
		upsert        DialectUpsertStyle
		rowLock       DialectRowLockStyle
		json          DialectJSONStyle
		orderRequired bool
	}{
		{DbTypeQuestion, DialectPlaceholderQuestion, DialectIdentifierQuoteBacktick, DialectGeneratedKeyNone, DialectUpsertNone, DialectRowLockNone, DialectJSONNone, false},
		{DbTypePostgres, DialectPlaceholderDollarNumber, DialectIdentifierQuoteDouble, DialectGeneratedKeyReturning, DialectUpsertOnConflict, DialectRowLockForUpdate, DialectJSONNative, false},
		{DbTypeMySQL, DialectPlaceholderQuestion, DialectIdentifierQuoteBacktick, DialectGeneratedKeyLastInsertID, DialectUpsertOnDuplicateKey, DialectRowLockForUpdate, DialectJSONNative, false},
		{DbTypeMariaDB, DialectPlaceholderQuestion, DialectIdentifierQuoteBacktick, DialectGeneratedKeyLastInsertID, DialectUpsertOnDuplicateKey, DialectRowLockForUpdate, DialectJSONNative, false},
		{DbTypeSQLite, DialectPlaceholderQuestion, DialectIdentifierQuoteDouble, DialectGeneratedKeyLastInsertID, DialectUpsertOnConflict, DialectRowLockNone, DialectJSONExtension, false},
		{DbTypeSQLServer, DialectPlaceholderAtPNumber, DialectIdentifierQuoteBracket, DialectGeneratedKeyOutput, DialectUpsertMerge, DialectRowLockHints, DialectJSONNative, true},
		{DbTypeOracle, DialectPlaceholderColonNumber, DialectIdentifierQuoteDouble, DialectGeneratedKeyReturningInto, DialectUpsertMerge, DialectRowLockForUpdate, DialectJSONNative, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.dbType), func(t *testing.T) {
			t.Parallel()
			capabilities, err := NewDialectCapabilities(tt.dbType)
			if err != nil {
				t.Fatalf("new capabilities failed: %v", err)
			}
			if capabilities.DBType != tt.dbType ||
				capabilities.Placeholder != tt.placeholder ||
				capabilities.IdentifierQuote != tt.quote ||
				capabilities.GeneratedKey != tt.generatedKey ||
				capabilities.Upsert != tt.upsert ||
				capabilities.RowLock != tt.rowLock ||
				capabilities.JSON != tt.json ||
				capabilities.LimitOffsetRequiresOrderBy != tt.orderRequired {
				t.Fatalf("unexpected capabilities %#v", capabilities)
			}
			if capabilities.SupportsGeneratedKey() != (tt.generatedKey != DialectGeneratedKeyNone) {
				t.Fatalf("unexpected generated-key helper result")
			}
			if capabilities.SupportsUpsert() != (tt.upsert != DialectUpsertNone) {
				t.Fatalf("unexpected upsert helper result")
			}
			if capabilities.SupportsRowLock() != (tt.rowLock != DialectRowLockNone) {
				t.Fatalf("unexpected row-lock helper result")
			}
			if capabilities.SupportsJSON() != (tt.json != DialectJSONNone) {
				t.Fatalf("unexpected JSON helper result")
			}
		})
	}
}

func TestDialectCapabilitiesOf_whenDialectProvided_shouldUseOptionalProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dialect Dialect
		dbType  DbType
	}{
		{name: "postgres", dialect: NewPostgresDialect(), dbType: DbTypePostgres},
		{name: "mysql", dialect: NewMySQLDialect(), dbType: DbTypeMySQL},
		{name: "mariadb", dialect: NewMariaDBDialect(), dbType: DbTypeMariaDB},
		{name: "sqlserver", dialect: NewSQLServerDialect(), dbType: DbTypeSQLServer},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			capabilities := DialectCapabilitiesOf(tt.dialect)
			if capabilities.DBType != tt.dbType {
				t.Fatalf("unexpected db type %q", capabilities.DBType)
			}
			if !capabilities.LimitOffset || !capabilities.BatchInsert || !capabilities.Savepoint {
				t.Fatalf("expected common SQL capabilities, got %#v", capabilities)
			}
		})
	}
}

func TestDialectCapabilitiesOf_whenCustomDialectProvided_shouldFallbackToName(t *testing.T) {
	t.Parallel()

	capabilities := DialectCapabilitiesOf(customCapabilityDialect{name: "customdb"})
	if capabilities.DBType != DbType("customdb") || capabilities.LimitOffset {
		t.Fatalf("unexpected custom capabilities %#v", capabilities)
	}
}

func TestNewDialectCapabilities_whenUnsupportedDbTypeProvided_shouldReturnError(t *testing.T) {
	t.Parallel()

	if _, err := NewDialectCapabilities(DbType("db2")); err == nil {
		t.Fatalf("expected unsupported capability error")
	}
}

type customCapabilityDialect struct {
	name string
}

func (d customCapabilityDialect) Name() string {
	return d.name
}

func (customCapabilityDialect) Placeholder(int) string {
	return "?"
}

func (customCapabilityDialect) QuoteIdent(identifier string) string {
	return identifier
}
