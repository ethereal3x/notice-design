package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereal3x/notice/constants"
	"github.com/ethereal3x/notice/notification"
	"github.com/ethereal3x/notice/repo"
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
	extDataJSON, _ := json.Marshal(ext)
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
	return m.repo.InsertNotice(ctx, n)
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
