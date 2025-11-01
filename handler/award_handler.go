package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereal3x/notice/constants"
	"github.com/ethereal3x/notice/notification"
	"github.com/ethereal3x/notice/repo"
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
	extDataJSON, _ := json.Marshal(ext)
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
	return a.repo.InsertNotice(ctx, n)
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
