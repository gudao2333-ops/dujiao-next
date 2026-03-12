package models

import (
	"time"

	"gorm.io/gorm"
)

// SiteWithdraw 子站提现申请
type SiteWithdraw struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	SiteID       uint           `gorm:"not null;index" json:"site_id"`
	Amount       Money          `gorm:"type:decimal(20,2);not null;default:0" json:"amount"`
	Channel      string         `gorm:"type:varchar(50);not null" json:"channel"`
	Account      string         `gorm:"type:varchar(255);not null" json:"account"`
	Status       string         `gorm:"type:varchar(32);not null;index" json:"status"`
	RejectReason string         `gorm:"type:varchar(255)" json:"reject_reason"`
	ProcessedBy  *uint          `gorm:"index" json:"processed_by,omitempty"`
	ProcessedAt  *time.Time     `gorm:"index" json:"processed_at,omitempty"`
	CreatedAt    time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (SiteWithdraw) TableName() string { return "site_withdraws" }
