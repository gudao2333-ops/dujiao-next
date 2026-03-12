package repository

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SiteRepository 子站仓储
type SiteRepository interface {
	Create(site *models.Site) error
	Update(site *models.Site) error
	GetByID(id uint) (*models.Site, error)
	GetByOwnerUserID(userID uint) (*models.Site, error)
	GetByPrefix(prefix string) (*models.Site, error)
	GetByFullDomain(fullDomain string) (*models.Site, error)
	GetByOpenedOrderID(orderID uint) (*models.Site, error)
	GetByOpenedOrderIDForUpdate(orderID uint) (*models.Site, error)
	List(page, pageSize int, status string) ([]models.Site, int64, error)
	WithTx(tx *gorm.DB) *GormSiteRepository
}

type GormSiteRepository struct {
	BaseRepository
}

func NewSiteRepository(db *gorm.DB) *GormSiteRepository {
	return &GormSiteRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormSiteRepository) WithTx(tx *gorm.DB) *GormSiteRepository {
	if tx == nil {
		return r
	}
	return &GormSiteRepository{BaseRepository: BaseRepository{db: tx}}
}

func (r *GormSiteRepository) Create(site *models.Site) error {
	return r.db.Create(site).Error
}

func (r *GormSiteRepository) Update(site *models.Site) error {
	return r.db.Save(site).Error
}

func (r *GormSiteRepository) GetByID(id uint) (*models.Site, error) {
	if id == 0 {
		return nil, nil
	}
	var site models.Site
	if err := r.db.First(&site, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *GormSiteRepository) GetByOwnerUserID(userID uint) (*models.Site, error) {
	if userID == 0 {
		return nil, nil
	}
	var site models.Site
	if err := r.db.Where("owner_user_id = ?", userID).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *GormSiteRepository) GetByPrefix(prefix string) (*models.Site, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil, nil
	}
	var site models.Site
	if err := r.db.Where("subdomain_prefix = ?", prefix).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *GormSiteRepository) GetByFullDomain(fullDomain string) (*models.Site, error) {
	fullDomain = strings.ToLower(strings.TrimSpace(fullDomain))
	if fullDomain == "" {
		return nil, nil
	}
	var site models.Site
	if err := r.db.Where("full_domain = ?", fullDomain).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *GormSiteRepository) GetByOpenedOrderID(orderID uint) (*models.Site, error) {
	if orderID == 0 {
		return nil, nil
	}
	var site models.Site
	if err := r.db.Where("opened_order_id = ?", orderID).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *GormSiteRepository) GetByOpenedOrderIDForUpdate(orderID uint) (*models.Site, error) {
	if orderID == 0 {
		return nil, nil
	}
	var site models.Site
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("opened_order_id = ?", orderID).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *GormSiteRepository) List(page, pageSize int, status string) ([]models.Site, int64, error) {
	rows := make([]models.Site, 0)
	q := r.db.Model(&models.Site{})
	if status = strings.TrimSpace(status); status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = applyPagination(q, page, pageSize)
	if err := q.Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
