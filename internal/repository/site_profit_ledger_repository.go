package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SiteProfitLedgerListFilter struct {
	SiteID    uint
	Status    string
	Page      int
	PageSize  int
	OrderNo   string
	CreatedTo *time.Time
}

type SiteProfitLedgerRepository interface {
	WithTx(tx *gorm.DB) SiteProfitLedgerRepository
	Create(row *models.SiteProfitLedger) error
	Update(row *models.SiteProfitLedger) error
	GetByOrderItem(siteID, orderID uint, orderItemID *uint) (*models.SiteProfitLedger, error)
	ListByOrderForUpdate(orderID uint, statuses []string) ([]models.SiteProfitLedger, error)
	ListPendingBeforeForUpdate(before time.Time) ([]models.SiteProfitLedger, error)
	ListByWithdrawIDForUpdate(withdrawID uint) ([]models.SiteProfitLedger, error)
	ListAvailableBySiteForUpdate(siteID uint) ([]models.SiteProfitLedger, error)
	ListBySite(filter SiteProfitLedgerListFilter) ([]models.SiteProfitLedger, int64, error)
	BatchUpdate(ids []uint, updates map[string]interface{}) error
}

type GormSiteProfitLedgerRepository struct{ BaseRepository }

func NewSiteProfitLedgerRepository(db *gorm.DB) *GormSiteProfitLedgerRepository {
	return &GormSiteProfitLedgerRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormSiteProfitLedgerRepository) WithTx(tx *gorm.DB) SiteProfitLedgerRepository {
	if tx == nil {
		return r
	}
	return &GormSiteProfitLedgerRepository{BaseRepository: BaseRepository{db: tx}}
}

func (r *GormSiteProfitLedgerRepository) Create(row *models.SiteProfitLedger) error {
	return r.db.Create(row).Error
}
func (r *GormSiteProfitLedgerRepository) Update(row *models.SiteProfitLedger) error {
	return r.db.Save(row).Error
}

func (r *GormSiteProfitLedgerRepository) GetByOrderItem(siteID, orderID uint, orderItemID *uint) (*models.SiteProfitLedger, error) {
	if siteID == 0 || orderID == 0 {
		return nil, nil
	}
	query := r.db.Where("site_id = ? AND order_id = ? AND ledger_type = ?", siteID, orderID, "order_profit")
	if orderItemID != nil && *orderItemID > 0 {
		query = query.Where("order_item_id = ?", *orderItemID)
	} else {
		query = query.Where("order_item_id IS NULL")
	}
	var row models.SiteProfitLedger
	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *GormSiteProfitLedgerRepository) ListByOrderForUpdate(orderID uint, statuses []string) ([]models.SiteProfitLedger, error) {
	rows := make([]models.SiteProfitLedger, 0)
	if orderID == 0 {
		return rows, nil
	}
	q := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id = ?", orderID)
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormSiteProfitLedgerRepository) ListPendingBeforeForUpdate(before time.Time) ([]models.SiteProfitLedger, error) {
	rows := make([]models.SiteProfitLedger, 0)
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("status = ? AND confirm_at IS NOT NULL AND confirm_at <= ?", constants.SiteProfitStatusPendingConfirm, before).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormSiteProfitLedgerRepository) ListByWithdrawIDForUpdate(withdrawID uint) ([]models.SiteProfitLedger, error) {
	rows := make([]models.SiteProfitLedger, 0)
	if withdrawID == 0 {
		return rows, nil
	}
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("withdraw_id = ?", withdrawID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormSiteProfitLedgerRepository) ListAvailableBySiteForUpdate(siteID uint) ([]models.SiteProfitLedger, error) {
	rows := make([]models.SiteProfitLedger, 0)
	if siteID == 0 {
		return rows, nil
	}
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("site_id = ? AND status = ? AND withdraw_id IS NULL", siteID, constants.SiteProfitStatusAvailable).
		Order("id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *GormSiteProfitLedgerRepository) ListBySite(filter SiteProfitLedgerListFilter) ([]models.SiteProfitLedger, int64, error) {
	rows := make([]models.SiteProfitLedger, 0)
	q := r.db.Model(&models.SiteProfitLedger{}).Where("site_id = ?", filter.SiteID)
	if status := strings.TrimSpace(filter.Status); status != "" {
		q = q.Where("status = ?", status)
	}
	if filter.CreatedTo != nil {
		q = q.Where("created_at <= ?", *filter.CreatedTo)
	}
	if orderNo := strings.TrimSpace(filter.OrderNo); orderNo != "" {
		q = q.Joins("JOIN orders ON orders.id = site_profit_ledgers.order_id").Where("orders.order_no LIKE ?", "%"+orderNo+"%")
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

func (r *GormSiteProfitLedgerRepository) BatchUpdate(ids []uint, updates map[string]interface{}) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&models.SiteProfitLedger{}).Where("id IN ?", ids).Updates(updates).Error
}
