package orm

import (
	"strings"
	"time"
)

// ResultSetType 描述 Statement 期望的结果集游标类型。
type ResultSetType string

const (
	// ResultSetTypeForwardOnly 表示只向前读取的结果集。
	ResultSetTypeForwardOnly ResultSetType = "FORWARD_ONLY"
	// ResultSetTypeScrollInsensitive 表示可滚动且不感知底层数据变化的结果集。
	ResultSetTypeScrollInsensitive ResultSetType = "SCROLL_INSENSITIVE"
	// ResultSetTypeScrollSensitive 表示可滚动且感知底层数据变化的结果集。
	ResultSetTypeScrollSensitive ResultSetType = "SCROLL_SENSITIVE"
)

// StatementOptions 描述语句级执行选项。
type StatementOptions struct {
	Timeout       time.Duration
	FetchSize     int
	ResultSetType ResultSetType
	ResultOrdered bool
	KeyColumn     string
}

// ParseResultSetType 解析 resultSetType 配置值。
func ParseResultSetType(value string) (ResultSetType, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case string(ResultSetTypeForwardOnly):
		return ResultSetTypeForwardOnly, nil
	case string(ResultSetTypeScrollInsensitive):
		return ResultSetTypeScrollInsensitive, nil
	case string(ResultSetTypeScrollSensitive):
		return ResultSetTypeScrollSensitive, nil
	default:
		return "", configurationErrorf("resultSetType %q is invalid", value)
	}
}

func (o StatementOptions) withDefaults(timeout time.Duration, fetchSize int) StatementOptions {
	if o.Timeout <= 0 {
		o.Timeout = timeout
	}
	if o.FetchSize <= 0 {
		o.FetchSize = fetchSize
	}
	return o
}

func (o StatementOptions) isZero() bool {
	return o.Timeout <= 0 &&
		o.FetchSize <= 0 &&
		o.ResultSetType == "" &&
		!o.ResultOrdered &&
		strings.TrimSpace(o.KeyColumn) == ""
}
