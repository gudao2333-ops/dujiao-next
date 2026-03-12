package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupSiteProfitServiceTest(t *testing.T) (*SiteProfitService, *SettingService, *gorm.DB, *models.Site) {
	t.Helper()
	dsn := fmt.Sprintf("file:site_profit_service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Setting{}, &models.Site{}, &models.Order{}, &models.OrderItem{}, &models.Fulfillment{}, &models.SiteProfitAccount{}, &models.SiteProfitLedger{}, &models.SiteWithdraw{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	settingSvc := NewSettingService(repository.NewSettingRepository(db))
	_, _ = settingSvc.UpdateSiteOpenSetting(SiteOpenSetting{Enabled: true, ProfitConfirmDays: 1, MinWithdrawAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("10")), WithdrawChannels: []string{"alipay"}})
	svc := NewSiteProfitService(repository.NewSiteRepository(db), repository.NewOrderRepository(db), repository.NewSiteProfitAccountRepository(db), repository.NewSiteProfitLedgerRepository(db), repository.NewSiteWithdrawRepository(db), settingSvc)
	now := time.Now()
	u := models.User{ID: 1, Email: "u@test.local", PasswordHash: "x", Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	site := &models.Site{OwnerUserID: 1, Name: "s", SubdomainPrefix: "a", Suffix: ".x", FullDomain: "a.x", Status: models.SiteStatusActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(site).Error; err != nil {
		t.Fatal(err)
	}
	return svc, settingSvc, db, site
}

func TestSiteProfitCreateAndConfirm(t *testing.T) {
	svc, settingSvc, db, site := setupSiteProfitServiceTest(t)
	now := time.Now().Add(-48 * time.Hour)
	order := models.Order{OrderNo: "O1", UserID: 2, SiteID: &site.ID, Status: constants.OrderStatusPaid, OrderScene: constants.OrderSceneProduct, TotalAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("30")), PaidAt: &now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	item := models.OrderItem{OrderID: order.ID, TitleJSON: models.JSON{"zh-CN": "p"}, Quantity: 2, SiteProfitSnapshot: models.NewMoneyFromDecimal(decimal.RequireFromString("5")), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleOrderPaid(order.ID); err != nil {
		t.Fatal(err)
	}
	rows, total, err := svc.ListMyLedgers(1, 1, 20, "")
	if err != nil || total != 1 || rows[0].Status != constants.SiteProfitStatusPendingConfirm {
		t.Fatalf("unexpected ledger: total=%d err=%v", total, err)
	}
	if err := svc.ConfirmDueProfits(time.Now()); err != nil {
		t.Fatal(err)
	}
	rows, _, _ = svc.ListMyLedgers(1, 1, 20, constants.SiteProfitStatusAvailable)
	if len(rows) != 1 {
		t.Fatalf("expected available row")
	}
	cfg, _ := settingSvc.GetSiteOpenSetting()
	if cfg.ProfitConfirmDays != 1 {
		t.Fatal("unexpected setting")
	}
}

func TestSiteProfitWithdrawTransitions(t *testing.T) {
	svc, _, db, site := setupSiteProfitServiceTest(t)
	now := time.Now()
	if err := db.Create(&models.SiteProfitAccount{SiteID: site.ID, AvailableAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("50")), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	l1 := models.SiteProfitLedger{SiteID: site.ID, OrderID: 101, Amount: models.NewMoneyFromDecimal(decimal.RequireFromString("20")), Status: constants.SiteProfitStatusAvailable, CreatedAt: now, UpdatedAt: now}
	l2 := models.SiteProfitLedger{SiteID: site.ID, OrderID: 102, Amount: models.NewMoneyFromDecimal(decimal.RequireFromString("30")), Status: constants.SiteProfitStatusAvailable, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&[]models.SiteProfitLedger{l1, l2}).Error; err != nil {
		t.Fatal(err)
	}
	w, err := svc.ApplyWithdraw(1, SiteWithdrawApplyInput{Amount: decimal.RequireFromString("20"), Channel: "alipay", Account: "a@b"})
	if err != nil || w == nil {
		t.Fatalf("apply withdraw failed: %v", err)
	}
	if _, err := svc.ApplyWithdraw(1, SiteWithdrawApplyInput{Amount: decimal.RequireFromString("1"), Channel: "alipay", Account: "a@b"}); !errors.Is(err, ErrSiteWithdrawAmountInvalid) {
		t.Fatalf("expected min amount invalid")
	}
	if _, err := svc.ReviewWithdraw(99, w.ID, constants.SiteWithdrawActionReject, "r"); err != nil {
		t.Fatal(err)
	}
	w2, err := svc.ApplyWithdraw(1, SiteWithdrawApplyInput{Amount: decimal.RequireFromString("20"), Channel: "alipay", Account: "a@b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewWithdraw(99, w2.ID, constants.SiteWithdrawActionPay, ""); err != nil {
		t.Fatal(err)
	}
}

func TestSiteProfitReverseOnCancel(t *testing.T) {
	svc, _, db, site := setupSiteProfitServiceTest(t)
	now := time.Now()
	if err := db.Create(&models.SiteProfitAccount{SiteID: site.ID, PendingAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("10")), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	led := models.SiteProfitLedger{SiteID: site.ID, OrderID: 10, Amount: models.NewMoneyFromDecimal(decimal.RequireFromString("10")), Status: constants.SiteProfitStatusPendingConfirm, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&led).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleOrderCanceled(10, "cancel"); err != nil {
		t.Fatal(err)
	}
	rows, _, _ := svc.ListMyLedgers(1, 1, 20, constants.SiteProfitStatusReversed)
	if len(rows) != 1 {
		t.Fatal("expected reversed")
	}
}

func TestSiteProfitPermissionAndMaskedOrders(t *testing.T) {
	svc, _, db, site := setupSiteProfitServiceTest(t)
	now := time.Now()
	order := models.Order{OrderNo: "SO", UserID: 2, SiteID: &site.ID, Status: constants.OrderStatusPaid, OrderScene: constants.OrderSceneProduct, TotalAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("10")), CreatedAt: now, UpdatedAt: now}
	_ = db.Create(&order).Error
	_ = db.Create(&models.OrderItem{OrderID: order.ID, TitleJSON: models.JSON{"zh-CN": "p1"}, Quantity: 1, CreatedAt: now, UpdatedAt: now}).Error
	rows, _, err := svc.ListMyOrders(1, 1, 20, "")
	if err != nil || len(rows) != 1 || rows[0].OrderNo == "" {
		t.Fatalf("unexpected rows")
	}
	if _, _, err := svc.ListMyOrders(999, 1, 20, ""); !errors.Is(err, ErrSiteNotOpened) {
		t.Fatalf("expected permission/not-opened error")
	}
}
