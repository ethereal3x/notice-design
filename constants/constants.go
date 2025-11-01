package constants

// 通知类型常量
const (
	NOTIFICATION_TYPE_MANUSCRIPT_AUDIT    int8 = 1 // 稿件审核通知
	NOTIFICATION_TYPE_CERTIFICATION_AUDIT int8 = 2 // 认证审核通知
	NOTIFICATION_TYPE_REWARD_DISTRIBUTE   int8 = 3 // 奖励发放通知
)

// 通知状态常量
const (
	NOTIFICATION_STATUS_UNREAD int8 = 0 // 未读
	NOTIFICATION_STATUS_READ   int8 = 1 // 已读
)

// 稿件审核状态常量
const (
	MANUSCRIPT_AUDIT_STATUS_PENDING  int8 = 1 // 审核中
	MANUSCRIPT_AUDIT_STATUS_APPROVED int8 = 2 // 审核通过
	MANUSCRIPT_AUDIT_STATUS_REJECTED int8 = 3 // 审核未通过
)
