# 消息队列扩展使用指南

## 设计概述

系统已经抽象出 `MessageQueue` 接口，支持多种消息队列实现：
- **Channel Queue** - 基于Go channel的内存队列（已实现）
- **Redis Queue** - 基于Redis的持久化队列（接口已预留）
- **Kafka Queue** - 基于Kafka的分布式消息队列（接口已预留）

## 当前实现：Channel Queue（默认）

### 使用方式

```go
// 方式1: 使用默认的channel队列（推荐）
dispatcher := notification.NewEventDispatcher(ctx, 1000)
dispatcher.Start(5)

// 方式2: 显式创建channel队列
queue := notification.NewChannelQueue(1000)
dispatcher := notification.NewEventDispatcherWithQueue(ctx, queue)
dispatcher.Start(5)

// 方式3: 使用配置创建
config := &notification.QueueConfig{
    Type:       notification.QueueTypeChannel,
    BufferSize: 1000,
}
dispatcher, err := notification.NewEventDispatcherWithConfig(ctx, config)
if err != nil {
    log.Fatal(err)
}
dispatcher.Start(5)
```

### 特点

✅ **优点:**
- 零依赖，无需外部服务
- 性能最高，内存操作
- 实现简单，易于调试

❌ **缺点:**
- 无法持久化，进程重启丢失数据
- 无法跨进程/跨机器共享
- 内存受限

### 适用场景
- 单机部署
- QPS < 1000
- 数据丢失可接受
- 开发测试环境

---

## Redis Queue（待实现）

### 设计思路

使用Redis的List数据结构实现队列：
- `LPUSH` - 生产者推送消息
- `BRPOP` - 消费者阻塞式获取消息
- `LLEN` - 获取队列长度

### 使用方式

```go
config := &notification.QueueConfig{
    Type:       notification.QueueTypeRedis,
    BufferSize: 5000, // 队列最大长度限制
    Extra: map[string]interface{}{
        "addr":      "localhost:6379",
        "password":  "",
        "db":        0,
        "queue_key": "notification:events",
    },
}

dispatcher, err := notification.NewEventDispatcherWithConfig(ctx, config)
if err != nil {
    log.Fatal(err)
}
dispatcher.Start(10) // 可以启动更多worker
```

### 实现要点

1. **依赖添加**
```bash
go get github.com/redis/go-redis/v9
```

2. **连接池配置**
```go
client := redis.NewClient(&redis.Options{
    Addr:         config.Addr,
    Password:     config.Password,
    DB:           config.DB,
    PoolSize:     10,
    MinIdleConns: 5,
})
```

3. **消息序列化**
- 使用JSON序列化Event对象
- 在消息中包含事件类型信息用于反序列化

4. **错误处理**
- Redis连接断开时的重连机制
- 消息序列化失败的处理
- 长时间无消息时的心跳保持

### 特点

✅ **优点:**
- 消息持久化，重启不丢失
- 支持多进程消费（通过不同的消费者组）
- 可以跨机器部署
- 支持消息积压查询

❌ **缺点:**
- 需要额外的Redis服务
- 网络开销，性能略低于channel
- 需要考虑序列化/反序列化开销

### 适用场景
- 多实例部署
- QPS 500-5000
- 需要消息持久化
- 可以接受轻微性能损失

---

## Kafka Queue（待实现）

### 设计思路

使用Kafka的Topic实现消息队列：
- Producer - 发送消息到Topic
- Consumer Group - 消费者组消费消息
- Partition - 支持并行消费

### 使用方式

```go
config := &notification.QueueConfig{
    Type:       notification.QueueTypeKafka,
    BufferSize: 10000,
    Extra: map[string]interface{}{
        "brokers":  []string{"localhost:9092"},
        "topic":    "notification-events",
        "group_id": "notification-service",
        "version":  "2.8.0",
    },
}

dispatcher, err := notification.NewEventDispatcherWithConfig(ctx, config)
if err != nil {
    log.Fatal(err)
}
dispatcher.Start(20) // Kafka支持更多并发消费
```

### 实现要点

1. **依赖添加**
```bash
go get github.com/IBM/sarama
# 或者使用
go get github.com/confluentinc/confluent-kafka-go/v2/kafka
```

