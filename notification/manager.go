package notification

import (
	"context"
	"sync"

	"github.com/ethereal3x/apc/logger"
)

var (
	globalManager *Manager
	once          sync.Once
)

type Manager struct {
	dispatcher *EventDispatcher
	ctx        context.Context
}

// InitGlobalManager 全局管理初始化
func InitGlobalManager(ctx context.Context, dispatcher *EventDispatcher) {
	once.Do(func() {
		logger.ContextInfo(ctx, "Initializing global notification manager")
		globalManager = &Manager{
			dispatcher: dispatcher,
			ctx:        ctx,
		}
		logger.ContextInfo(ctx, "Global notification manager initialized")
	})
}

func GetGlobalManager() *Manager {
	return globalManager
}

func (m *Manager) Dispatcher(event Event) {
	if m.dispatcher != nil {
		m.dispatcher.Dispatch(event)
	}
}

func (m *Manager) Stop() {
	if m.dispatcher != nil {
		m.dispatcher.Stop()
	}
}

func (m *Manager) GetMetrics() map[string]interface{} {
	if m.dispatcher == nil {
		return nil
	}
	return map[string]interface{}{
		"event_channel_len": m.dispatcher.GetEventChannelLen(),
		"event_channel_cap": m.dispatcher.GetEventChanCap(),
	}
}

// DispatchManuscriptAuditEvent 分发稿件审核事件
func DispatchManuscriptAuditEvent(ctx context.Context, accountID int64, manuscriptID string, oldStatus, newStatus int8, auditReason, operateUser, activityName string) {
	if globalManager == nil {
		logger.ContextWarn(ctx, "DispatchManuscriptAuditEvent: global manager is not initialized")
		return
	}
	event := NewManuscriptAuditEvent(ctx, accountID, manuscriptID, oldStatus, newStatus)
	event.AuditReason = auditReason
	event.OperateUser = operateUser
	event.ActivityName = activityName
	globalManager.Dispatcher(event)
}

// DispatchAwardEvent 分发奖励发放事件
func DispatchAwardEvent(ctx context.Context, accountID int64, manuscriptID string, awardAmount int, awardType, activityName string) {
	if globalManager == nil {
		logger.ContextWarn(ctx, "DispatchAwardEvent: global manager is not initialized")
		return
	}
	event := NewAwardEvent(ctx, accountID, manuscriptID, awardAmount, awardType)
	event.ActivityName = activityName
	globalManager.Dispatcher(event)
}
