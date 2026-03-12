package service

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SiteProfitService struct {
	siteRepo       repository.SiteRepository
	orderRepo      repository.OrderRepository
	accountRepo    repository.SiteProfitAccountRepository
	ledgerRepo     repository.SiteProfitLedgerRepository
	withdrawRepo   repository.SiteWithdrawRepository
	settingService *SettingService
}

type SiteOwnerSummary struct {
	Site    *models.Site              `json:"site"`
	Account *models.SiteProfitAccount `json:"account"`
}

type SiteOrderSummary struct {
	OrderID      uint         `json:"order_id"`
	OrderNo      string       `json:"order_no"`
	Status       string       `json:"status"`
	OrderScene   string       `json:"order_scene"`
	TotalAmount  models.Money `json:"total_amount"`
	Currency     string       `json:"currency"`
	CreatedAt    time.Time    `json:"created_at"`
	PaidAt       *time.Time   `json:"paid_at,omitempty"`
	ItemCount    int          `json:"item_count"`
	TotalQty     int          `json:"total_qty"`
	ProductNames []string     `json:"product_names"`
}

type SiteWithdrawApplyInput struct {
	Amount  decimal.Decimal
	Channel string
	Account string
}

type SiteAdminWithdrawListFilter struct {
	Page     int
	PageSize int
	Status   string
	Keyword  string
}

func NewSiteProfitService(siteRepo repository.SiteRepository, orderRepo repository.OrderRepository, accountRepo repository.SiteProfitAccountRepository, ledgerRepo repository.SiteProfitLedgerRepository, withdrawRepo repository.SiteWithdrawRepository, settingService *SettingService) *SiteProfitService {
	return &SiteProfitService{siteRepo: siteRepo, orderRepo: orderRepo, accountRepo: accountRepo, ledgerRepo: ledgerRepo, withdrawRepo: withdrawRepo, settingService: settingService}
}

func (s *SiteProfitService) getMySite(userID uint) (*models.Site, error) {
	if userID == 0 || s.siteRepo == nil {
		return nil, ErrSiteNotOpened
	}
	site, err := s.siteRepo.GetByOwnerUserID(userID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, ErrSiteNotOpened
	}
	return site, nil
}

func (s *SiteProfitService) GetMySummary(userID uint) (*SiteOwnerSummary, error) {
	site, err := s.getMySite(userID)
	if err != nil {
		return nil, err
	}
	acct, err := s.accountRepo.GetBySiteID(site.ID)
	if err != nil {
		return nil, err
	}
	if acct == nil {
		now := time.Now()
		acct = &models.SiteProfitAccount{SiteID: site.ID, CreatedAt: now, UpdatedAt: now}
	}
	return &SiteOwnerSummary{Site: site, Account: acct}, nil
}

func (s *SiteProfitService) ListMyOrders(userID uint, page, pageSize int, status string) ([]SiteOrderSummary, int64, error) {
	site, err := s.getMySite(userID)
	if err != nil {
		return nil, 0, err
	}
	orders, total, err := s.orderRepo.ListBySite(site.ID, page, pageSize, status)
	if err != nil {
		return nil, 0, err
	}
	result := make([]SiteOrderSummary, 0, len(orders))
	for _, order := range orders {
		names := make([]string, 0, len(order.Items))
		totalQty := 0
		for _, item := range order.Items {
			totalQty += item.Quantity
			if len(names) < 3 {
				if zh, ok := item.TitleJSON["zh-CN"].(string); ok && strings.TrimSpace(zh) != "" {
					names = append(names, strings.TrimSpace(zh))
				}
			}
		}
		result = append(result, SiteOrderSummary{OrderID: order.ID, OrderNo: order.OrderNo, Status: order.Status, OrderScene: order.OrderScene, TotalAmount: order.TotalAmount, Currency: order.Currency, CreatedAt: order.CreatedAt, PaidAt: order.PaidAt, ItemCount: len(order.Items), TotalQty: totalQty, ProductNames: names})
	}
	return result, total, nil
}

func (s *SiteProfitService) ListMyLedgers(userID uint, page, pageSize int, status string) ([]models.SiteProfitLedger, int64, error) {
	site, err := s.getMySite(userID)
	if err != nil {
		return nil, 0, err
	}
	return s.ledgerRepo.ListBySite(repository.SiteProfitLedgerListFilter{SiteID: site.ID, Status: status, Page: page, PageSize: pageSize})
}

