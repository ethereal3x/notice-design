package notification

import (
	"context"
	"errors"
	"time"
)

// ChannelQueue 基于Go channel的内存队列实现
type ChannelQueue struct {
	eventChan chan Event
	closed    bool
}

// NewChannelQueue 创建channel队列
func NewChannelQueue(bufferSize int) *ChannelQueue {
	return &ChannelQueue{
		eventChan: make(chan Event, bufferSize),
		closed:    false,
	}
}

// Push 推送事件到队列
func (q *ChannelQueue) Push(ctx context.Context, event Event, timeout time.Duration) error {
	if q.closed {
		return errors.New("queue is closed")
	}

	select {
	case q.eventChan <- event:
		return nil
	case <-time.After(timeout):
		return errors.New("push timeout: queue is full")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Pop 从队列获取事件
func (q *ChannelQueue) Pop(ctx context.Context) (Event, error) {
	select {
	case event, ok := <-q.eventChan:
		if !ok {
			return nil, errors.New("queue is closed")
		}
		return event, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close 关闭队列
func (q *ChannelQueue) Close() error {
	if q.closed {
		return nil
	}
	q.closed = true
	close(q.eventChan)
	return nil
}

// Len 获取队列当前长度
func (q *ChannelQueue) Len() int {
	return len(q.eventChan)
}

// Cap 获取队列容量
func (q *ChannelQueue) Cap() int {
	return cap(q.eventChan)
}
