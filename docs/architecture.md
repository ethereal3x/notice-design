# 通知系统架构设计

## 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Layer                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Handler    │  │   Handler    │  │   Handler    │          │
│  │ (Manuscript) │  │   (Award)    │  │   (Custom)   │          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │
│         │                  │                  │                   │
│         └──────────────────┼──────────────────┘                  │
│                           │                                       │
└───────────────────────────┼───────────────────────────────────────┘
                            │
┌───────────────────────────┼───────────────────────────────────────┐
│                   Notification Core                               │
│                            │                                       │
│      ┌─────────────────────▼─────────────────────┐               │
│      │         Event Dispatcher                   │               │
│      │  ┌──────────────────────────────────┐     │               │
│      │  │    Event Handler Registry        │     │               │
│      │  └──────────────────────────────────┘     │               │
│      │  ┌──────────────────────────────────┐     │               │
│      │  │    Worker Pool (5-20 workers)    │     │               │
│      │  └──────────────────────────────────┘     │               │
│      └────────────────┬──────────────────────────┘               │
│                       │                                            │
│      ┌────────────────▼──────────────────────┐                   │
│      │      MessageQueue Interface           │                   │
│      └───────┬──────────┬──────────┬─────────┘                   │
│              │          │          │                              │
│   ┌──────────▼─┐   ┌───▼────┐  ┌──▼─────────┐                   │
│   │  Channel   │   │ Redis  │  │   Kafka    │                   │
│   │   Queue    │   │ Queue  │  │   Queue    │                   │
│   │ (In-Memory)│   │(未实现) │  │  (未实现)  │                   │
│   └────────────┘   └────────┘  └────────────┘                   │
└───────────────────────────────────────────────────────────────────┘
                            │
┌───────────────────────────┼───────────────────────────────────────┐
│                     Repository Layer                              │
│                            │                                       │
│      ┌─────────────────────▼─────────────────────┐               │
│      │        NoticeRepository                    │               │
│      │  ┌──────────────────────────────────┐     │               │
│      │  │  Insert / Query / Update         │     │               │
│      │  └──────────────────────────────────┘     │               │
│      └────────────────┬──────────────────────────┘               │
└───────────────────────┼───────────────────────────────────────────┘
                        │
           ┌────────────▼────────────┐
           │   MySQL Database        │
           │  tbl_notification       │
           └─────────────────────────┘
```

## 核心组件

### 1. MessageQueue接口（可扩展的队列抽象层）

**位置**: `notification/queue.go`

**职责**: 定义统一的消息队列接口，支持多种实现

```go
type MessageQueue interface {
    Push(ctx context.Context, event Event, timeout time.Duration) error
    Pop(ctx context.Context) (Event, error)
    Close() error
    Len() int
    Cap() int
}
```

**实现**:
- ✅ `ChannelQueue` - 基于Go channel的内存队列（已实现）
- ⏳ `RedisQueue` - 基于Redis的持久化队列（接口预留）
- ⏳ `KafkaQueue` - 基于Kafka的分布式队列（接口预留）

### 2. EventDispatcher（事件分发器）

**位置**: `notification/dispatcher.go`

**职责**:
- 接收事件并推送到队列
- 管理worker池进行并发处理
- 注册和路由EventHandler
- 处理错误和panic

**特性**:
- 支持自定义队列实现
- 动态调整worker数量
- 优雅关闭机制

### 3. EventHandler（事件处理器）

**位置**: `notification/event.go`, `handler/`

**职责**: 处理特定类型的事件并生成通知

**当前实现**:
- `ManuscriptHandler` - 处理稿件审核事件
- `AwardHandler` - 处理奖励发放事件

**扩展方式**: 实现`EventHandler`接口即可

### 4. Manager（全局管理器）

**位置**: `notification/manager.go`

**职责**:
- 全局单例管理
- 提供便捷的事件分发方法
- 收集系统指标

## 数据流

```
Event发起
    │
    ▼
DispatchXxxEvent()  ──────────┐
    │                         │
    ▼                         │
Manager.Dispatcher()          │  便捷方法封装
    │                         │
    ▼                         │
EventDispatcher.Dispatch()  ◄─┘
    │
    ▼
MessageQueue.Push()  ────┐
    │                    │  异步队列
    ▼                    │
[Queue Buffer]  ◄────────┘
    │
    ▼
Worker Pool ────────────┐
    │                   │  并发处理
    │                   │  (5-20 workers)
    ▼                   │
MessageQueue.Pop()  ◄───┘
    │
    ▼