func (s *SiteProfitService) ListMyWithdraws(userID uint, page, pageSize int, status string) ([]models.SiteWithdraw, int64, error) {
	site, err := s.getMySite(userID)
	if err != nil {
		return nil, 0, err
	}
	return s.withdrawRepo.List(repository.SiteWithdrawListFilter{SiteID: site.ID, Status: status, Page: page, PageSize: pageSize})
}

func (s *SiteProfitService) ApplyWithdraw(userID uint, input SiteWithdrawApplyInput) (*models.SiteWithdraw, error) {
	site, err := s.getMySite(userID)
	if err != nil {
		return nil, err
	}
	cfg, err := s.settingService.GetSiteOpenSetting()
	if err != nil {
		return nil, err
	}
	amount := input.Amount.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) || amount.LessThan(cfg.MinWithdrawAmount.Decimal.Round(2)) {
		return nil, ErrSiteWithdrawAmountInvalid
	}
	channel := strings.TrimSpace(input.Channel)
	account := strings.TrimSpace(input.Account)
	if channel == "" || account == "" {
		return nil, ErrSiteWithdrawChannelInvalid
	}
	if len(cfg.WithdrawChannels) > 0 && !containsString(cfg.WithdrawChannels, strings.ToLower(channel)) {
		return nil, ErrSiteWithdrawChannelInvalid
	}

	var createdID uint
	err = s.withdrawRepo.Transaction(func(tx *gorm.DB) error {
		accountRepo := s.accountRepo.WithTx(tx)
		ledgerRepo := s.ledgerRepo.WithTx(tx)
		withdrawRepo := s.withdrawRepo.WithTx(tx)
		now := time.Now()
		acct, err := accountRepo.EnsureBySiteID(site.ID, now)
		if err != nil {
			return err
		}
		if acct.AvailableAmount.Decimal.Round(2).LessThan(amount) {
			return ErrSiteWithdrawInsufficient
		}
		ledgers, err := ledgerRepo.ListAvailableBySiteForUpdate(site.ID)
		if err != nil {
			return err
		}
		remaining := amount
		selected := make([]uint, 0)
		for _, ledger := range ledgers {
			if remaining.LessThanOrEqual(decimal.Zero) {
				break
			}
			current := ledger.Amount.Decimal.Round(2)
			if current.LessThanOrEqual(decimal.Zero) {
				continue
			}
			if current.LessThanOrEqual(remaining) {
				selected = append(selected, ledger.ID)
				remaining = remaining.Sub(current).Round(2)
				continue
			}
		}
		if remaining.GreaterThan(decimal.Zero) {
			return ErrSiteWithdrawInsufficient
		}
		withdraw := &models.SiteWithdraw{SiteID: site.ID, Amount: models.NewMoneyFromDecimal(amount), Channel: channel, Account: account, Status: constants.SiteWithdrawStatusPendingReview, CreatedAt: now, UpdatedAt: now}
		if err := withdrawRepo.Create(withdraw); err != nil {
			return err
		}
		if err := ledgerRepo.BatchUpdate(selected, map[string]interface{}{"withdraw_id": withdraw.ID, "updated_at": now}); err != nil {
			return err
		}
		acct.AvailableAmount = models.NewMoneyFromDecimal(acct.AvailableAmount.Decimal.Sub(amount).Round(2))
		acct.UpdatedAt = now
		if err := accountRepo.Update(acct); err != nil {
			return err
		}
		createdID = withdraw.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.withdrawRepo.GetByID(createdID)
}

func (s *SiteProfitService) ListAdminWithdraws(filter SiteAdminWithdrawListFilter) ([]models.SiteWithdraw, int64, error) {
	return s.withdrawRepo.List(repository.SiteWithdrawListFilter{Page: filter.Page, PageSize: filter.PageSize, Status: filter.Status, Keyword: filter.Keyword})
}

func (s *SiteProfitService) ReviewWithdraw(adminID, withdrawID uint, action, rejectReason string) (*models.SiteWithdraw, error) {
	if withdrawID == 0 {
		return nil, ErrNotFound
	}
	action = strings.TrimSpace(strings.ToLower(action))
	if action != constants.SiteWithdrawActionReject && action != constants.SiteWithdrawActionPay {
		return nil, ErrSiteWithdrawStatusInvalid
	}
	err := s.withdrawRepo.Transaction(func(tx *gorm.DB) error {
		withdrawRepo := s.withdrawRepo.WithTx(tx)
		ledgerRepo := s.ledgerRepo.WithTx(tx)
		accountRepo := s.accountRepo.WithTx(tx)
		row, err := withdrawRepo.GetByIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		if row == nil {
			return ErrNotFound
		}
		if row.Status != constants.SiteWithdrawStatusPendingReview {
			return ErrSiteWithdrawStatusInvalid
		}
		now := time.Now()
		ledgers, err := ledgerRepo.ListByWithdrawIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		total := decimal.Zero
		ids := make([]uint, 0, len(ledgers))
		for _, ledger := range ledgers {
			total = total.Add(ledger.Amount.Decimal)
			ids = append(ids, ledger.ID)
		}
		acct, err := accountRepo.EnsureBySiteID(row.SiteID, now)
		if err != nil {
			return err
		}
		if action == constants.SiteWithdrawActionReject {
			row.Status = constants.SiteWithdrawStatusRejected
			row.RejectReason = strings.TrimSpace(rejectReason)
			acct.AvailableAmount = models.NewMoneyFromDecimal(acct.AvailableAmount.Decimal.Add(total).Round(2))
			if err := ledgerRepo.BatchUpdate(ids, map[string]interface{}{"withdraw_id": nil, "updated_at": now}); err != nil {
				return err
			}
		} else {
			row.Status = constants.SiteWithdrawStatusPaid
			row.RejectReason = ""
			acct.WithdrawnAmount = models.NewMoneyFromDecimal(acct.WithdrawnAmount.Decimal.Add(total).Round(2))
			if err := ledgerRepo.BatchUpdate(ids, map[string]interface{}{"status": constants.SiteProfitStatusWithdrawn, "updated_at": now}); err != nil {
				return err
			}
		}
		row.ProcessedBy = &adminID
		row.ProcessedAt = &now
		row.UpdatedAt = now
		acct.UpdatedAt = now
		if err := accountRepo.Update(acct); err != nil {
			return err
		}
		return withdrawRepo.Update(row)
	})
	if err != nil {
		return nil, err
	}
	return s.withdrawRepo.GetByID(withdrawID)
}

func (s *SiteProfitService) HandleOrderPaid(orderID uint) error {
	if orderID == 0 || s.orderRepo == nil || s.ledgerRepo == nil {
		return nil
	}
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil || order == nil || order.SiteID == nil || *order.SiteID == 0 {
		return err
	}
	if strings.TrimSpace(order.OrderScene) != constants.OrderSceneProduct {
		return nil
	}
	if strings.TrimSpace(order.OrderScene) == constants.OrderSceneRedeem || order.TotalAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	cfg, err := s.settingService.GetSiteOpenSetting()
	if err != nil {
		return err
	}
	return s.withdrawRepo.Transaction(func(tx *gorm.DB) error {
		ledgerRepo := s.ledgerRepo.WithTx(tx)
		accountRepo := s.accountRepo.WithTx(tx)
		now := time.Now()
		acct, err := accountRepo.EnsureBySiteID(*order.SiteID, now)
		if err != nil {
			return err
		}
		for _, item := range order.Items {
			profit := item.SiteProfitSnapshot.Decimal.Mul(decimal.NewFromInt(int64(item.Quantity))).Round(2)
			if profit.LessThanOrEqual(decimal.Zero) {
				continue
			}
			itemID := item.ID
			existing, err := ledgerRepo.GetByOrderItem(*order.SiteID, order.ID, &itemID)
			if err != nil {
				return err
			}
			if existing != nil {
				continue
			}
			status := constants.SiteProfitStatusPendingConfirm
			var confirmAt *time.Time
			var availableAt *time.Time
			paidAt := now
			if order.PaidAt != nil {
				paidAt = *order.PaidAt
			}
			if cfg.ProfitConfirmDays <= 0 {
				status = constants.SiteProfitStatusAvailable
				availableAt = &paidAt
				acct.AvailableAmount = models.NewMoneyFromDecimal(acct.AvailableAmount.Decimal.Add(profit).Round(2))
			} else {
				t := paidAt.Add(time.Duration(cfg.ProfitConfirmDays) * 24 * time.Hour)
				confirmAt = &t
				acct.PendingAmount = models.NewMoneyFromDecimal(acct.PendingAmount.Decimal.Add(profit).Round(2))
			}
			row := &models.SiteProfitLedger{SiteID: *order.SiteID, OrderID: order.ID, OrderItemID: &itemID, LedgerType: "order_profit", Amount: models.NewMoneyFromDecimal(profit), Status: status, ConfirmAt: confirmAt, AvailableAt: availableAt, CreatedAt: now, UpdatedAt: now}
			if err := ledgerRepo.Create(row); err != nil {
				return err
			}
		}
		acct.UpdatedAt = now
		return accountRepo.Update(acct)
	})
}

func (s *SiteProfitService) ConfirmDueProfits(now time.Time) error {
	return s.withdrawRepo.Transaction(func(tx *gorm.DB) error {
		ledgerRepo := s.ledgerRepo.WithTx(tx)
		accountRepo := s.accountRepo.WithTx(tx)
		rows, err := ledgerRepo.ListPendingBeforeForUpdate(now)
		if err != nil {
			return err
		}
		for _, row := range rows {
			acct, err := accountRepo.EnsureBySiteID(row.SiteID, now)
			if err != nil {
				return err
			}
			amount := row.Amount.Decimal.Round(2)
			acct.PendingAmount = models.NewMoneyFromDecimal(acct.PendingAmount.Decimal.Sub(amount).Round(2))
			if acct.PendingAmount.Decimal.IsNegative() {
				acct.PendingAmount = models.NewMoneyFromDecimal(decimal.Zero)
			}
			acct.AvailableAmount = models.NewMoneyFromDecimal(acct.AvailableAmount.Decimal.Add(amount).Round(2))
			acct.UpdatedAt = now
			row.Status = constants.SiteProfitStatusAvailable
			row.AvailableAt = &now
			row.UpdatedAt = now
			if err := ledgerRepo.Update(&row); err != nil {
				return err
			}
			if err := accountRepo.Update(acct); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SiteProfitService) HandleOrderCanceled(orderID uint, reason string) error {
	if orderID == 0 {
		return nil
	}
	return s.reverseOrderProfit(orderID, reason)
}

func (s *SiteProfitService) HandleOrderRefundedTx(tx *gorm.DB, order *models.Order, _ decimal.Decimal, _ decimal.Decimal, reason string) error {
	if tx == nil || order == nil {
		return nil
	}
	return s.reverseOrderProfitTx(tx, order.ID, reason)
}

func (s *SiteProfitService) reverseOrderProfit(orderID uint, reason string) error {
	return s.withdrawRepo.Transaction(func(tx *gorm.DB) error {
		return s.reverseOrderProfitTx(tx, orderID, reason)
	})
}

func (s *SiteProfitService) reverseOrderProfitTx(tx *gorm.DB, orderID uint, reason string) error {
	ledgerRepo := s.ledgerRepo.WithTx(tx)
	accountRepo := s.accountRepo.WithTx(tx)
	rows, err := ledgerRepo.ListByOrderForUpdate(orderID, []string{constants.SiteProfitStatusPendingConfirm, constants.SiteProfitStatusAvailable})
	if err != nil || len(rows) == 0 {
		return err
	}
	now := time.Now()
	for _, row := range rows {
		acct, err := accountRepo.EnsureBySiteID(row.SiteID, now)
		if err != nil {
			return err
		}
		amount := row.Amount.Decimal.Round(2)
		if row.Status == constants.SiteProfitStatusPendingConfirm {
			acct.PendingAmount = models.NewMoneyFromDecimal(acct.PendingAmount.Decimal.Sub(amount).Round(2))
			if acct.PendingAmount.Decimal.IsNegative() {
				acct.PendingAmount = models.NewMoneyFromDecimal(decimal.Zero)
			}
		} else if row.Status == constants.SiteProfitStatusAvailable {
			acct.AvailableAmount = models.NewMoneyFromDecimal(acct.AvailableAmount.Decimal.Sub(amount).Round(2))
			if acct.AvailableAmount.Decimal.IsNegative() {
				acct.AvailableAmount = models.NewMoneyFromDecimal(decimal.Zero)
			}
		}
		row.Status = constants.SiteProfitStatusReversed
		row.InvalidReason = strings.TrimSpace(reason)
		row.ConfirmAt = nil
		row.AvailableAt = nil
		row.UpdatedAt = now
		acct.UpdatedAt = now
		if err := ledgerRepo.Update(&row); err != nil {
			return err
		}
		if err := accountRepo.Update(acct); err != nil {
			return err
		}
	}
	return nil
}
