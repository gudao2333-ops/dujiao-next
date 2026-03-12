package repository

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SiteProfitAccountRepository interface {
	WithTx(tx *gorm.DB) SiteProfitAccountRepository
	GetBySiteID(siteID uint) (*models.SiteProfitAccount, error)
	GetBySiteIDForUpdate(siteID uint) (*models.SiteProfitAccount, error)
	Create(account *models.SiteProfitAccount) error
	Update(account *models.SiteProfitAccount) error
	EnsureBySiteID(siteID uint, now time.Time) (*models.SiteProfitAccount, error)
}

type GormSiteProfitAccountRepository struct{ BaseRepository }

func NewSiteProfitAccountRepository(db *gorm.DB) *GormSiteProfitAccountRepository {
	return &GormSiteProfitAccountRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormSiteProfitAccountRepository) WithTx(tx *gorm.DB) SiteProfitAccountRepository {
	if tx == nil {
		return r
	}
	return &GormSiteProfitAccountRepository{BaseRepository: BaseRepository{db: tx}}
}

func (r *GormSiteProfitAccountRepository) GetBySiteID(siteID uint) (*models.SiteProfitAccount, error) {
	if siteID == 0 {
		return nil, nil
	}
	var row models.SiteProfitAccount
	if err := r.db.Where("site_id = ?", siteID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *GormSiteProfitAccountRepository) GetBySiteIDForUpdate(siteID uint) (*models.SiteProfitAccount, error) {
	if siteID == 0 {
		return nil, nil
	}
	var row models.SiteProfitAccount
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ?", siteID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *GormSiteProfitAccountRepository) Create(account *models.SiteProfitAccount) error {
	return r.db.Create(account).Error
}
func (r *GormSiteProfitAccountRepository) Update(account *models.SiteProfitAccount) error {
	return r.db.Save(account).Error
}

func (r *GormSiteProfitAccountRepository) EnsureBySiteID(siteID uint, now time.Time) (*models.SiteProfitAccount, error) {
	if siteID == 0 {
		return nil, nil
	}
	row, err := r.GetBySiteIDForUpdate(siteID)
	if err != nil {
		return nil, err
	}
	if row != nil {
		return row, nil
	}
	created := &models.SiteProfitAccount{SiteID: siteID, CreatedAt: now, UpdatedAt: now}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(created).Error; err != nil {
		return nil, err
	}
	return r.GetBySiteIDForUpdate(siteID)
}
