package orm

import (
	"fmt"
	"strconv"
	"strings"
)

// DbType 对齐 MyBatis-Plus 的数据库类型枚举，并保持 Go 字符串常量形式。
type DbType string

const (
	DbTypeQuestion  DbType = "question"
	DbTypePostgres  DbType = "postgres"
	DbTypeMySQL     DbType = "mysql"
	DbTypeMariaDB   DbType = "mariadb"
	DbTypeSQLite    DbType = "sqlite"
	DbTypeSQLServer DbType = "sqlserver"
	DbTypeOracle    DbType = "oracle"
)

// Dialect 封装数据库方言差异。
type Dialect interface {
	Name() string
	Placeholder(index int) string
	QuoteIdent(identifier string) string
}

// ParseDbType 解析常见数据库类型别名。
func ParseDbType(value string) (DbType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "question":
		return DbTypeQuestion, nil
	case "postgres", "postgresql", "pg":
		return DbTypePostgres, nil
	case "mysql":
		return DbTypeMySQL, nil
	case "mariadb":
		return DbTypeMariaDB, nil
	case "sqlite", "sqlite3":
		return DbTypeSQLite, nil
	case "sqlserver", "mssql":
		return DbTypeSQLServer, nil
	case "oracle":
		return DbTypeOracle, nil
	default:
		return "", fmt.Errorf("goark-orm: unsupported db type %q", value)
	}
}

// NewDialect 根据数据库类型创建方言实例。
func NewDialect(dbType DbType) (Dialect, error) {
	switch dbType {
	case "", DbTypeQuestion:
		return NewQuestionDialect(), nil
	case DbTypePostgres:
		return NewPostgresDialect(), nil
	case DbTypeMySQL:
		return NewMySQLDialect(), nil
	case DbTypeMariaDB:
		return NewMariaDBDialect(), nil
	case DbTypeSQLite:
		return NewSQLiteDialect(), nil
	case DbTypeSQLServer:
		return NewSQLServerDialect(), nil
	case DbTypeOracle:
		return NewOracleDialect(), nil
	default:
		return nil, fmt.Errorf("goark-orm: unsupported db type %q", dbType)
	}
}

type questionDialect struct{}

// NewQuestionDialect 创建使用问号占位符的通用方言。
func NewQuestionDialect() Dialect {
	return questionDialect{}
}

func (questionDialect) Name() string {
	return "question"
}

func (questionDialect) Placeholder(index int) string {
	return "?"
}

func (questionDialect) QuoteIdent(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func (questionDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	return query + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
}

type postgresDialect struct{}

// NewPostgresDialect 创建 PostgreSQL 方言。
func NewPostgresDialect() Dialect {
	return postgresDialect{}
}

func (postgresDialect) Name() string {
	return "postgres"
}

func (postgresDialect) Placeholder(index int) string {
	return "$" + strconv.Itoa(index)
}

func (postgresDialect) QuoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func (postgresDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	return query + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
}

type mysqlDialect struct {
	name string
}

// NewMySQLDialect 创建 MySQL 方言。
func NewMySQLDialect() Dialect {
	return mysqlDialect{name: string(DbTypeMySQL)}
}

// NewMariaDBDialect 创建 MariaDB 方言。
func NewMariaDBDialect() Dialect {
	return mysqlDialect{name: string(DbTypeMariaDB)}
}

func (d mysqlDialect) Name() string {
	return d.name
}

func (mysqlDialect) Placeholder(index int) string {
	return "?"
}

func (mysqlDialect) QuoteIdent(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func (mysqlDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	return query + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
}

type sqliteDialect struct{}

// NewSQLiteDialect 创建 SQLite 方言。
func NewSQLiteDialect() Dialect {
	return sqliteDialect{}
}

func (sqliteDialect) Name() string {
	return string(DbTypeSQLite)
}

func (sqliteDialect) Placeholder(index int) string {
	return "?"
}

func (sqliteDialect) QuoteIdent(identifier string) string {
	return quoteDoubleIdentifier(identifier)
}

func (sqliteDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	return query + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
}

type sqlServerDialect struct{}

// NewSQLServerDialect 创建 SQL Server 方言。
func NewSQLServerDialect() Dialect {
	return sqlServerDialect{}
}

func (sqlServerDialect) Name() string {
	return string(DbTypeSQLServer)
}

func (sqlServerDialect) Placeholder(index int) string {
	return "@p" + strconv.Itoa(index)
}

func (sqlServerDialect) QuoteIdent(identifier string) string {
	return "[" + strings.ReplaceAll(identifier, "]", "]]") + "]"
}

func (sqlServerDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	if !containsOrderBy(query) {
		query += " ORDER BY (SELECT 0)"
	}
	return query + " OFFSET " + offsetPlaceholder + " ROWS FETCH NEXT " + limitPlaceholder + " ROWS ONLY"
}

type oracleDialect struct{}

// NewOracleDialect 创建 Oracle 方言。
func NewOracleDialect() Dialect {
	return oracleDialect{}
}

func (oracleDialect) Name() string {
	return string(DbTypeOracle)
}

func (oracleDialect) Placeholder(index int) string {
	return ":" + strconv.Itoa(index)
}

func (oracleDialect) QuoteIdent(identifier string) string {
	return quoteDoubleIdentifier(identifier)
}

func (oracleDialect) LimitOffsetSQL(query string, limitPlaceholder string, offsetPlaceholder string) string {
	return query + " OFFSET " + offsetPlaceholder + " ROWS FETCH NEXT " + limitPlaceholder + " ROWS ONLY"
}

func quoteDoubleIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func containsOrderBy(query string) bool {
	depth := 0
	for index := 0; index < len(query); {
		if next, ok := skipSQLComment(query, index); ok {
			index = next
			continue
		}
		switch query[index] {
		case '\'':
			index = skipSQLSingleQuoted(query, index)
			continue
		case '#', '$':
			if next, ok := skipSQLPlaceholder(query, index); ok {
				index = next
				continue
			}
		case '"', '`':
			_, _, next, ok := readSQLQuotedIdentifier(query, index)
			if !ok {
				return false
			}
			index = next
			continue
		case '[':
			index = skipSQLBracketQuotedIdentifier(query, index)
			continue
		case '(':
			depth++
			index++
			continue
		case ')':
			if depth > 0 {
				depth--
			}
			index++
			continue
		}
		if depth == 0 && isSQLIdentStart(query[index]) {
			start := index
			for index < len(query) && isSQLIdentPart(query[index]) {
				index++
			}
			if strings.EqualFold(query[start:index], "order") {
				next := skipSQLSpacesAndComments(query, index)
				if hasSQLKeywordAt(query, next, "by") {
					return true
				}
			}
			continue
		}
		index++
	}
	return false
}
