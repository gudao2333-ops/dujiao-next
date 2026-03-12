package models

import (
	"time"

	"gorm.io/gorm"
)

// SiteProfitLedger 子站利润流水
type SiteProfitLedger struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	SiteID        uint           `gorm:"not null;index" json:"site_id"`
	OrderID       uint           `gorm:"not null;index" json:"order_id"`
	OrderItemID   *uint          `gorm:"index;uniqueIndex:idx_site_order_item_profit" json:"order_item_id,omitempty"`
	LedgerType    string         `gorm:"type:varchar(32);not null;default:'order_profit';index;uniqueIndex:idx_site_order_item_profit" json:"ledger_type"`
	Amount        Money          `gorm:"type:decimal(20,2);not null;default:0" json:"amount"`
	Status        string         `gorm:"type:varchar(32);not null;index" json:"status"`
	ConfirmAt     *time.Time     `gorm:"index" json:"confirm_at,omitempty"`
	AvailableAt   *time.Time     `gorm:"index" json:"available_at,omitempty"`
	WithdrawID    *uint          `gorm:"index" json:"withdraw_id,omitempty"`
	InvalidReason string         `gorm:"type:varchar(255)" json:"invalid_reason"`
	CreatedAt     time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (SiteProfitLedger) TableName() string { return "site_profit_ledgers" }
