package account

import (
	"context"
	"time"

	orm "goark.dev/orm"
)

type auditFillHandler struct {
	now func() time.Time
}

// NewAuditFillHandler 创建账号模块的审计字段自动填充处理器。
func NewAuditFillHandler(now func() time.Time) orm.MetaObjectHandler {
	if now == nil {
		now = time.Now
	}
	return auditFillHandler{now: now}
}

func (h auditFillHandler) InsertFill(ctx context.Context, meta *orm.MetaObject) error {
	_ = ctx
	now := h.now()
	if err := meta.StrictInsertFill("CreatedAt", now); err != nil {
		return err
	}
	return meta.StrictInsertFill("UpdatedAt", now)
}

func (h auditFillHandler) UpdateFill(ctx context.Context, meta *orm.MetaObject) error {
	_ = ctx
	return meta.StrictUpdateFill("UpdatedAt", h.now())
}
