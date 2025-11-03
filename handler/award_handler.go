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

type AwardHandler struct {
	repo *repo.NoticeRepository
}

func NewAwardHandler(repo *repo.NoticeRepository) *AwardHandler {
	return &AwardHandler{repo: repo}
}

func (a *AwardHandler) SupportEventType() notification.EventType {
	return notification.EventTypeAward
}

func (a *AwardHandler) Handle(event notification.Event) error {
	awardEvent, ok := event.(*notification.AwardEvent)
	if !ok {
		return nil
	}
	ctx := awardEvent.GetContext()

	title := a.buildTitle(awardEvent)
	content := a.buildContent(awardEvent)
	ext := map[string]interface{}{
		"manuscript_id": awardEvent.ManuscriptId,
		"award_type":    awardEvent.AwardType,
		"award_amount":  awardEvent.AwardAmount,
		"activity_name": awardEvent.ActivityName,
	}
	extDataJSON, err := json.Marshal(ext)
	if err != nil {
		logger.ContextError(ctx, "AwardHandler: failed to marshal ext data",
			zap.String("manuscript_id", awardEvent.ManuscriptId),
			zap.Int64("account_id", awardEvent.GetAccountID()),
			zap.String("award_type", awardEvent.AwardType),
			zap.Int("award_amount", awardEvent.AwardAmount),
			zap.Error(err))
		return fmt.Errorf("marshal ext data failed: %w", err)
	}

	n := &repo.Notification{
		AccountID: event.GetAccountID(),
		Type:      constants.NOTIFICATION_TYPE_REWARD_DISTRIBUTE,
		Title:     title,
		Content:   content,
		Status:    constants.NOTIFICATION_STATUS_UNREAD,
		ExtData:   string(extDataJSON),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 记录即将插入的通知信息
	logger.ContextDebug(ctx, "AwardHandler: inserting notification",
		zap.String("manuscript_id", awardEvent.ManuscriptId),
		zap.Int64("account_id", n.AccountID),
		zap.Int8("type", n.Type),
		zap.String("title", n.Title),
		zap.String("content", n.Content))

	// 执行数据库插入
	err = a.repo.InsertNotice(ctx, n)
	if err != nil {
		logger.ContextError(ctx, "AwardHandler: DATABASE INSERT FAILED - MESSAGE WILL BE LOST",
			zap.String("event_type", "award"),
			zap.String("manuscript_id", awardEvent.ManuscriptId),
			zap.Int64("account_id", n.AccountID),
			zap.String("award_type", awardEvent.AwardType),
			zap.Int("award_amount", awardEvent.AwardAmount),
			zap.String("activity_name", awardEvent.ActivityName),
			zap.String("title", n.Title),
			zap.String("content", n.Content),
			zap.String("ext_data", n.ExtData),
			zap.Error(err),
			zap.String("error_type", fmt.Sprintf("%T", err)),
			zap.Stack("stack_trace"))
		return fmt.Errorf("insert notification failed: %w", err)
	}

	logger.ContextDebug(ctx, "AwardHandler: notification inserted successfully",
		zap.String("manuscript_id", awardEvent.ManuscriptId),
		zap.Int64("account_id", n.AccountID),
		zap.Uint64("notification_id", n.ID))

	return nil
}

func (a *AwardHandler) buildTitle(event *notification.AwardEvent) string {
	return "奖励发放通知"
}

func (a *AwardHandler) buildContent(event *notification.AwardEvent) string {
	content := fmt.Sprintf("恭喜您！您在活动【%s】的稿件获得奖励", event.ActivityName)
	if event.AwardAmount > 0 {
		content += fmt.Sprintf("，金额：%d", event.AwardAmount)
	}
	if event.AwardType != "" {
		content += fmt.Sprintf("，类型：%s", event.AwardType)
	}
	return content
}
