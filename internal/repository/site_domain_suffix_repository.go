package repository

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// SiteDomainSuffixRepository 子站域名后缀仓储
type SiteDomainSuffixRepository interface {
	ListEnabled() ([]models.SiteDomainSuffix, error)
	GetBySuffix(suffix string) (*models.SiteDomainSuffix, error)
	WithTx(tx *gorm.DB) *GormSiteDomainSuffixRepository
}

type GormSiteDomainSuffixRepository struct {
	BaseRepository
}

func NewSiteDomainSuffixRepository(db *gorm.DB) *GormSiteDomainSuffixRepository {
	return &GormSiteDomainSuffixRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormSiteDomainSuffixRepository) WithTx(tx *gorm.DB) *GormSiteDomainSuffixRepository {
	if tx == nil {
		return r
	}
	return &GormSiteDomainSuffixRepository{BaseRepository: BaseRepository{db: tx}}
}

func (r *GormSiteDomainSuffixRepository) ListEnabled() ([]models.SiteDomainSuffix, error) {
	rows := make([]models.SiteDomainSuffix, 0)
	if err := r.db.Where("is_enabled = ?", true).Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormSiteDomainSuffixRepository) GetBySuffix(suffix string) (*models.SiteDomainSuffix, error) {
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	if suffix == "" {
		return nil, nil
	}
	var row models.SiteDomainSuffix
	if err := r.db.Where("suffix = ?", suffix).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
