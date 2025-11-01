# 通知服务 (Notice Service)

一个基于Go语言开发的异步通知系统，支持多种类型的通知事件处理。

## 功能特性

- ✅ 异步事件分发机制
- ✅ 支持多种通知类型（稿件审核、奖励发放等）
- ✅ 基于GORM的数据库操作
- ✅ 优雅的工作池设计
- ✅ 完善的日志记录
- ✅ 优雅关闭支持

## 项目结构

```
.
├── constants/          # 常量定义
│   └── constants.go    # 通知类型、状态等常量
├── docs/              # 文档
│   └── notification.sql # 数据库表结构
├── notification/      # 通知处理核心
│   ├── event.go       # 事件定义
│   ├── dispatcher.go  # 事件分发器
│   ├── manager.go     # 全局管理器
│   ├── manuscript_handler.go  # 稿件审核处理器
│   └── award_handler.go       # 奖励发放处理器
├── repo/             # 数据访问层
│   ├── db.go         # 数据库初始化
│   └── repo.go       # 通知仓储
├── go.mod
└── main.go           # 程序入口
```

## 快速开始

### 1. 环境要求

- Go 1.24+
- MySQL 5.7+

### 2. 数据库初始化

```bash
# 执行数据库脚本
mysql -u root -p < docs/notification.sql
```

### 3. 配置环境变量

```bash
# 设置数据库连接信息
export DB_HOST=localhost
export DB_PORT=3306
export DB_USER=root
export DB_PASSWORD=your_password
export DB_NAME=notice
export DB_CHARSET=utf8mb4
```

### 4. 安装依赖

```bash
go mod tidy
```

### 5. 运行服务

```bash
go run main.go
```

## 使用示例

### 发送稿件审核通知

```go
package main

import (
    "context"
    "notice/notification"
)

func main() {
    ctx := context.Background()
    
    // 发送稿件审核通知
    notification.DispatchManuscriptAuditEvent(
        ctx,
        123456,              // 用户账号ID
        "MS001",             // 稿件ID
        1,                   // 旧状态（审核中）
        2,                   // 新状态（审核通过）
        "内容质量优秀",       // 审核原因
        "admin",             // 操作人
        "2024春季征文大赛",  // 活动名称
    )
}
```

### 发送奖励发放通知

```go
// 发送奖励通知
notification.DispatchAwardEvent(
    ctx,
    123456,              // 用户账号ID
    "MS001",             // 稿件ID
    500,                 // 奖励金额
    "现金奖励",          // 奖励类型
    "2024春季征文大赛",  // 活动名称
)
```

## 数据库表结构

通知表包含以下字段：

- `id` - 主键ID
- `account_id` - 用户账号ID
- `type` - 通知类型（1-稿件审核 2-认证审核 3-奖励发放）
- `title` - 通知标题
- `content` - 通知内容
- `status` - 状态（0-未读 1-已读）
- `ext_data` - 扩展数据（JSON格式）
- `created_at` - 创建时间
- `updated_at` - 更新时间

## 扩展开发

### 添加新的通知类型

1. 在 `constants/constants.go` 中添加新的通知类型常量
2. 在 `notification/event.go` 中定义新的事件类型
3. 创建新的 handler 实现 `EventHandler` 接口
4. 在 `notification/manager.go` 中注册新的 handler

示例：

```go
// 1. 定义新的事件类型
type NewEvent struct {
    BaseEvent
    // 自定义字段
}

// 2. 实现新的处理器
type NewHandler struct {
    repo *repo.NoticeRepository
}

func (h *NewHandler) SupportEventType() EventType {
    return EventTypeNew
}

func (h *NewHandler) Handle(event Event) error {
    // 处理逻辑
    return nil
}

// 3. 在InitGlobalManager中注册
dispatcher.RegisterHandler(NewNewHandler(noticeRepo))
```

## API接口（待实现）

项目预留了以下数据访问接口：

- `GetNoticeByID` - 根据ID查询通知
- `GetNoticesByAccountID` - 查询用户通知列表（支持分页和状态过滤）
- `UpdateNoticeStatus` - 更新通知状态
- `GetUnreadCount` - 获取未读通知数量

## 监控指标

通过 `GetMetrics()` 方法可以获取以下指标：

- `event_channel_len` - 当前待处理事件数量
- `event_channel_cap` - 事件队列容量

## 优雅关闭

服务支持优雅关闭，收到 SIGINT 或 SIGTERM 信号后会：

1. 停止接收新事件
2. 等待现有事件处理完成
3. 关闭数据库连接
4. 退出程序

## 注意事项

1. 确保数据库连接配置正确
2. 生产环境建议调整工作池大小（默认5个worker）
3. 事件队列默认大小1000，可根据实际情况调整
4. 建议配置数据库连接池参数
