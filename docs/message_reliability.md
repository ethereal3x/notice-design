# 消息可靠性分析

## Channel队列消息丢失场景

### 场景1: 进程崩溃/重启 ⚠️ **最严重**

**发生条件**:
- 程序崩溃 (panic, SIGKILL)
- 服务重启/部署
- 系统宕机/断电
- OOM被kill

**丢失数据**:
```
队列中的消息: 最多1000条 (bufferSize)
正在处理的消息: 最多5条 (worker数量)
总计: 最多1005条
```

**概率**: ⭐⭐⭐⭐⭐ 极高（生产环境必然发生）

**示例**:
```go
// 时间线：
T0: 队列中有500条消息待处理
T1: 服务发版，执行 kubectl rollout restart
T2: 进程收到SIGTERM信号
T3: 执行优雅关闭，等待2秒
T4: 如果2秒内未处理完，强制kill
T5: 500条消息全部丢失 ❌
```

**解决方案**:
- ✅ 使用Redis/Kafka持久化队列
- ✅ 增加优雅关闭等待时间
- ✅ 限制队列大小，防止积压
- ⚠️ 接受数据丢失（如果业务允许）

---

### 场景2: 队列满导致丢弃 ⚠️ **常见**

**发生条件**:
- 突发流量
- Worker处理慢（数据库慢、网络慢）
- Worker数量不足

**当前实现**:
```go
func (q *ChannelQueue) Push(ctx, event, timeout) error {
    select {
    case q.eventChan <- event:
        return nil  // ✅ 成功
    case <-time.After(5 * time.Second):
        return errors.New("queue is full")  // ❌ 丢弃
    }
}
```

**计算丢失率**:
```
假设：
- QPS: 200
- Worker: 5个
- 每条消息处理时间: 50ms
- 队列容量: 1000

处理能力: 5 / 0.05 = 100 QPS
积压速度: 200 - 100 = 100 消息/秒
队列填满时间: 1000 / 100 = 10秒

结论: 10秒后开始丢消息
```

**概率**: ⭐⭐⭐⭐ 高（高负载时）

**监控指标**:
```go
func (m *Manager) GetMetrics() map[string]interface{} {
    queueLen := m.dispatcher.GetEventChannelLen()
    queueCap := m.dispatcher.GetEventChanCap()
    return map[string]interface{}{
        "queue_len": queueLen,
        "queue_cap": queueCap,
        "queue_usage_pct": float64(queueLen) / float64(queueCap) * 100,
    }
}

// 告警规则：
// queue_usage_pct > 80%  => Warning
// queue_usage_pct > 95%  => Critical
```

**解决方案**:
- ✅ 增加Worker数量
- ✅ 增大队列容量
- ✅ 优化处理速度
- ✅ 限流保护
- ✅ 降级处理（同步写入）

---

### 场景3: Handler处理失败无重试 ⚠️ **设计缺陷**

**当前实现**:
```go
func (h *ManuscriptHandler) Handle(event Event) error {
    // ...
    err := h.repo.InsertNotice(ctx, n)
    return err  // ❌ 错误后消息就丢了
}

func (d *EventDispatcher) handleEvent(event Event) {
    err := handler.Handle(event)
    if err != nil {
        logger.ContextError(...)  // 只记录日志
        // ❌ 没有重试！消息丢失！
    }
}
```

**可能的失败原因**:
- 数据库连接失败
- 数据库deadlock
- 网络超时
- 磁盘满了

**概率**: ⭐⭐⭐ 中（数据库问题时）

**改进方案**:

#### 方案A: 重试机制
```go
func (d *EventDispatcher) handleEvent(event Event) {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        err := handler.Handle(event)
        if err == nil {
            return  // ✅ 成功
        }
        
        // 指数退避
        backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
        time.Sleep(backoff)
        logger.ContextWarn(ctx, "retry handling event", 
            zap.Int("attempt", i+1))
    }
    
    // 重试失败，发送到死信队列
    d.sendToDeadLetterQueue(event)
}
```

