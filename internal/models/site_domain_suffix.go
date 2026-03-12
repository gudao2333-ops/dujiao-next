package models

import (
	"time"

	"gorm.io/gorm"
)

// SiteDomainSuffix 子站域名后缀配置
type SiteDomainSuffix struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Suffix    string         `gorm:"type:varchar(120);not null;uniqueIndex" json:"suffix"`
	IsEnabled bool           `gorm:"not null;default:true;index" json:"is_enabled"`
	SortOrder int            `gorm:"not null;default:0;index" json:"sort_order"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (SiteDomainSuffix) TableName() string {
	return "site_domain_suffixes"
}
