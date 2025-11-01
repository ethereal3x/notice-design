package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereal3x/apc/logger"
	"github.com/ethereal3x/notice/handler"
	"github.com/ethereal3x/notice/notification"
	"github.com/ethereal3x/notice/repo"
	"gorm.io/gorm"
)

func main() {
	initLog()

	// 1. 初始化日志和上下文
	ctx := context.Background()
	logger.ContextInfo(ctx, "Starting notification service...")

	// 2. 初始化数据库
	db, err := initDB()
	if err != nil {
		logger.ContextError(ctx, fmt.Sprintf("Failed to initialize database: %v", err))
		os.Exit(1)
	}
	logger.ContextInfo(ctx, "Database initialized successfully")

	// 3. 初始化仓储层
	noticeRepo := repo.NewNoticeRepository(db)
	logger.ContextInfo(ctx, "Repository initialized successfully")

	// 4. 初始化事件分发器并注册处理器
	dispatcher := notification.NewEventDispatcher(ctx, 1000)
	dispatcher.RegisterHandler(handler.NewManuscriptHandler(noticeRepo))
	dispatcher.RegisterHandler(handler.NewAwardHandler(noticeRepo))
	dispatcher.Start(5)
	logger.ContextInfo(ctx, "Event dispatcher initialized successfully")

	// 5. 初始化通知管理器
	notification.InitGlobalManager(ctx, dispatcher)
	logger.ContextInfo(ctx, "Notification manager initialized successfully")

	// 6. 启动完成
	logger.ContextInfo(ctx, "Notification service started successfully")

	// 测试发送一条通知
	testNotification(ctx)

	// 7. 等待退出信号
	waitForShutdown(ctx)
}

func initLog() {
	logger.LogInit(logger.Config{
		Level:      logger.LevelDebug,
		Format:     logger.FormatConsole,
		OutputPath: "app.log",
	})
}

// initDB 初始化数据库连接
func initDB() (*gorm.DB, error) {
	config := &repo.DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvAsInt("DB_PORT", 3306),
		User:     getEnv("DB_USER", "root"),
		Password: getEnv("DB_PASSWORD", "123456"),
		DBName:   getEnv("DB_NAME", "notice"),
		Charset:  getEnv("DB_CHARSET", "utf8mb4"),
	}

	db, err := repo.NewDB(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// testNotification 测试发送通知
func testNotification(ctx context.Context) {
	logger.ContextInfo(ctx, "Testing notification dispatch...")

	// 测试发送稿件审核通知
	notification.DispatchManuscriptAuditEvent(
		ctx,
		123456,       // accountID
		"MS001",      // manuscriptID
		1,            // oldStatus
		2,            // newStatus
		"内容质量优秀",     // auditReason
		"admin",      // operateUser
		"2024春季征文大赛", // activityName
	)

	// 测试发送奖励通知
	notification.DispatchAwardEvent(
		ctx,
		123456,       // accountID
		"MS001",      // manuscriptID
		500,          // awardAmount
		"现金奖励",       // awardType
		"2024春季征文大赛", // activityName
	)

	logger.ContextInfo(ctx, "Test notifications dispatched")
}

// waitForShutdown 等待关闭信号
func waitForShutdown(ctx context.Context) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	logger.ContextInfo(ctx, fmt.Sprintf("Received shutdown signal: %s", sig.String()))

	// 优雅关闭
	logger.ContextInfo(ctx, "Shutting down notification service...")
	manager := notification.GetGlobalManager()
	if manager != nil {
		manager.Stop()
	}

	// 等待一段时间让正在处理的任务完成
	time.Sleep(2 * time.Second)
	logger.ContextInfo(ctx, "Notification service stopped")
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt 获取整数类型的环境变量
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