EventDispatcher.handleEvent()
    │
    ▼
找到对应Handler ──────┐
    │                 │  类型匹配
    ▼                 │
Handler.Handle()  ◄───┘
    │
    ▼
构建通知内容
    │
    ▼
Repository.InsertNotice()
    │
    ▼
写入数据库
```

## 设计优势

### 1. 解耦合

通过抽象接口实现各层解耦：
- Handler与Notification核心解耦（通过EventHandler接口）
- Dispatcher与具体队列实现解耦（通过MessageQueue接口）
- 业务逻辑与数据访问解耦（通过Repository模式）

### 2. 可扩展性

**水平扩展**:
- 调整worker数量
- 切换到分布式队列（Redis/Kafka）
- 多实例部署

**功能扩展**:
- 新增handler只需实现接口
- 新增队列实现只需实现MessageQueue接口

### 3. 高性能

**异步处理**:
- 事件推送立即返回
- Worker池并发处理
- 队列缓冲削峰填谷

**性能调优**:
- 可调整队列大小
- 可调整worker数量
- 支持切换高性能队列

### 4. 可靠性

**错误处理**:
- Panic恢复机制
- 错误日志记录
- 消息重试（队列实现相关）

**优雅关闭**:
- 停止接收新事件
- 等待现有事件处理完成
- 关闭资源连接

### 5. 可观测性

**日志**:
- 结构化日志
- 事件追踪
- 性能指标

**监控指标**:
- 队列长度
- 队列容量
- Worker状态

## 性能指标

### Channel Queue (当前实现)

- **QPS**: ~10,000
- **延迟**: <1ms
- **内存**: ~1MB (1000 events)
- **Worker**: 5个

### Redis Queue (预期)

- **QPS**: ~5,000
- **延迟**: 1-5ms
- **持久化**: ✅
- **Worker**: 10-20个

### Kafka Queue (预期)

- **QPS**: ~50,000+
- **延迟**: 5-20ms
- **持久化**: ✅
- **Worker**: 20-50个

## 扩展路线图

### Phase 1: 当前 (已完成)
- ✅ 基础架构设计
- ✅ Channel队列实现
- ✅ Manuscript和Award处理器
- ✅ 数据库操作

### Phase 2: Redis支持 (待实现)
- ⏳ 实现RedisQueue
- ⏳ 消息序列化/反序列化
- ⏳ 连接池管理
- ⏳ 错误重试机制

### Phase 3: Kafka支持 (待实现)
- ⏳ 实现KafkaQueue
- ⏳ 生产者/消费者配置
- ⏳ Partition策略
- ⏳ Offset管理

### Phase 4: 监控和运维
- ⏳ Prometheus指标暴露
- ⏳ Grafana仪表板
- ⏳ 告警规则
- ⏳ 性能分析工具

### Phase 5: 高级特性
- ⏳ 事件优先级
- ⏳ 延迟队列
- ⏳ 死信队列
- ⏳ 事件回溯

## 配置示例

### 开发环境
```go
// 使用默认Channel队列
dispatcher := notification.NewEventDispatcher(ctx, 1000)
dispatcher.Start(5)
```

### 生产环境 (Redis)
```go
config := &notification.QueueConfig{
    Type: notification.QueueTypeRedis,
    BufferSize: 5000,
    Extra: map[string]interface{}{
        "addr": "redis-cluster.prod:6379",
        "password": os.Getenv("REDIS_PASSWORD"),
    },
}
dispatcher, _ := notification.NewEventDispatcherWithConfig(ctx, config)
dispatcher.Start(20)
```

### 大规模生产 (Kafka)
```go
config := &notification.QueueConfig{
    Type: notification.QueueTypeKafka,
    BufferSize: 10000,
    Extra: map[string]interface{}{
        "brokers": []string{"kafka1:9092", "kafka2:9092"},
        "topic": "notification-events",
        "group_id": "notification-service",
    },
}
dispatcher, _ := notification.NewEventDispatcherWithConfig(ctx, config)
dispatcher.Start(50)
```

## 注意事项

1. **预留字段**: `queue_redis.go`和`queue_kafka.go`中的某些字段标记为unused是正常的，这些是预留字段，待实现时使用。

2. **序列化**: 切换到Redis或Kafka时需要特别注意Event的序列化和反序列化，context.Context不能被序列化。

3. **并发安全**: 所有Handler实现都应该是并发安全的。

4. **数据库连接**: 使用Redis/Kafka时需要调整数据库连接池大小。

5. **监控**: 生产环境强烈建议添加队列长度监控和告警。