2. **生产者配置**
```go
config := sarama.NewConfig()
config.Producer.Return.Successes = true
config.Producer.RequiredAcks = sarama.WaitForAll // 最高可靠性
config.Producer.Retry.Max = 3
config.Producer.Compression = sarama.CompressionSnappy
```

3. **消费者配置**
```go
config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
config.Consumer.Offsets.Initial = sarama.OffsetNewest
config.Consumer.Return.Errors = true
```

4. **分区策略**
- 使用AccountID作为partition key，保证同一用户的消息顺序
- 根据QPS需求调整partition数量

5. **错误处理**
- 消费者组rebalance处理
- 消息消费失败的重试机制
- Offset提交策略

### 特点

✅ **优点:**
- 高吞吐量，支持大规模并发
- 消息持久化且支持回溯
- 支持水平扩展
- 完善的监控和管理工具

❌ **缺点:**
- 需要部署Kafka集群，运维复杂
- 学习成本较高
- 资源消耗较大
- 可能存在消息延迟

### 适用场景
- 大规模分布式部署
- QPS > 5000
- 需要消息审计和回溯
- 已有Kafka基础设施

---

## 性能对比

| 队列类型 | QPS | 延迟 | 持久化 | 扩展性 | 复杂度 |
|---------|-----|------|--------|--------|--------|
| Channel | ~10k | <1ms | ❌ | 低 | 低 |
| Redis   | ~5k | 1-5ms | ✅ | 中 | 中 |
| Kafka   | ~50k+ | 5-20ms | ✅ | 高 | 高 |

## 迁移指南

### 从Channel迁移到Redis

1. **部署Redis**
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

2. **修改配置**
```go
// 在main.go中修改
config := &notification.QueueConfig{
    Type:       notification.QueueTypeRedis,
    BufferSize: 2000,
    Extra: map[string]interface{}{
        "addr":      os.Getenv("REDIS_ADDR"),
        "password":  os.Getenv("REDIS_PASSWORD"),
        "db":        0,
        "queue_key": "notification:events",
    },
}
dispatcher, err := notification.NewEventDispatcherWithConfig(ctx, config)
```

3. **监控队列**
```bash
# 查看队列长度
redis-cli LLEN notification:events

# 查看队列内容
redis-cli LRANGE notification:events 0 10
```

### 从Redis迁移到Kafka

1. **部署Kafka**
```bash
# 使用docker-compose
docker-compose up -d kafka
```

2. **创建Topic**
```bash
kafka-topics --create --topic notification-events \
  --partitions 10 \
  --replication-factor 3 \
  --bootstrap-server localhost:9092
```

3. **修改配置**
```go
config := &notification.QueueConfig{
    Type:       notification.QueueTypeKafka,
    BufferSize: 5000,
    Extra: map[string]interface{}{
        "brokers":  []string{os.Getenv("KAFKA_BROKERS")},
        "topic":    "notification-events",
        "group_id": "notification-service",
    },
}
```

## 扩展新的队列实现

1. **实现MessageQueue接口**
```go
type CustomQueue struct {
    // 自定义字段
}

func (q *CustomQueue) Push(ctx context.Context, event Event, timeout time.Duration) error {
    // 实现推送逻辑
}

func (q *CustomQueue) Pop(ctx context.Context) (Event, error) {
    // 实现获取逻辑
}

func (q *CustomQueue) Close() error {
    // 实现关闭逻辑
}

func (q *CustomQueue) Len() int {
    // 实现长度获取
}

func (q *CustomQueue) Cap() int {
    // 实现容量获取
}
```

2. **注册到工厂函数**
```go
// 在queue.go中添加
case QueueTypeCustom:
    return NewCustomQueue(config)
```

3. **使用新队列**
```go
config := &notification.QueueConfig{
    Type: QueueTypeCustom,
    // ...
}
```

## 最佳实践

1. **选择合适的队列类型**
   - 开发环境：Channel
   - 小规模生产：Redis
   - 大规模生产：Kafka

2. **配置合适的缓冲区大小**
   - 根据QPS和处理时间计算
   - 留有余量应对突发流量

3. **监控关键指标**
   - 队列长度
   - 消费延迟
   - 失败率

4. **优雅降级**
   - 队列满时的处理策略
   - 消费失败的重试机制
   - 降级到同步处理

5. **测试切换**
   - 先在测试环境验证
   - 使用feature flag控制切换
   - 准备回滚方案

