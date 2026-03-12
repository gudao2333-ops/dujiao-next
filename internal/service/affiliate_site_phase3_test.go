package service

import (
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

func setupAffiliateSitePhase3Test(t *testing.T) (*AffiliateService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:affiliate_site_phase3_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Setting{}, &models.Product{}, &models.Category{}, &models.ProductSKU{}, &models.Order{}, &models.OrderItem{}, &models.Fulfillment{}, &models.AffiliateProfile{}, &models.AffiliateCommission{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	settingSvc := NewSettingService(repository.NewSettingRepository(db))
	_, _ = settingSvc.UpdateAffiliateSetting(AffiliateSetting{Enabled: true, CommissionRate: 20})
	svc := NewAffiliateService(repository.NewAffiliateRepository(db), repository.NewUserRepository(db), repository.NewOrderRepository(db), repository.NewProductRepository(db), settingSvc)
	return svc, db
}

func TestAffiliateHandleOrderPaidSuppressWhenSiteAttributedProductOrder(t *testing.T) {
	svc, db := setupAffiliateSitePhase3Test(t)
	now := time.Now()
	buyer := models.User{ID: 1001, Email: "buyer1@test", PasswordHash: "h", Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	promoter := models.User{ID: 1002, Email: "promoter1@test", PasswordHash: "h", Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	_ = db.Create(&buyer).Error
	_ = db.Create(&promoter).Error
	profile := models.AffiliateProfile{UserID: promoter.ID, AffiliateCode: "AFFX01", Status: constants.AffiliateProfileStatusActive, CreatedAt: now, UpdatedAt: now}
	_ = db.Create(&profile).Error
	cat := models.Category{ID: 1010, NameJSON: models.JSON{"zh-CN": "c"}, Slug: "c1", CreatedAt: now}
	_ = db.Create(&cat).Error
	product := models.Product{ID: 1011, CategoryID: cat.ID, Slug: "p1", TitleJSON: models.JSON{"zh-CN": "p"}, PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("10")), IsAffiliateEnabled: true, FulfillmentType: constants.FulfillmentTypeManual, ManualStockTotal: -1, IsActive: true, CreatedAt: now, UpdatedAt: now}
	_ = db.Create(&product).Error
	order := models.Order{ID: 1020, OrderNo: "ORD1020", UserID: buyer.ID, Status: constants.OrderStatusPaid, Currency: "CNY", TotalAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("15")), OnlinePaidAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("15")), OrderScene: constants.OrderSceneProduct, SiteID: ptrUint(99), AffiliateProfileID: &profile.ID, AffiliateCode: profile.AffiliateCode, PaidAt: &now, CreatedAt: now, UpdatedAt: now}
	_ = db.Create(&order).Error
	item := models.OrderItem{OrderID: order.ID, ProductID: product.ID, SKUID: 1, TitleJSON: models.JSON{"zh-CN": "p"}, UnitPrice: models.NewMoneyFromDecimal(decimal.RequireFromString("15")), Quantity: 1, TotalPrice: models.NewMoneyFromDecimal(decimal.RequireFromString("15")), CouponDiscount: models.NewMoneyFromDecimal(decimal.Zero), FulfillmentType: constants.FulfillmentTypeManual, CreatedAt: now, UpdatedAt: now}
	_ = db.Create(&item).Error

	if err := svc.HandleOrderPaid(order.ID); err != nil {
		t.Fatalf("handle order paid failed: %v", err)
	}
	var cnt int64
	if err := db.Model(&models.AffiliateCommission{}).Where("order_id = ?", order.ID).Count(&cnt).Error; err != nil {
		t.Fatalf("count commission failed: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("expected no commission for site-attributed product order")
	}
}

func ptrUint(v uint) *uint { return &v }
