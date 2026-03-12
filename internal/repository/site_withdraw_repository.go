package repository

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SiteWithdrawListFilter struct {
	SiteID   uint
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

type SiteWithdrawRepository interface {
	Transaction(fn func(tx *gorm.DB) error) error
	WithTx(tx *gorm.DB) SiteWithdrawRepository
	Create(row *models.SiteWithdraw) error
	Update(row *models.SiteWithdraw) error
	GetByID(id uint) (*models.SiteWithdraw, error)
	GetByIDForUpdate(id uint) (*models.SiteWithdraw, error)
	List(filter SiteWithdrawListFilter) ([]models.SiteWithdraw, int64, error)
}

type GormSiteWithdrawRepository struct{ BaseRepository }

func NewSiteWithdrawRepository(db *gorm.DB) *GormSiteWithdrawRepository {
	return &GormSiteWithdrawRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormSiteWithdrawRepository) WithTx(tx *gorm.DB) SiteWithdrawRepository {
	if tx == nil {
		return r
	}
	return &GormSiteWithdrawRepository{BaseRepository: BaseRepository{db: tx}}
}

func (r *GormSiteWithdrawRepository) Create(row *models.SiteWithdraw) error {
	return r.db.Create(row).Error
}
func (r *GormSiteWithdrawRepository) Update(row *models.SiteWithdraw) error {
	return r.db.Save(row).Error
}

func (r *GormSiteWithdrawRepository) GetByID(id uint) (*models.SiteWithdraw, error) {
	if id == 0 {
		return nil, nil
	}
	var row models.SiteWithdraw
	if err := r.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *GormSiteWithdrawRepository) GetByIDForUpdate(id uint) (*models.SiteWithdraw, error) {
	if id == 0 {
		return nil, nil
	}
	var row models.SiteWithdraw
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *GormSiteWithdrawRepository) List(filter SiteWithdrawListFilter) ([]models.SiteWithdraw, int64, error) {
	rows := make([]models.SiteWithdraw, 0)
	q := r.db.Model(&models.SiteWithdraw{})
	if filter.SiteID > 0 {
		q = q.Where("site_id = ?", filter.SiteID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("account LIKE ? OR channel LIKE ?", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = applyPagination(q, filter.Page, filter.PageSize)
	if err := q.Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
