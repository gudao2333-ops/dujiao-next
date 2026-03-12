package models

import (
	"time"

	"gorm.io/gorm"
)

// SiteProfitAccount 子站利润账户
type SiteProfitAccount struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	SiteID          uint           `gorm:"not null;uniqueIndex" json:"site_id"`
	PendingAmount   Money          `gorm:"type:decimal(20,2);not null;default:0" json:"pending_amount"`
	AvailableAmount Money          `gorm:"type:decimal(20,2);not null;default:0" json:"available_amount"`
	WithdrawnAmount Money          `gorm:"type:decimal(20,2);not null;default:0" json:"withdrawn_amount"`
	CreatedAt       time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (SiteProfitAccount) TableName() string { return "site_profit_accounts" }
