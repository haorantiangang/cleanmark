package model

import (
	"time"
)

type User struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	OpenID         string     `json:"openid" gorm:"uniqueIndex;size:128"`
	UnionID        string     `json:"union_id" gorm:"index;size:128"`
	Phone          string     `json:"phone" gorm:"size:20"`
	Nickname       string     `json:"nickname" gorm:"size:64"`
	Avatar         string     `json:"avatar" gorm:"size:512"`
	VipLevel       int        `json:"vip_level" gorm:"default:0;comment:'0免费 1月卡 2年卡 3永久'"`
	VipExpireTime  *time.Time `json:"vip_expire_time"`
	DailyQuota     int        `json:"daily_quota" gorm:"default:3"`
	UsedQuota      int        `json:"used_quota" gorm:"default:0"`
	LastActiveDate *time.Time `json:"last_active_date"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

type Task struct {
	ID           uint            `json:"id" gorm:"primaryKey"`
	UserID       uint            `json:"user_id" gorm:"index;not null"`
	SourceURL    string          `json:"source_url" gorm:"size:1024;not null"`
	PlatformType string          `json:"platform_type" gorm:"size:32;not null;comment:'douyin/kuaishou/xiaohongshu/bilibili'"`
	Status       string          `json:"status" gorm:"size:16;default:'pending';comment:'pending/processing/success/failed'"`
	Title        string          `json:"title" gorm:"size:256"`
	CoverURL     string          `json:"cover_url" gorm:"size:1024"`
	OriginalURL  string          `json:"original_url" gorm:"size:2048"`
	CleanURL     string          `json:"clean_url" gorm:"size:2048"`
	FileSize     int64           `json:"file_size" gorm:"default:0"`
	Duration     int             `json:"duration" gorm:"default:0;comment:'视频时长（秒）'"`
	Quality      string          `json:"quality" gorm:"size:16;default:'hd';comment:'sd/hd/uhd'"`
	ErrorMessage string          `json:"error_message" gorm:"size:512"`
	ProcessTime  int             `json:"process_time" gorm:"default:0;comment:'处理耗时（毫秒）'"`
	CreatedAt    time.Time       `json:"created_at"`
	CompletedAt  *time.Time      `json:"completed_at"`
	User         *User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

func (Task) TableName() string {
	return "tasks"
}

type Order struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	UserID       uint       `json:"user_id" gorm:"index;not null"`
	OrderNo      string     `json:"order_no" gorm:"uniqueIndex;size:64;not null"`
	ProductType  string     `json:"product_type" gorm:"size:32;not null;comment:'monthly/yearly/lifetime/single'"`
	Amount       float64    `json:"amount" gorm:"not null"`
	PayMethod    string     `json:"pay_method" gorm:"size:16;comment:'wechat/alipay'"`
	Status       string     `json:"status" gorm:"size:16;default:'pending';comment:'pending/paid/refunded'"`
	PayTime      *time.Time `json:"pay_time"`
	ExpireTime   *time.Time `json:"expire_time"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	User         *User      `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

func (Order) TableName() string {
	return "orders"
}
