package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereal3x/apc/logger"
	"github.com/ethereal3x/notice/constants"
	"github.com/ethereal3x/notice/notification"
	"github.com/ethereal3x/notice/repo"
	"go.uber.org/zap"
)

type ManuscriptHandler struct {
	repo *repo.NoticeRepository
}

func NewManuscriptHandler(repo *repo.NoticeRepository) *ManuscriptHandler {
	return &ManuscriptHandler{repo: repo}
}

func (m *ManuscriptHandler) SupportEventType() notification.EventType {
	return notification.EventTypeManuscript
}

func (m *ManuscriptHandler) Handle(event notification.Event) error {
	auditEvent, ok := event.(*notification.ManuscriptEvent)
	if !ok {
		return nil
	}
	ctx := auditEvent.GetContext()
	if auditEvent.NewStatus == auditEvent.OldStatus {
		return nil
	}
	title := m.buildTitle(auditEvent)
	content := m.buildContent(auditEvent)
	ext := map[string]interface{}{
		"manuscript_id": auditEvent.ManuscriptId,
		"old_status":    auditEvent.OldStatus,
		"new_status":    auditEvent.NewStatus,
		"audit_reason":  auditEvent.AuditReason,
		"operate_user":  auditEvent.OperateUser,
		"activity_name": auditEvent.ActivityName,
	}
	extDataJSON, err := json.Marshal(ext)
	if err != nil {
		logger.ContextError(ctx, "ManuscriptHandler: failed to marshal ext data",
			zap.String("manuscript_id", auditEvent.ManuscriptId),
			zap.Int64("account_id", auditEvent.GetAccountID()),
			zap.Int8("old_status", auditEvent.OldStatus),
			zap.Int8("new_status", auditEvent.NewStatus),
			zap.Error(err))
		return fmt.Errorf("marshal ext data failed: %w", err)
	}

	n := &repo.Notification{
		AccountID: event.GetAccountID(),
		Type:      constants.NOTIFICATION_TYPE_MANUSCRIPT_AUDIT,
		Title:     title,
		Content:   content,
		Status:    constants.NOTIFICATION_STATUS_UNREAD,
		ExtData:   string(extDataJSON),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 记录即将插入的通知信息
	logger.ContextDebug(ctx, "ManuscriptHandler: inserting notification",
		zap.String("manuscript_id", auditEvent.ManuscriptId),
		zap.Int64("account_id", n.AccountID),
		zap.Int8("type", n.Type),
		zap.String("title", n.Title),
		zap.String("content", n.Content))

	// 执行数据库插入
	err = m.repo.InsertNotice(ctx, n)
	if err != nil {
		logger.ContextError(ctx, "ManuscriptHandler: DATABASE INSERT FAILED - MESSAGE WILL BE LOST",
			zap.String("event_type", "manuscript_audit"),
			zap.String("manuscript_id", auditEvent.ManuscriptId),
			zap.Int64("account_id", n.AccountID),
			zap.Int8("old_status", auditEvent.OldStatus),
			zap.Int8("new_status", auditEvent.NewStatus),
			zap.String("audit_reason", auditEvent.AuditReason),
			zap.String("operate_user", auditEvent.OperateUser),
			zap.String("activity_name", auditEvent.ActivityName),
			zap.String("title", n.Title),
			zap.String("content", n.Content),
			zap.String("ext_data", n.ExtData),
			zap.Error(err),
			zap.String("error_type", fmt.Sprintf("%T", err)),
			zap.Stack("stack_trace"))
		return fmt.Errorf("insert notification failed: %w", err)
	}
	logger.ContextDebug(ctx, "ManuscriptHandler: notification inserted successfully",
		zap.String("manuscript_id", auditEvent.ManuscriptId),
		zap.Int64("account_id", n.AccountID),
		zap.Uint64("notification_id", n.ID))

	return nil
}

func (m *ManuscriptHandler) buildTitle(event *notification.ManuscriptEvent) string {
	statusText := m.getStatusText(event.NewStatus)
	return fmt.Sprintf("稿件审核%s", statusText)
}

func (m *ManuscriptHandler) buildContent(event *notification.ManuscriptEvent) string {
	statusText := m.getStatusText(event.NewStatus)
	content := fmt.Sprintf("您在活动【%s】提交的稿件已%s", event.ActivityName, statusText)

	if event.NewStatus == constants.MANUSCRIPT_AUDIT_STATUS_REJECTED && event.AuditReason != "" {
		content += fmt.Sprintf("，原因：%s", event.AuditReason)
	}

	return content
}

func (m *ManuscriptHandler) getStatusText(status int8) string {
	switch status {
	case constants.MANUSCRIPT_AUDIT_STATUS_APPROVED:
		return "审核通过"
	case constants.MANUSCRIPT_AUDIT_STATUS_REJECTED:
		return "审核未通过"
	case constants.MANUSCRIPT_AUDIT_STATUS_PENDING:
		return "审核中"
	default:
		return "状态更新"
	}
}
