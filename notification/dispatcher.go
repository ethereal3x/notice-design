package notification

import (
	"context"
	"sync"
	"time"

	"github.com/ethereal3x/apc/logger"
	"go.uber.org/zap"
)

// EventDispatcher 事件分发器
type EventDispatcher struct {
	eventChan chan Event
	handlers  map[EventType]EventHandler
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    bool
}

// NewEventDispatcher 初始化事件分发器
func NewEventDispatcher(ctx context.Context, bufferSize int) *EventDispatcher {
	ctx, cancel := context.WithCancel(ctx)
	return &EventDispatcher{
		eventChan: make(chan Event, bufferSize),
		handlers:  make(map[EventType]EventHandler),
		ctx:       ctx,
		cancel:    cancel,
		closed:    false,
	}
}

func (d *EventDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		d.mu.RUnlock()
		return
	}
	select {
	case d.eventChan <- event:
		logger.ContextDebug(d.ctx, "EventDispatcher.Dispatch: event dispatched",
			zap.String("event_type", string(event.GetType())),
			zap.Int64("account_id", event.GetAccountID()))
	case <-time.After(5 * time.Second):
		logger.ContextError(d.ctx, "EventDispatcher.Dispatch: event channel is full, drop event",
			zap.String("event_type", string(event.GetType())),
			zap.Int64("account_id", event.GetAccountID()))
	}
}

func (d *EventDispatcher) RegisterHandler(handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[handler.SupportEventType()] = handler
	logger.ContextDebug(d.ctx, "EventDispatcher.RegisterHandler: register handler",
		zap.String("event_type", string(handler.SupportEventType())))
}

// Start 启动
func (d *EventDispatcher) Start(workerCount int) {
	for i := 0; i < workerCount; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}
}

// Stop 停止分发
func (d *EventDispatcher) Stop() {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	d.mu.Unlock()
	logger.ContextDebug(d.ctx, "EventDispatcher.Stop: waiting for all workers to stop")
	d.cancel()
	d.wg.Wait()
	close(d.eventChan)
	logger.ContextDebug(d.ctx, "EventDispatcher.Stop: all workers stopped")
}

func (d *EventDispatcher) worker(id int) {
	defer d.wg.Done()
	logger.ContextDebug(d.ctx, "EventDispatcher.worker", zap.Int("id", id))

	for {
		select {
		case event := <-d.eventChan:
			d.handleEvent(event)
		case <-d.ctx.Done():
			logger.ContextDebug(d.ctx, "EventDispatcher.worker", zap.Int("id", id))
			return
		}
	}
}

func (d *EventDispatcher) handleEvent(event Event) {
	defer func() {
		if r := recover(); r != nil {
			logger.ContextError(d.ctx, "EventDispatcher.handleEvent: panic",
				zap.String("event_type", string(event.GetType())),
				zap.Any("error", r))
		}
	}()

	d.mu.RLock()
	handler, exist := d.handlers[event.GetType()]
	d.mu.RUnlock()
	if !exist {
		logger.ContextError(d.ctx, "EventDispatcher.handleEvent: no handler for event",
			zap.String("event_type", string(event.GetType())),
			zap.Int64("account_id", event.GetAccountID()))
		return
	}

	start := time.Now()
	err := handler.Handle(event)
	duration := time.Since(start)
	if err != nil {
		logger.ContextError(d.ctx, "EventDispatcher.handleEvent: handle event error",
			zap.String("event_type", string(event.GetType())),
			zap.Int64("account_id", event.GetAccountID()),
			zap.Error(err))
	} else {
		logger.ContextDebug(d.ctx, "EventDispatcher.handleEvent: handle event",
			zap.String("event_type", string(event.GetType())),
			zap.Int64("account_id", event.GetAccountID()),
			zap.Duration("duration", duration))
	}
}

func (d *EventDispatcher) GetEventChannelLen() int {
	return len(d.eventChan)
}

func (d *EventDispatcher) GetEventChanCap() int {
	return cap(d.eventChan)
}
