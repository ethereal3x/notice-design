package notification

import (
	"context"
	"time"
)

type EventType string

type EventHandler interface {
	Handle(event Event) error
	SupportEventType() EventType
}

const (
	EventTypeManuscript EventType = "manuscript"
	EventTypeAward      EventType = "award"
)

type Event interface {
	GetType() EventType
	GetAccountID() int64
	GetContext() context.Context
	GetTimeStamp() time.Time
}

type BaseEvent struct {
	Type    EventType       `json:"type"`
	Account int64           `json:"account"`
	Ctx     context.Context `json:"ctx"`
	Time    time.Time       `json:"time"`
}

func (e BaseEvent) GetType() EventType {
	return e.Type
}

func (e BaseEvent) GetAccountID() int64 {
	return e.Account
}

func (e BaseEvent) GetContext() context.Context {
	return e.Ctx
}

func (e BaseEvent) GetTimeStamp() time.Time {
	return e.Time
}

type ManuscriptEvent struct {
	BaseEvent
	ManuscriptId string `json:"manuscript_id"`
	OldStatus    int8   `json:"old_status"`
	NewStatus    int8   `json:"new_status"`
	AuditReason  string `json:"audit_reason"`
	OperateUser  string `json:"operate_user"`
	ActivityName string `json:"activity_name"`
}

type AwardEvent struct {
	BaseEvent
	ManuscriptId string `json:"manuscript_id"`
	AwardType    string `json:"award_type"`
	AwardAmount  int    `json:"award_amount"`
	ActivityName string `json:"activity_name"`
}

func NewManuscriptAuditEvent(ctx context.Context, accountId int64, manuscriptId string, oldStatus int8, newStatus int8) *ManuscriptEvent {
	return &ManuscriptEvent{
		BaseEvent: BaseEvent{
			Type:    EventTypeManuscript,
			Account: accountId,
			Ctx:     ctx,
			Time:    time.Now(),
		},
		ManuscriptId: manuscriptId,
		OldStatus:    oldStatus,
		NewStatus:    newStatus,
	}
}

func NewAwardEvent(ctx context.Context, accountId int64, manuscriptId string, awardAmount int, awardType string) *AwardEvent {
	return &AwardEvent{
		BaseEvent: BaseEvent{
			Type:    EventTypeAward,
			Account: accountId,
			Ctx:     ctx,
			Time:    time.Now(),
		},
		ManuscriptId: manuscriptId,
		AwardType:    awardType,
		AwardAmount:  awardAmount,
	}
}
