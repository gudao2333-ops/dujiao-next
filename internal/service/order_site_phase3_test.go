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

func setupOrderSitePhase3Test(t *testing.T) (*OrderService, *SiteService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:order_site_phase3_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Setting{}, &models.Category{}, &models.Product{}, &models.ProductSKU{}, &models.Promotion{}, &models.Coupon{}, &models.CouponUsage{}, &models.Order{}, &models.OrderItem{}, &models.Site{}, &models.SiteDomainSuffix{}, &models.SiteProductPrice{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	settingSvc := NewSettingService(repository.NewSettingRepository(db))
	_, _ = settingSvc.Update(constants.SettingKeySiteConfig, map[string]interface{}{constants.SettingFieldSiteCurrency: "CNY"})
	siteSvc := NewSiteService(repository.NewSiteRepository(db), repository.NewSiteDomainSuffixRepository(db), repository.NewSiteProductPriceRepository(db), repository.NewOrderRepository(db), repository.NewProductRepository(db), repository.NewProductSKURepository(db), settingSvc)
	orderSvc := NewOrderService(OrderServiceOptions{
		OrderRepo:       repository.NewOrderRepository(db),
		ProductRepo:     repository.NewProductRepository(db),
		ProductSKURepo:  repository.NewProductSKURepository(db),
		CouponRepo:      repository.NewCouponRepository(db),
		CouponUsageRepo: repository.NewCouponUsageRepository(db),
		PromotionRepo:   repository.NewPromotionRepository(db),
		SettingService:  settingSvc,
		SiteService:     siteSvc,
		ExpireMinutes:   30,
	})
	return orderSvc, siteSvc, db
}

func TestBuildOrderResultSiteAttributionAndSnapshots(t *testing.T) {
	orderSvc, _, db := setupOrderSitePhase3Test(t)
	now := time.Now()
	if err := db.Create(&models.User{ID: 1, Email: "buyer@test.local", PasswordHash: "hash", Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	cat := models.Category{ID: 9201, NameJSON: models.JSON{"zh-CN": "分类"}, Slug: "cat-order-site", CreatedAt: now}
	_ = db.FirstOrCreate(&cat, models.Category{ID: cat.ID}).Error
	product := models.Product{ID: 9202, CategoryID: cat.ID, Slug: "order-site-product", TitleJSON: models.JSON{"zh-CN": "商品"}, PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("10.00")), FulfillmentType: constants.FulfillmentTypeManual, ManualStockTotal: -1, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := models.ProductSKU{ID: 9203, ProductID: product.ID, SKUCode: "SKU-9203", PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("10.00")), ManualStockTotal: -1, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	site := models.Site{ID: 9204, OwnerUserID: 2, Name: "子站", SubdomainPrefix: "sitea", Suffix: ".shop.test", FullDomain: "sitea.shop.test", Status: models.SiteStatusActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&site).Error; err != nil {
		t.Fatalf("create site failed: %v", err)
	}
	if err := db.Create(&models.SiteProductPrice{SiteID: site.ID, ProductID: product.ID, SKUID: sku.ID, SitePrice: models.NewMoneyFromDecimal(decimal.RequireFromString("15.00")), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create site price failed: %v", err)
	}

	result, err := orderSvc.buildOrderResult(orderCreateParams{
		UserID:      1,
		OrderScene:  constants.OrderSceneProduct,
		RequestHost: "sitea.shop.test",
		Items:       []CreateOrderItem{{ProductID: product.ID, SKUID: sku.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("build order result failed: %v", err)
	}
	if result.AttributedSiteID == nil || *result.AttributedSiteID != site.ID {
		t.Fatalf("expected attributed site id %d got %+v", site.ID, result.AttributedSiteID)
	}
	if len(result.OrderItems) != 1 {
		t.Fatalf("unexpected order items len: %d", len(result.OrderItems))
	}
	item := result.OrderItems[0]
	if item.BasePriceSnapshot.Decimal.StringFixed(2) != "10.00" || item.SitePriceSnapshot.Decimal.StringFixed(2) != "15.00" || item.SiteProfitSnapshot.Decimal.StringFixed(2) != "5.00" {
		t.Fatalf("unexpected snapshots: base=%s site=%s profit=%s", item.BasePriceSnapshot.String(), item.SitePriceSnapshot.String(), item.SiteProfitSnapshot.String())
	}
	if item.UnitPrice.Decimal.StringFixed(2) != "15.00" {
		t.Fatalf("expected checkout unit price use site price, got %s", item.UnitPrice.String())
	}
}
