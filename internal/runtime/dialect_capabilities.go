package runtime

import (
	"fmt"
	"strings"
)

// DialectPlaceholderStyle 描述方言占位符风格。
type DialectPlaceholderStyle string

const (
	// DialectPlaceholderQuestion 表示 `?` 占位符。
	DialectPlaceholderQuestion DialectPlaceholderStyle = "question"
	// DialectPlaceholderDollarNumber 表示 PostgreSQL `$1` 风格占位符。
	DialectPlaceholderDollarNumber DialectPlaceholderStyle = "dollar_number"
	// DialectPlaceholderAtPNumber 表示 SQL Server `@p1` 风格占位符。
	DialectPlaceholderAtPNumber DialectPlaceholderStyle = "at_p_number"
	// DialectPlaceholderColonNumber 表示 Oracle `:1` 风格占位符。
	DialectPlaceholderColonNumber DialectPlaceholderStyle = "colon_number"
)

// DialectIdentifierQuoteStyle 描述标识符引用风格。
type DialectIdentifierQuoteStyle string

const (
	// DialectIdentifierQuoteBacktick 表示反引号标识符。
	DialectIdentifierQuoteBacktick DialectIdentifierQuoteStyle = "backtick"
	// DialectIdentifierQuoteDouble 表示双引号标识符。
	DialectIdentifierQuoteDouble DialectIdentifierQuoteStyle = "double_quote"
	// DialectIdentifierQuoteBracket 表示 SQL Server 方括号标识符。
	DialectIdentifierQuoteBracket DialectIdentifierQuoteStyle = "bracket"
)

// DialectGeneratedKeyStyle 描述主键回读能力。
type DialectGeneratedKeyStyle string

const (
	// DialectGeneratedKeyNone 表示方言没有声明统一主键回读能力。
	DialectGeneratedKeyNone DialectGeneratedKeyStyle = ""
	// DialectGeneratedKeyLastInsertID 表示依赖 database/sql Result.LastInsertId。
	DialectGeneratedKeyLastInsertID DialectGeneratedKeyStyle = "last_insert_id"
	// DialectGeneratedKeyReturning 表示支持 INSERT ... RETURNING。
	DialectGeneratedKeyReturning DialectGeneratedKeyStyle = "returning"
	// DialectGeneratedKeyOutput 表示 SQL Server OUTPUT inserted.<column>。
	DialectGeneratedKeyOutput DialectGeneratedKeyStyle = "output"
	// DialectGeneratedKeyReturningInto 表示 Oracle RETURNING ... INTO。
	DialectGeneratedKeyReturningInto DialectGeneratedKeyStyle = "returning_into"
)

// DialectUpsertStyle 描述方言原生 UPSERT 语义。
type DialectUpsertStyle string

const (
	// DialectUpsertNone 表示方言没有声明统一 UPSERT 语义。
	DialectUpsertNone DialectUpsertStyle = ""
	// DialectUpsertOnConflict 表示 ON CONFLICT 语义。
	DialectUpsertOnConflict DialectUpsertStyle = "on_conflict"
	// DialectUpsertOnDuplicateKey 表示 ON DUPLICATE KEY UPDATE 语义。
	DialectUpsertOnDuplicateKey DialectUpsertStyle = "on_duplicate_key"
	// DialectUpsertMerge 表示 MERGE 语义。
	DialectUpsertMerge DialectUpsertStyle = "merge"
)

// DialectRowLockStyle 描述行锁语义。
type DialectRowLockStyle string

const (
	// DialectRowLockNone 表示方言没有声明统一行锁语义。
	DialectRowLockNone DialectRowLockStyle = ""
	// DialectRowLockForUpdate 表示 SELECT ... FOR UPDATE。
	DialectRowLockForUpdate DialectRowLockStyle = "for_update"
	// DialectRowLockHints 表示 SQL Server WITH (...) 锁提示。
	DialectRowLockHints DialectRowLockStyle = "lock_hints"
)

