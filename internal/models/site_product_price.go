package models

import (
	"time"

	"gorm.io/gorm"
)

// SiteProductPrice 子站商品SKU定价
type SiteProductPrice struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	SiteID    uint           `gorm:"not null;index;uniqueIndex:idx_site_sku_price" json:"site_id"`
	ProductID uint           `gorm:"not null;index" json:"product_id"`
	SKUID     uint           `gorm:"column:sku_id;not null;index;uniqueIndex:idx_site_sku_price" json:"sku_id"`
	SitePrice Money          `gorm:"type:decimal(20,2);not null;default:0" json:"site_price"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (SiteProductPrice) TableName() string {
	return "site_product_prices"
}