#### 方案B: 死信队列
```go
type EventDispatcher struct {
    queue          MessageQueue
    deadLetterChan chan Event  // 死信队列
    // ...
}

func (d *EventDispatcher) sendToDeadLetterQueue(event Event) {
    select {
    case d.deadLetterChan <- event:
        logger.ContextError(ctx, "event sent to dead letter queue")
    default:
        logger.ContextError(ctx, "dead letter queue is full, event lost")
    }
}

// 后续可以：
// 1. 人工介入处理
// 2. 定时重试
// 3. 记录到数据库
```

---

### 场景4: 非优雅关闭 ⚠️ **运维问题**

**危险操作**:
```bash
# ❌ 强制kill
kill -9 <pid>

# ❌ 没有等待时间的重启
docker stop --time=0 container_name

# ❌ K8s的terminationGracePeriodSeconds太短
terminationGracePeriodSeconds: 5  # 太短了
```

**当前优雅关闭实现**:
```go
func waitForShutdown(ctx context.Context) {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    
    sig := <-quit
    logger.ContextInfo(ctx, "Shutting down...")
    
    manager := notification.GetGlobalManager()
    if manager != nil {
        manager.Stop()  // 停止接收新消息
    }
    
    time.Sleep(2 * time.Second)  // ⚠️ 只等2秒！
}
```

**问题**:
```
假设：
- 队列中有500条消息
- 每条处理50ms
- 5个worker

最坏情况处理时间: 500 / 5 * 0.05 = 5秒
当前等待时间: 2秒
丢失消息: ~300条
```

**改进方案**:
```go
func waitForShutdown(ctx context.Context) {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    
    sig := <-quit
    logger.ContextInfo(ctx, "Shutting down...")
    
    manager := notification.GetGlobalManager()
    if manager != nil {
        manager.Stop()  // 停止接收新消息
        
        // 等待队列清空
        timeout := time.After(30 * time.Second)
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-timeout:
                logger.ContextWarn(ctx, "shutdown timeout, force exit")
                return
            case <-ticker.C:
                metrics := manager.GetMetrics()
                queueLen := metrics["event_channel_len"].(int)
                if queueLen == 0 {
                    logger.ContextInfo(ctx, "queue is empty, exit gracefully")
                    return
                }
                logger.ContextInfo(ctx, "waiting for queue to drain",
                    zap.Int("remaining", queueLen))
            }
        }
    }
}
```

**K8s配置**:
```yaml
spec:
  template:
    spec:
      terminationGracePeriodSeconds: 60  # 增加到60秒
      containers:
      - name: notification-service
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 5"]  # 延迟5秒
```

---

### 场景5: 消息处理时间过长 ⚠️ **性能问题**

**示例**:
```go
func (h *ManuscriptHandler) Handle(event Event) error {
    // ❌ 糟糕的设计
    
    // 1. 数据库慢查询 (2秒)
    existingNotices := h.repo.FindDuplicates(event)
    
    // 2. 外部API调用 (3秒)
    userInfo := externalAPI.GetUserInfo(event.AccountID)
    
    // 3. 复杂业务逻辑 (1秒)
    content := h.buildComplexContent(event, userInfo)
    
    // 总耗时: 6秒/条
    return h.repo.InsertNotice(ctx, n)
}

// 结果：
// 5个worker，QPS只有: 5 / 6 ≈ 0.83
// 完全无法处理正常流量
```

**解决方案**:
- ✅ 异步处理耗时操作
- ✅ 缓存频繁查询的数据
- ✅ 批量处理
- ✅ 优化数据库查询

---

## 各场景对比表