// DialectJSONStyle 描述 JSON 列支持级别。
type DialectJSONStyle string

const (
	// DialectJSONNone 表示方言没有声明 JSON 能力。
	DialectJSONNone DialectJSONStyle = ""
	// DialectJSONNative 表示数据库内建 JSON 类型或函数。
	DialectJSONNative DialectJSONStyle = "native"
	// DialectJSONExtension 表示 JSON 能力依赖数据库扩展或构建选项。
	DialectJSONExtension DialectJSONStyle = "extension"
)

// DialectCapabilities 描述数据库方言的可用能力，不绑定具体 driver。
type DialectCapabilities struct {
	DBType                     DbType
	Placeholder                DialectPlaceholderStyle
	IdentifierQuote            DialectIdentifierQuoteStyle
	LimitOffset                bool
	LimitOffsetRequiresOrderBy bool
	GeneratedKey               DialectGeneratedKeyStyle
	Upsert                     DialectUpsertStyle
	RowLock                    DialectRowLockStyle
	JSON                       DialectJSONStyle
	BatchInsert                bool
	Savepoint                  bool
	SkipLocked                 bool
	NoWait                     bool
}

// SupportsGeneratedKey 返回方言是否声明了主键回读能力。
func (c DialectCapabilities) SupportsGeneratedKey() bool {
	return c.GeneratedKey != DialectGeneratedKeyNone
}

// SupportsUpsert 返回方言是否声明了原生 UPSERT 能力。
func (c DialectCapabilities) SupportsUpsert() bool {
	return c.Upsert != DialectUpsertNone
}

// SupportsRowLock 返回方言是否声明了行锁能力。
func (c DialectCapabilities) SupportsRowLock() bool {
	return c.RowLock != DialectRowLockNone
}

// SupportsJSON 返回方言是否声明了 JSON 能力。
func (c DialectCapabilities) SupportsJSON() bool {
	return c.JSON != DialectJSONNone
}

// DialectCapabilitiesProvider 由支持能力查询的方言可选实现。
type DialectCapabilitiesProvider interface {
	DialectCapabilities() DialectCapabilities
}

// NewDialectCapabilities 根据数据库类型返回内置方言能力。
func NewDialectCapabilities(dbType DbType) (DialectCapabilities, error) {
	switch dbType {
	case "", DbTypeQuestion:
		return questionDialectCapabilities(), nil
	case DbTypePostgres:
		return postgresDialectCapabilities(), nil
	case DbTypeMySQL:
		return mysqlDialectCapabilities(DbTypeMySQL), nil
	case DbTypeMariaDB:
		return mysqlDialectCapabilities(DbTypeMariaDB), nil
	case DbTypeSQLite:
		return sqliteDialectCapabilities(), nil
	case DbTypeSQLServer:
		return sqlServerDialectCapabilities(), nil
	case DbTypeOracle:
		return oracleDialectCapabilities(), nil
	default:
		return DialectCapabilities{}, fmt.Errorf("goark-orm: unsupported db type %q", dbType)
	}
}

// DialectCapabilitiesOf 返回方言能力；未知自定义方言只保留名称。
func DialectCapabilitiesOf(dialect Dialect) DialectCapabilities {
	if dialect == nil {
		return DialectCapabilities{}
	}
	if provider, ok := dialect.(DialectCapabilitiesProvider); ok {
		return provider.DialectCapabilities()
	}
	dbType, err := ParseDbType(dialect.Name())
	if err == nil {
		capabilities, capErr := NewDialectCapabilities(dbType)
		if capErr == nil {
			return capabilities
		}
	}
	return DialectCapabilities{DBType: DbType(strings.TrimSpace(dialect.Name()))}
}

func (questionDialect) DialectCapabilities() DialectCapabilities {
	return questionDialectCapabilities()
}

func (postgresDialect) DialectCapabilities() DialectCapabilities {
	return postgresDialectCapabilities()
}

