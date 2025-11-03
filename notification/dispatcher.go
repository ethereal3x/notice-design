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
	queue    MessageQueue
	handlers map[EventType]EventHandler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	closed   bool
}

// NewEventDispatcher 初始化事件分发器（使用默认channel队列）
func NewEventDispatcher(ctx context.Context, bufferSize int) *EventDispatcher {
	return NewEventDispatcherWithQueue(ctx, NewChannelQueue(bufferSize))
}

// NewEventDispatcherWithQueue 使用指定队列初始化事件分发器
func NewEventDispatcherWithQueue(ctx context.Context, queue MessageQueue) *EventDispatcher {
	ctx, cancel := context.WithCancel(ctx)
	return &EventDispatcher{
		queue:    queue,
		handlers: make(map[EventType]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
		closed:   false,
	}
}

// NewEventDispatcherWithConfig 使用配置初始化事件分发器
func NewEventDispatcherWithConfig(ctx context.Context, config *QueueConfig) (*EventDispatcher, error) {
	queue, err := NewMessageQueue(config)
	if err != nil {
		return nil, err
	}
	return NewEventDispatcherWithQueue(ctx, queue), nil
}

func (d *EventDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return
	}

	// 使用MessageQueue接口的Push方法
	err := d.queue.Push(d.ctx, event, 5*time.Second)
	if err != nil {
		logger.ContextError(d.ctx, "EventDispatcher.Dispatch: failed to push event",
			zap.String("event_type", string(event.GetType())),
			zap.Int64("account_id", event.GetAccountID()),
			zap.Error(err))
	} else {
		logger.ContextDebug(d.ctx, "EventDispatcher.Dispatch: event dispatched",
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
	d.queue.Close()
	logger.ContextDebug(d.ctx, "EventDispatcher.Stop: all workers stopped")
}

func (d *EventDispatcher) worker(id int) {
	defer d.wg.Done()
	logger.ContextDebug(d.ctx, "EventDispatcher.worker started", zap.Int("id", id))

	for {
		select {
		case <-d.ctx.Done():
			logger.ContextDebug(d.ctx, "EventDispatcher.worker stopped", zap.Int("id", id))
			return
		default:
			event, err := d.queue.Pop(d.ctx)
			if err != nil {
				if d.ctx.Err() != nil {
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			d.handleEvent(event)
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

	maxRetries := 3
	var lastErr error
	// 重试
	for attempt := 1; attempt <= maxRetries; attempt++ {
		start := time.Now()
		err := handler.Handle(event)
		duration := time.Since(start)
		if err == nil {
			logger.ContextDebug(d.ctx, "EventDispatcher.handleEvent: handle event success",
				zap.String("event_type", string(event.GetType())),
				zap.Int64("account_id", event.GetAccountID()),
				zap.Int("attempt", attempt),
				zap.Duration("duration", duration))
			return
		}
		lastErr = err
		if attempt < maxRetries {
			backoff := time.Duration(attempt) * time.Second
			logger.ContextWarn(d.ctx, "EventDispatcher.handleEvent: handle event failed, will retry",
				zap.String("event_type", string(event.GetType())),
				zap.Int64("account_id", event.GetAccountID()),
				zap.Int("attempt", attempt),
				zap.Int("max_retries", maxRetries),
				zap.Duration("backoff", backoff),
				zap.Error(err))
			time.Sleep(backoff)
		} else {
			logger.ContextError(d.ctx, "EventDispatcher.handleEvent: handle event failed after all retries - MESSAGE LOST",
				zap.String("event_type", string(event.GetType())),
				zap.Int64("account_id", event.GetAccountID()),
				zap.Int("total_attempts", maxRetries),
				zap.Duration("total_duration", duration),
				zap.Error(lastErr),
				zap.Stack("stack_trace"))
		}
	}
}

func (d *EventDispatcher) GetEventChannelLen() int {
	return d.queue.Len()
}

func (d *EventDispatcher) GetEventChanCap() int {
	return d.queue.Cap()
}
