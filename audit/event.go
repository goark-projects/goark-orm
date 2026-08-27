package audit

import (
	"time"

	orm "goark.dev/orm"
)

// Operation 表示审计事件对应的 ORM 调用入口。
type Operation string

const (
	// OperationQuery 表示多行查询入口。
	OperationQuery Operation = "query"
	// OperationQueryOne 表示单行查询入口。
	OperationQueryOne Operation = "queryOne"
	// OperationExec 表示写语句执行入口。
	OperationExec Operation = "exec"
)

// Event 描述一次 Statement 执行后的审计事件。
type Event struct {
	Statement    orm.StatementMeta
	Operation    Operation
	Command      orm.StatementCommand
	Namespace    string
	ID           string
	FullName     string
	AffectData   bool
	RowsAffected int64
	LastInsertID int64
	StartedAt    time.Time
	Duration     time.Duration
	Err          error
}

// Success 判断业务执行是否成功。
func (e Event) Success() bool {
	return e.Err == nil
}

func newEvent(operation Operation, meta orm.StatementMeta, result orm.Result, startedAt time.Time, err error) Event {
	return Event{
		Statement:    meta,
		Operation:    operation,
		Command:      meta.Command,
		Namespace:    meta.Namespace,
		ID:           meta.ID,
		FullName:     meta.FullName,
		AffectData:   meta.AffectData,
		RowsAffected: result.RowsAffected,
		LastInsertID: result.LastInsertID,
		StartedAt:    startedAt,
		Duration:     time.Since(startedAt),
		Err:          err,
	}
}
