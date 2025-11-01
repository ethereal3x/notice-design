package repo

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Notification struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	AccountID int64     `gorm:"column:account_id;not null;comment:用户账号ID" json:"account_id"`
	Type      int8      `gorm:"column:type;not null;comment:通知类型: 1-稿件审核 2-认证审核 3-奖励发放" json:"type"`
	Title     string    `gorm:"column:title;type:varchar(255);not null;comment:通知标题" json:"title"`
	Content   string    `gorm:"column:content;type:text;not null;comment:通知内容" json:"content"`
	Status    int8      `gorm:"column:status;not null;default:0;comment:状态: 0-未读 1-已读" json:"status"`
	ExtData   string    `gorm:"column:ext_data;type:text;comment:扩展数据(JSON格式)" json:"ext_data"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名
func (Notification) TableName() string {
	return "tbl_notification"
}

type NoticeRepository struct {
	db *gorm.DB
}

func NewNoticeRepository(db *gorm.DB) *NoticeRepository {
	return &NoticeRepository{db: db}
}

func (r *NoticeRepository) InsertNotice(ctx context.Context, n *Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *NoticeRepository) GetNoticeByID(ctx context.Context, id uint64) (*Notification, error) {
	var n Notification
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&n).Error
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *NoticeRepository) GetNoticesByAccountID(ctx context.Context, accountID int64, status *int8, limit, offset int) ([]*Notification, error) {
	var notices []*Notification
	query := r.db.WithContext(ctx).Where("account_id = ?", accountID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&notices).Error
	if err != nil {
		return nil, err
	}
	return notices, nil
}

func (r *NoticeRepository) UpdateNoticeStatus(ctx context.Context, id uint64, status int8) error {
	return r.db.WithContext(ctx).Model(&Notification{}).Where("id = ?", id).Update("status", status).Error
}

func (r *NoticeRepository) GetUnreadCount(ctx context.Context, accountID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Notification{}).Where("account_id = ? AND status = ?", accountID, 0).Count(&count).Error
	return count, err
}
