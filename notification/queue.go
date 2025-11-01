package notification

import (
	"context"
	"time"
)

// MessageQueue 消息队列接口抽象
type MessageQueue interface {
	// Push 推送事件到队列，如果队列满或超时返回错误
	Push(ctx context.Context, event Event, timeout time.Duration) error

	// Pop 从队列获取事件，阻塞直到有事件或context取消
	Pop(ctx context.Context) (Event, error)

	// Close 关闭队列
	Close() error

	// Len 获取队列当前长度
	Len() int

	// Cap 获取队列容量
	Cap() int
}

// QueueType 队列类型
type QueueType string

const (
	QueueTypeChannel QueueType = "channel" // 基于Go channel的内存队列
	QueueTypeRedis   QueueType = "redis"   // 基于Redis的队列
	QueueTypeKafka   QueueType = "kafka"   // 基于Kafka的队列
)

// QueueConfig 队列配置
type QueueConfig struct {
	Type       QueueType              // 队列类型
	BufferSize int                    // 缓冲区大小
	Extra      map[string]interface{} // 额外配置（如Redis地址、Kafka配置等）
}

// NewMessageQueue 根据配置创建消息队列
func NewMessageQueue(config *QueueConfig) (MessageQueue, error) {
	switch config.Type {
	case QueueTypeChannel:
		return NewChannelQueue(config.BufferSize), nil
	case QueueTypeRedis:
		return NewRedisQueue(config)
	case QueueTypeKafka:
		return NewKafkaQueue(config)
	default:
		// 默认使用channel队列
		return NewChannelQueue(config.BufferSize), nil
	}
}