| 场景 | 严重程度 | 发生频率 | 影响范围 | 可预防性 |
|-----|---------|---------|---------|---------|
| 进程崩溃 | ⭐⭐⭐⭐⭐ | 高 | 所有队列消息 | 低 |
| 队列满 | ⭐⭐⭐⭐ | 中高 | 新消息 | 高 |
| 处理失败 | ⭐⭐⭐ | 中 | 单条消息 | 高 |
| 非优雅关闭 | ⭐⭐⭐⭐ | 低 | 队列消息 | 高 |
| 处理慢 | ⭐⭐⭐ | 中 | 间接丢失 | 高 |

## 解决方案对比

### 方案1: 接受丢失（当前）✅

**适用场景**:
- 通知类消息（丢了不影响核心业务）
- 开发/测试环境
- 用户量小

**优点**:
- 实现简单
- 无外部依赖
- 性能最好

**缺点**:
- 消息可能丢失
- 无法追溯

---

### 方案2: 切换Redis队列 ✅✅

**改动**:
```go
config := &notification.QueueConfig{
    Type: notification.QueueTypeRedis,
    BufferSize: 5000,
    Extra: map[string]interface{}{
        "addr": "redis:6379",
    },
}
dispatcher, _ := notification.NewEventDispatcherWithConfig(ctx, config)
```

**优点**:
- ✅ 进程重启不丢消息
- ✅ 可以查看积压情况
- ✅ 支持多实例消费

**缺点**:
- ⚠️ 需要Redis服务
- ⚠️ 网络延迟增加
- ⚠️ 性能略降

**丢失场景**:
- Redis宕机（可主从高可用）
- 网络分区（短暂）

---

### 方案3: 切换Kafka队列 ✅✅✅

**优点**:
- ✅ 高可靠性
- ✅ 高吞吐量
- ✅ 消息可回溯
- ✅ 支持大规模

**缺点**:
- ⚠️ 运维复杂
- ⚠️ 延迟较高
- ⚠️ 资源消耗大

---

### 方案4: 添加重试和死信队列 ✅✅✅

**实现**:
```go
type EventDispatcher struct {
    queue          MessageQueue
    retryQueue     MessageQueue  // 重试队列
    deadLetterChan chan Event    // 死信队列
}

// 处理失败后：
1. 重试3次
2. 仍失败 -> 发送到死信队列
3. 记录到数据库或文件
4. 人工介入/定时重试
```

**优点**:
- ✅ 大幅降低丢失率
- ✅ 可追溯失败消息

**缺点**:
- ⚠️ 增加复杂度
- ⚠️ 需要额外存储

---

## 推荐方案

### 小规模（QPS < 200）
```go
Channel队列 + 重试机制 + 监控告警
```

### 中规模（QPS 200-2000）
```go
Redis队列 + 重试机制 + 死信队列
```

### 大规模（QPS > 2000）
```go
Kafka队列 + 重试机制 + 完善的监控体系
```

## 监控建议

### 必须监控的指标

```go
// 1. 队列长度
queue_length

// 2. 队列使用率
queue_usage_percent = queue_length / queue_capacity * 100

// 3. 消息处理延迟
message_process_latency

// 4. 失败率
message_failure_rate = failed / total

// 5. 丢弃率
message_drop_rate = dropped / total
```

### 告警规则

```
# 队列积压告警
queue_usage_percent > 80% for 5m  => Warning
queue_usage_percent > 95% for 1m  => Critical

# 失败率告警
message_failure_rate > 0.01 for 5m  => Warning  (1%)
message_failure_rate > 0.05 for 1m  => Critical (5%)

# 处理延迟告警
message_process_latency_p99 > 1s for 5m  => Warning
```

## 总结

**Channel队列的本质问题**: 
- 内存存储，进程退出必然丢失
- 适合对可靠性要求不高的场景

**如果需要高可靠性**:
1. 短期: 添加重试+死信队列
2. 长期: 切换到Redis/Kafka

**当前系统优势**:
- ✅ 已经抽象了MessageQueue接口
- ✅ 切换队列实现只需改配置
- ✅ 架构已为升级做好准备

