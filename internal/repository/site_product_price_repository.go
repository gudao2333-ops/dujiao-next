package repository

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// SiteProductPriceRepository 子站商品SKU定价仓储
type SiteProductPriceRepository interface {
	Upsert(row *models.SiteProductPrice) error
	GetBySiteAndSKU(siteID, skuID uint) (*models.SiteProductPrice, error)
	ListBySite(siteID uint) ([]models.SiteProductPrice, error)
	DeleteBySiteAndSKU(siteID, skuID uint) error
	WithTx(tx *gorm.DB) *GormSiteProductPriceRepository
}

type GormSiteProductPriceRepository struct {
	BaseRepository
}

func NewSiteProductPriceRepository(db *gorm.DB) *GormSiteProductPriceRepository {
	return &GormSiteProductPriceRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormSiteProductPriceRepository) WithTx(tx *gorm.DB) *GormSiteProductPriceRepository {
	if tx == nil {
		return r
	}
	return &GormSiteProductPriceRepository{BaseRepository: BaseRepository{db: tx}}
}

func (r *GormSiteProductPriceRepository) Upsert(row *models.SiteProductPrice) error {
	if row == nil || row.SiteID == 0 || row.SKUID == 0 {
		return nil
	}
	now := row.UpdatedAt
	if now.IsZero() {
		now = row.CreatedAt
	}
	if now.IsZero() {
		now = time.Now()
	}
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	updates := map[string]interface{}{
		"product_id": row.ProductID,
		"site_price": row.SitePrice,
		"updated_at": row.UpdatedAt,
	}
	return r.db.Where("site_id = ? AND sku_id = ?", row.SiteID, row.SKUID).
		Assign(updates).
		FirstOrCreate(row, &models.SiteProductPrice{SiteID: row.SiteID, SKUID: row.SKUID}).Error
}

func (r *GormSiteProductPriceRepository) GetBySiteAndSKU(siteID, skuID uint) (*models.SiteProductPrice, error) {
	if siteID == 0 || skuID == 0 {
		return nil, nil
	}
	var row models.SiteProductPrice
	if err := r.db.Where("site_id = ? AND sku_id = ?", siteID, skuID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *GormSiteProductPriceRepository) ListBySite(siteID uint) ([]models.SiteProductPrice, error) {
	rows := make([]models.SiteProductPrice, 0)
	if siteID == 0 {
		return rows, nil
	}
	if err := r.db.Where("site_id = ?", siteID).Order("sku_id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormSiteProductPriceRepository) DeleteBySiteAndSKU(siteID, skuID uint) error {
	if siteID == 0 || skuID == 0 {
		return nil
	}
	return r.db.Where("site_id = ? AND sku_id = ?", siteID, skuID).Delete(&models.SiteProductPrice{}).Error
}
