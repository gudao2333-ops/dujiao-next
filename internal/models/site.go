package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	SiteStatusActive   = "active"
	SiteStatusDisabled = "disabled"
)

// Site 用户子站（v1）
type Site struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	OwnerUserID     uint           `gorm:"not null;uniqueIndex" json:"owner_user_id"`
	Name            string         `gorm:"type:varchar(120);not null" json:"name"`
	SubdomainPrefix string         `gorm:"type:varchar(63);not null;uniqueIndex" json:"subdomain_prefix"`
	Suffix          string         `gorm:"type:varchar(120);not null" json:"suffix"`
	FullDomain      string         `gorm:"type:varchar(191);not null;uniqueIndex" json:"full_domain"`
	OpenedOrderID   *uint          `gorm:"index" json:"opened_order_id,omitempty"`
	Status          string         `gorm:"type:varchar(24);not null;default:'active';index" json:"status"`
	CreatedAt       time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Site) TableName() string {
	return "sites"
}