func (d mysqlDialect) DialectCapabilities() DialectCapabilities {
	dbType := DbType(d.name)
	if dbType != DbTypeMariaDB {
		dbType = DbTypeMySQL
	}
	return mysqlDialectCapabilities(dbType)
}

func (sqliteDialect) DialectCapabilities() DialectCapabilities {
	return sqliteDialectCapabilities()
}

func (sqlServerDialect) DialectCapabilities() DialectCapabilities {
	return sqlServerDialectCapabilities()
}

func (oracleDialect) DialectCapabilities() DialectCapabilities {
	return oracleDialectCapabilities()
}

func questionDialectCapabilities() DialectCapabilities {
	return DialectCapabilities{
		DBType:          DbTypeQuestion,
		Placeholder:     DialectPlaceholderQuestion,
		IdentifierQuote: DialectIdentifierQuoteBacktick,
		LimitOffset:     true,
		BatchInsert:     true,
		Savepoint:       true,
	}
}

func postgresDialectCapabilities() DialectCapabilities {
	return DialectCapabilities{
		DBType:          DbTypePostgres,
		Placeholder:     DialectPlaceholderDollarNumber,
		IdentifierQuote: DialectIdentifierQuoteDouble,
		LimitOffset:     true,
		GeneratedKey:    DialectGeneratedKeyReturning,
		Upsert:          DialectUpsertOnConflict,
		RowLock:         DialectRowLockForUpdate,
		JSON:            DialectJSONNative,
		BatchInsert:     true,
		Savepoint:       true,
		SkipLocked:      true,
		NoWait:          true,
	}
}

func mysqlDialectCapabilities(dbType DbType) DialectCapabilities {
	return DialectCapabilities{
		DBType:          dbType,
		Placeholder:     DialectPlaceholderQuestion,
		IdentifierQuote: DialectIdentifierQuoteBacktick,
		LimitOffset:     true,
		GeneratedKey:    DialectGeneratedKeyLastInsertID,
		Upsert:          DialectUpsertOnDuplicateKey,
		RowLock:         DialectRowLockForUpdate,
		JSON:            DialectJSONNative,
		BatchInsert:     true,
		Savepoint:       true,
	}
}

func sqliteDialectCapabilities() DialectCapabilities {
	return DialectCapabilities{
		DBType:          DbTypeSQLite,
		Placeholder:     DialectPlaceholderQuestion,
		IdentifierQuote: DialectIdentifierQuoteDouble,
		LimitOffset:     true,
		GeneratedKey:    DialectGeneratedKeyLastInsertID,
		Upsert:          DialectUpsertOnConflict,
		JSON:            DialectJSONExtension,
		BatchInsert:     true,
		Savepoint:       true,
	}
}

func sqlServerDialectCapabilities() DialectCapabilities {
	return DialectCapabilities{
		DBType:                     DbTypeSQLServer,
		Placeholder:                DialectPlaceholderAtPNumber,
		IdentifierQuote:            DialectIdentifierQuoteBracket,
		LimitOffset:                true,
		LimitOffsetRequiresOrderBy: true,
		GeneratedKey:               DialectGeneratedKeyOutput,
		Upsert:                     DialectUpsertMerge,
		RowLock:                    DialectRowLockHints,
		JSON:                       DialectJSONNative,
		BatchInsert:                true,
		Savepoint:                  true,
	}
}

func oracleDialectCapabilities() DialectCapabilities {
	return DialectCapabilities{
		DBType:          DbTypeOracle,
		Placeholder:     DialectPlaceholderColonNumber,
		IdentifierQuote: DialectIdentifierQuoteDouble,
		LimitOffset:     true,
		GeneratedKey:    DialectGeneratedKeyReturningInto,
		Upsert:          DialectUpsertMerge,
		RowLock:         DialectRowLockForUpdate,
		JSON:            DialectJSONNative,
		BatchInsert:     true,
		Savepoint:       true,
		NoWait:          true,
	}
}
