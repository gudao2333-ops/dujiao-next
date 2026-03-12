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

func setupSiteServiceTest(t *testing.T) (*SiteService, *SettingService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:site_service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Setting{}, &models.Order{}, &models.OrderItem{}, &models.Fulfillment{}, &models.Site{}, &models.SiteDomainSuffix{}, &models.Product{}, &models.ProductSKU{}, &models.Category{}, &models.SiteProductPrice{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	models.DB = db
	settingSvc := NewSettingService(repository.NewSettingRepository(db))
	_, err = settingSvc.UpdateSiteOpenSetting(SiteOpenSetting{
		Enabled:          true,
		OpeningPrice:     models.NewMoneyFromDecimal(decimal.RequireFromString("99.00")),
		DomainSuffixes:   []string{".shop.test", ".a.test"},
		ReservedPrefixes: []string{"admin", "api"},
		PrefixRegex:      `^[a-z][a-z0-9-]{1,15}$`,
	})
	if err != nil {
		t.Fatalf("update site setting failed: %v", err)
	}
	svc := NewSiteService(repository.NewSiteRepository(db), repository.NewSiteDomainSuffixRepository(db), repository.NewSiteProductPriceRepository(db), repository.NewOrderRepository(db), repository.NewProductRepository(db), repository.NewProductSKURepository(db), settingSvc)
	return svc, settingSvc, db
}

func createSiteTestUser(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	if err := db.Create(&models.User{ID: id, Email: fmt.Sprintf("u%d@test.local", id), PasswordHash: "hash", Status: constants.UserStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
}

func seedSiteTestSKU(t *testing.T, db *gorm.DB, productID, skuID uint, price string) {
	t.Helper()
	cat := models.Category{ID: 9001, NameJSON: models.JSON{"zh-CN": "分类"}, Slug: "site-test-cat", CreatedAt: time.Now()}
	_ = db.FirstOrCreate(&cat, models.Category{ID: cat.ID}).Error
	product := models.Product{ID: productID, CategoryID: cat.ID, Slug: fmt.Sprintf("site-test-product-%d", productID), TitleJSON: models.JSON{"zh-CN": "测试商品"}, PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString(price)), FulfillmentType: constants.FulfillmentTypeManual, ManualStockTotal: -1, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := models.ProductSKU{ID: skuID, ProductID: productID, SKUCode: fmt.Sprintf("SKU-%d", skuID), PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString(price)), ManualStockTotal: -1, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
}

func TestSiteServicePrefixValidationAndReserved(t *testing.T) {
	svc, _, db := setupSiteServiceTest(t)
	createSiteTestUser(t, db, 1)

	if _, err := svc.PreviewOpen(SiteOpenPreviewInput{UserID: 1, SiteName: "test", SubdomainPrefix: "A", SelectedSuffix: ".shop.test"}); err == nil {
		t.Fatalf("expected invalid prefix error")
	}
	if _, err := svc.PreviewOpen(SiteOpenPreviewInput{UserID: 1, SiteName: "test", SubdomainPrefix: "admin", SelectedSuffix: ".shop.test"}); err == nil {
		t.Fatalf("expected reserved prefix error")
	}
}

func TestSiteServiceUniquenessAndOneUserOneSite(t *testing.T) {
	svc, _, db := setupSiteServiceTest(t)
	createSiteTestUser(t, db, 1)
	createSiteTestUser(t, db, 2)

	site := &models.Site{OwnerUserID: 1, Name: "s", SubdomainPrefix: "abc", Suffix: ".shop.test", FullDomain: "abc.shop.test", Status: models.SiteStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(site).Error; err != nil {
		t.Fatalf("seed site failed: %v", err)
	}

	if _, err := svc.PreviewOpen(SiteOpenPreviewInput{UserID: 2, SiteName: "u2", SubdomainPrefix: "abc", SelectedSuffix: ".shop.test"}); err == nil {
		t.Fatalf("expected prefix exists error")
	}
	if _, err := svc.PreviewOpen(SiteOpenPreviewInput{UserID: 2, SiteName: "u2", SubdomainPrefix: "abcd", SelectedSuffix: ".shop.test"}); err != nil {
		t.Fatalf("preview should pass: %v", err)
	}
	if _, err := svc.PreviewOpen(SiteOpenPreviewInput{UserID: 1, SiteName: "u1", SubdomainPrefix: "newx", SelectedSuffix: ".shop.test"}); err == nil {
		t.Fatalf("expected one-user-one-site error")
	}
}

func TestSiteServiceCreateOpenOrderAndPaidCreateSiteIdempotent(t *testing.T) {
	svc, _, db := setupSiteServiceTest(t)
	createSiteTestUser(t, db, 3)

	created, err := svc.CreateOpenOrder(SiteOpenPreviewInput{UserID: 3, SiteName: "My Site", SubdomainPrefix: "myshop", SelectedSuffix: ".shop.test"})
	if err != nil {
		t.Fatalf("create open order failed: %v", err)
	}
	if created.OrderID == 0 {
		t.Fatalf("expected order id")
	}

	if err := db.Model(&models.Order{}).Where("id = ?", created.OrderID).Updates(map[string]interface{}{"status": constants.OrderStatusPaid, "paid_at": time.Now()}).Error; err != nil {
		t.Fatalf("mark paid failed: %v", err)
	}
	if err := svc.HandleOrderPaid(created.OrderID); err != nil {
		t.Fatalf("handle paid failed: %v", err)
	}
	if err := svc.HandleOrderPaid(created.OrderID); err != nil {
		t.Fatalf("second handle paid should be idempotent: %v", err)
	}

	var count int64
	if err := db.Model(&models.Site{}).Where("owner_user_id = ?", 3).Count(&count).Error; err != nil {
		t.Fatalf("count sites failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one site, got %d", count)
	}
}

func TestSiteServiceResolveByHostAndSetPriceValidation(t *testing.T) {
	svc, _, db := setupSiteServiceTest(t)
	createSiteTestUser(t, db, 10)
	seedSiteTestSKU(t, db, 9100, 9101, "20.00")

	site := &models.Site{OwnerUserID: 10, Name: "s", SubdomainPrefix: "mys", Suffix: ".shop.test", FullDomain: "mys.shop.test", Status: models.SiteStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(site).Error; err != nil {
		t.Fatalf("seed site failed: %v", err)
	}

	attr, err := svc.ResolveSiteByHost("mys.shop.test:8080")
	if err != nil {
		t.Fatalf("resolve by host failed: %v", err)
	}
	if attr == nil || attr.SiteID == nil || *attr.SiteID != site.ID {
		t.Fatalf("unexpected host attribution: %+v", attr)
	}

	if _, err := svc.SetSiteSKUPrice(SiteSetPriceInput{OwnerUserID: 10, ProductID: 9100, SKUID: 9101, SitePrice: models.NewMoneyFromDecimal(decimal.RequireFromString("19.99"))}); err == nil {
		t.Fatalf("expected site price validation error")
	}
	row, err := svc.SetSiteSKUPrice(SiteSetPriceInput{OwnerUserID: 10, ProductID: 9100, SKUID: 9101, SitePrice: models.NewMoneyFromDecimal(decimal.RequireFromString("25.00"))})
	if err != nil {
		t.Fatalf("set site sku price failed: %v", err)
	}
	if row == nil || row.SitePrice.Decimal.StringFixed(2) != "25.00" {
		t.Fatalf("unexpected site price row: %+v", row)
	}
}

func TestSiteServiceAdminSuffixAndStatus(t *testing.T) {
	svc, _, db := setupSiteServiceTest(t)
	createSiteTestUser(t, db, 20)
	site := &models.Site{OwnerUserID: 20, Name: "admin-site", SubdomainPrefix: "adminsite", Suffix: ".shop.test", FullDomain: "adminsite.shop.test", Status: models.SiteStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(site).Error; err != nil {
		t.Fatalf("seed site failed: %v", err)
	}

	created, err := svc.CreateAdminSuffix(AdminSiteSuffixInput{Suffix: "new.test", IsEnabled: true, SortOrder: 10})
	if err != nil || created == nil {
		t.Fatalf("create suffix failed: %v", err)
	}
	updated, err := svc.UpdateAdminSuffix(created.ID, AdminSiteSuffixInput{Suffix: ".newer.test", IsEnabled: false, SortOrder: 20})
	if err != nil || updated == nil || updated.Suffix != ".newer.test" {
		t.Fatalf("update suffix failed: %+v err=%v", updated, err)
	}
	if err := svc.DeleteAdminSuffix(created.ID); err != nil {
		t.Fatalf("delete suffix failed: %v", err)
	}

	if _, err := svc.UpdateAdminSiteStatus(site.ID, models.SiteStatusDisabled); err != nil {
		t.Fatalf("update site status failed: %v", err)
	}
	rows, total, err := svc.ListAdminSites(AdminSiteListFilter{Page: 1, PageSize: 20, Status: models.SiteStatusDisabled})
	if err != nil || total < 1 || len(rows) < 1 {
		t.Fatalf("list admin sites failed: total=%d len=%d err=%v", total, len(rows), err)
	}
}
