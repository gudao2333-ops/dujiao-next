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

func setupGiftCardServiceTest(t *testing.T) (*GiftCardService, *WalletService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:gift_card_service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Order{},
		&models.OrderItem{},
		&models.Fulfillment{},
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.Coupon{},
		&models.CouponUsage{},
		&models.Promotion{},
		&models.WalletAccount{},
		&models.WalletTransaction{},
		&models.Setting{},
		&models.GiftCardBatch{},
		&models.GiftCard{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	models.DB = db

	userRepo := repository.NewUserRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	settingSvc := NewSettingService(settingRepo)
	walletSvc := NewWalletService(repository.NewWalletRepository(db), repository.NewOrderRepository(db), userRepo, nil)
	orderSvc := NewOrderService(OrderServiceOptions{
		OrderRepo:       repository.NewOrderRepository(db),
		ProductRepo:     repository.NewProductRepository(db),
		ProductSKURepo:  repository.NewProductSKURepository(db),
		CouponRepo:      repository.NewCouponRepository(db),
		CouponUsageRepo: repository.NewCouponUsageRepository(db),
		PromotionRepo:   repository.NewPromotionRepository(db),
		SettingService:  settingSvc,
		ExpireMinutes:   30,
	})
	fulfillSvc := NewFulfillmentService(repository.NewOrderRepository(db), repository.NewFulfillmentRepository(db), repository.NewCardSecretRepository(db), nil, repository.NewUserOAuthIdentityRepository(db))
	giftSvc := NewGiftCardService(repository.NewGiftCardRepository(db), userRepo, repository.NewProductRepository(db), repository.NewProductSKURepository(db), walletSvc, settingSvc, orderSvc, fulfillSvc)
	return giftSvc, walletSvc, db
}

func seedRedeemProduct(t *testing.T, db *gorm.DB, productID uint, skuID uint, active bool) {
	t.Helper()
	now := time.Now()
	category := models.Category{ID: 7001, NameJSON: models.JSON{"zh-CN": "分类"}, Slug: "cat-gift", CreatedAt: now}
	_ = db.FirstOrCreate(&category, models.Category{ID: category.ID}).Error
	product := models.Product{ID: productID, CategoryID: category.ID, Slug: fmt.Sprintf("gift-product-%d", productID), TitleJSON: models.JSON{"zh-CN": "商品"}, PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("9.90")), FulfillmentType: constants.FulfillmentTypeManual, ManualStockTotal: -1, IsActive: active, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	if !active {
		if err := db.Model(&models.Product{}).Where("id = ?", product.ID).Update("is_active", false).Error; err != nil {
			t.Fatalf("disable product failed: %v", err)
		}
	}
	sku := models.ProductSKU{ID: skuID, ProductID: product.ID, SKUCode: fmt.Sprintf("SKU-%d", skuID), PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("9.90")), ManualStockTotal: -1, IsActive: active, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	if !active {
		if err := db.Model(&models.ProductSKU{}).Where("id = ?", sku.ID).Update("is_active", false).Error; err != nil {
			t.Fatalf("disable sku failed: %v", err)
		}
	}
}

func seedGiftCardUser(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	user := models.User{
		ID:           id,
		Email:        fmt.Sprintf("gift_card_user_%d@example.com", id),
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
}

func TestGiftCardServiceGenerateGiftCards(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	adminID := uint(999)
	batch, created, err := svc.GenerateGiftCards(GenerateGiftCardsInput{
		Name:      "测试礼品卡",
		Quantity:  3,
		Amount:    models.NewMoneyFromDecimal(decimal.RequireFromString("25.00")),
		CreatedBy: &adminID,
	})
	if err != nil {
		t.Fatalf("generate gift cards failed: %v", err)
	}
	if batch == nil || batch.ID == 0 {
		t.Fatalf("invalid batch result: %+v", batch)
	}
	if created != 3 {
		t.Fatalf("expected created=3, got: %d", created)
	}

	var count int64
	if err := db.Model(&models.GiftCard{}).Where("batch_id = ?", batch.ID).Count(&count).Error; err != nil {
		t.Fatalf("count gift cards failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 gift cards in batch, got: %d", count)
	}
}

func TestGiftCardServiceGenerateGiftCardsUsesSiteCurrency(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	settingRepo := repository.NewSettingRepository(db)
	settingSvc := NewSettingService(settingRepo)
	_, err := settingSvc.Update(constants.SettingKeySiteConfig, map[string]interface{}{
		constants.SettingFieldSiteCurrency: "USD",
	})
	if err != nil {
		t.Fatalf("set site currency failed: %v", err)
	}

	batch, created, err := svc.GenerateGiftCards(GenerateGiftCardsInput{
		Name:     "站点币种礼品卡",
		Quantity: 2,
		Amount:   models.NewMoneyFromDecimal(decimal.RequireFromString("9.90")),
	})
	if err != nil {
		t.Fatalf("generate gift cards failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected created=2, got: %d", created)
	}
	if batch == nil {
		t.Fatal("batch should not be nil")
	}
	if batch.Currency != "USD" {
		t.Fatalf("expected batch currency USD, got: %s", batch.Currency)
	}

	var cards []models.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).Find(&cards).Error; err != nil {
		t.Fatalf("query gift cards failed: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 gift cards, got: %d", len(cards))
	}
	for _, card := range cards {
		if card.Currency != "USD" {
			t.Fatalf("expected card currency USD, got: %s", card.Currency)
		}
	}
}

func TestGiftCardServiceGenerateGiftCardsModeValidation(t *testing.T) {
	svc, _, _ := setupGiftCardServiceTest(t)
	if _, _, err := svc.GenerateGiftCards(GenerateGiftCardsInput{Name: "混合模式", Quantity: 1, Amount: models.NewMoneyFromDecimal(decimal.RequireFromString("1.00")), ProductID: 8001, SKUID: 8101}); !errors.Is(err, ErrGiftCardInvalid) {
		t.Fatalf("expected mixed mode invalid, got: %v", err)
	}
	if _, _, err := svc.GenerateGiftCards(GenerateGiftCardsInput{Name: "缺 sku", Quantity: 1, ProductID: 8001}); !errors.Is(err, ErrGiftCardInvalid) {
		t.Fatalf("expected product mode missing sku invalid, got: %v", err)
	}
	if _, _, err := svc.GenerateGiftCards(GenerateGiftCardsInput{Name: "缺 product", Quantity: 1, SKUID: 8101}); !errors.Is(err, ErrGiftCardInvalid) {
		t.Fatalf("expected product mode missing product invalid, got: %v", err)
	}
}

func TestGiftCardServiceRedeemGiftCard(t *testing.T) {
	svc, walletSvc, db := setupGiftCardServiceTest(t)
	userID := uint(2001)
	seedGiftCardUser(t, db, userID)

	batch, _, err := svc.GenerateGiftCards(GenerateGiftCardsInput{
		Name:     "兑换测试卡",
		Quantity: 1,
		Amount:   models.NewMoneyFromDecimal(decimal.RequireFromString("59.90")),
	})
	if err != nil {
		t.Fatalf("generate gift card failed: %v", err)
	}

	var card models.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&card).Error; err != nil {
		t.Fatalf("query generated card failed: %v", err)
	}

	result, err := svc.RedeemGiftCard(GiftCardRedeemInput{
		UserID: userID,
		Code:   card.Code,
	})
	if err != nil {
		t.Fatalf("redeem gift card failed: %v", err)
	}
	redeemedCard := result.Card
	account := result.WalletAccount
	txn := result.WalletTransaction
	if redeemedCard == nil || redeemedCard.Status != models.GiftCardStatusRedeemed {
		t.Fatalf("unexpected redeemed card: %+v", redeemedCard)
	}
	if account == nil || !account.Balance.Decimal.Equal(decimal.RequireFromString("59.90")) {
		t.Fatalf("unexpected wallet account: %+v", account)
	}
	if txn == nil || txn.Type != constants.WalletTxnTypeGiftCard {
		t.Fatalf("unexpected wallet transaction: %+v", txn)
	}

	_, err = svc.RedeemGiftCard(GiftCardRedeemInput{
		UserID: userID,
		Code:   card.Code,
	})
	if !errors.Is(err, ErrGiftCardRedeemed) {
		t.Fatalf("expected ErrGiftCardRedeemed, got: %v", err)
	}

	accountAfter, err := walletSvc.GetAccount(userID)
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	if !accountAfter.Balance.Decimal.Equal(decimal.RequireFromString("59.90")) {
		t.Fatalf("unexpected account balance after duplicate redeem: %s", accountAfter.Balance.String())
	}
}

func TestGiftCardServiceRedeemExpiredGiftCard(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2002)
	seedGiftCardUser(t, db, userID)
	expiredAt := time.Now().Add(-1 * time.Hour)

	card := models.GiftCard{
		Name:      "过期礼品卡",
		Code:      "GC-EXPIRED-001",
		Amount:    models.NewMoneyFromDecimal(decimal.RequireFromString("10.00")),
		Currency:  "CNY",
		Status:    models.GiftCardStatusActive,
		ExpiresAt: &expiredAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create expired gift card failed: %v", err)
	}

	_, err := svc.RedeemGiftCard(GiftCardRedeemInput{
		UserID: userID,
		Code:   card.Code,
	})
	if !errors.Is(err, ErrGiftCardExpired) {
		t.Fatalf("expected ErrGiftCardExpired, got: %v", err)
	}
}

func TestGiftCardServiceRedeemGiftCardProductMode(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2010)
	seedGiftCardUser(t, db, userID)
	seedRedeemProduct(t, db, 8001, 8101, true)

	batch, _, err := svc.GenerateGiftCards(GenerateGiftCardsInput{Name: "商品兑换码", Quantity: 1, ProductID: 8001, SKUID: 8101})
	if err != nil {
		t.Fatalf("generate product code failed: %v", err)
	}
	var card models.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&card).Error; err != nil {
		t.Fatalf("query generated card failed: %v", err)
	}
	result, err := svc.RedeemGiftCard(GiftCardRedeemInput{UserID: userID, Code: card.Code})
	if err != nil {
		t.Fatalf("redeem product code failed: %v", err)
	}
	if result.OrderID == 0 || result.OrderNo == "" || result.RedeemMode != models.GiftCardRedeemModeProduct {
		t.Fatalf("unexpected redeem result: %+v", result)
	}
	var item models.OrderItem
	if err := db.Where("order_id = ?", result.OrderID).First(&item).Error; err != nil {
		t.Fatalf("query redeem order item failed: %v", err)
	}
	if item.SiteProfitSnapshot.Decimal.StringFixed(2) != "0.00" || item.SitePriceSnapshot.Decimal.StringFixed(2) != "0.00" || item.BasePriceSnapshot.Decimal.StringFixed(2) != "0.00" {
		t.Fatalf("redeem order should not carry site profit snapshots: base=%s site=%s profit=%s", item.BasePriceSnapshot.String(), item.SitePriceSnapshot.String(), item.SiteProfitSnapshot.String())
	}
	if result.WalletAccount != nil || result.WalletTransaction != nil {
		t.Fatalf("product redeem should not credit wallet")
	}
	var fulfillmentCount int64
	if err := db.Model(&models.Fulfillment{}).Where("order_id = ?", result.OrderID).Count(&fulfillmentCount).Error; err != nil {
		t.Fatalf("count fulfillment failed: %v", err)
	}
	if fulfillmentCount != 0 {
		t.Fatalf("manual fulfillment product redeem should not auto create fulfillment")
	}
}

func TestGiftCardServiceRedeemGiftCardProductModeInvalidSKU(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2011)
	seedGiftCardUser(t, db, userID)
	seedRedeemProduct(t, db, 8002, 8102, false)
	card := models.GiftCard{Name: "无效商品兑换码", Code: "GC-PRODUCT-INVALID-001", Amount: models.NewMoneyFromDecimal(decimal.Zero), Currency: "CNY", Status: models.GiftCardStatusActive, ProductID: nullableUint(8002), SKUID: nullableUint(8102), RedeemMode: models.GiftCardRedeemModeProduct, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card failed: %v", err)
	}
	_, err := svc.RedeemGiftCard(GiftCardRedeemInput{UserID: userID, Code: card.Code})
	if !errors.Is(err, ErrGiftCardRedeemTargetInvalid) {
		t.Fatalf("expected ErrGiftCardRedeemTargetInvalid, got: %v", err)
	}
}

func TestGiftCardServiceRedeemGiftCardProductModeDuplicatePrevention(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2012)
	seedGiftCardUser(t, db, userID)
	seedRedeemProduct(t, db, 8010, 8110, true)
	batch, _, err := svc.GenerateGiftCards(GenerateGiftCardsInput{Name: "商品兑换码重复", Quantity: 1, ProductID: 8010, SKUID: 8110})
	if err != nil {
		t.Fatalf("generate product code failed: %v", err)
	}
	var card models.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&card).Error; err != nil {
		t.Fatalf("query generated card failed: %v", err)
	}
	first, err := svc.RedeemGiftCard(GiftCardRedeemInput{UserID: userID, Code: card.Code})
	if err != nil {
		t.Fatalf("first redeem failed: %v", err)
	}
	if _, err = svc.RedeemGiftCard(GiftCardRedeemInput{UserID: userID, Code: card.Code}); !errors.Is(err, ErrGiftCardRedeemed) {
		t.Fatalf("expected ErrGiftCardRedeemed, got: %v", err)
	}
	var orderCount int64
	if err := db.Model(&models.Order{}).Where("id = ?", first.OrderID).Count(&orderCount).Error; err != nil {
		t.Fatalf("count order failed: %v", err)
	}
	if orderCount != 1 {
		t.Fatalf("expected only one redeem order, got: %d", orderCount)
	}
}

func TestGiftCardServiceRedeemGiftCardProductModeExpired(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2013)
	seedGiftCardUser(t, db, userID)
	seedRedeemProduct(t, db, 8020, 8120, true)
	expiredAt := time.Now().Add(-1 * time.Minute)
	card := models.GiftCard{Name: "过期商品兑换码", Code: "GC-PRODUCT-EXPIRED-001", Amount: models.NewMoneyFromDecimal(decimal.Zero), Currency: "CNY", Status: models.GiftCardStatusActive, ProductID: nullableUint(8020), SKUID: nullableUint(8120), RedeemMode: models.GiftCardRedeemModeProduct, ExpiresAt: &expiredAt, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card failed: %v", err)
	}
	_, err := svc.RedeemGiftCard(GiftCardRedeemInput{UserID: userID, Code: card.Code})
	if !errors.Is(err, ErrGiftCardExpired) {
		t.Fatalf("expected ErrGiftCardExpired, got: %v", err)
	}
}

func TestGiftCardServiceRedeemGiftCardProductModeMismatch(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2014)
	seedGiftCardUser(t, db, userID)
	seedRedeemProduct(t, db, 8030, 8130, true)
	seedRedeemProduct(t, db, 8040, 8140, true)
	card := models.GiftCard{Name: "商品SKU不匹配兑换码", Code: "GC-PRODUCT-MISMATCH-001", Amount: models.NewMoneyFromDecimal(decimal.Zero), Currency: "CNY", Status: models.GiftCardStatusActive, ProductID: nullableUint(8030), SKUID: nullableUint(8140), RedeemMode: models.GiftCardRedeemModeProduct, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card failed: %v", err)
	}
	_, err := svc.RedeemGiftCard(GiftCardRedeemInput{UserID: userID, Code: card.Code})
	if !errors.Is(err, ErrGiftCardRedeemTargetInvalid) {
		t.Fatalf("expected ErrGiftCardRedeemTargetInvalid, got: %v", err)
	}
}

func TestGiftCardServiceRedeemGiftCardProductModeOutOfStock(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	userID := uint(2015)
	seedGiftCardUser(t, db, userID)
	now := time.Now()
	category := models.Category{ID: 9001, NameJSON: models.JSON{"zh-CN": "分类"}, Slug: "cat-gift-stock", CreatedAt: now}
	_ = db.FirstOrCreate(&category, models.Category{ID: category.ID}).Error
	product := models.Product{ID: 8050, CategoryID: category.ID, Slug: "gift-product-stock-8050", TitleJSON: models.JSON{"zh-CN": "库存商品"}, PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("9.90")), FulfillmentType: constants.FulfillmentTypeManual, ManualStockTotal: 0, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := models.ProductSKU{ID: 8150, ProductID: product.ID, SKUCode: "SKU-8150", PriceAmount: models.NewMoneyFromDecimal(decimal.RequireFromString("9.90")), ManualStockTotal: 0, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	card := models.GiftCard{Name: "缺货兑换码", Code: "GC-PRODUCT-STOCK-001", Amount: models.NewMoneyFromDecimal(decimal.Zero), Currency: "CNY", Status: models.GiftCardStatusActive, ProductID: nullableUint(8050), SKUID: nullableUint(8150), RedeemMode: models.GiftCardRedeemModeProduct, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card failed: %v", err)
	}
	_, err := svc.RedeemGiftCard(GiftCardRedeemInput{UserID: userID, Code: card.Code})
	if !errors.Is(err, ErrManualStockInsufficient) {
		t.Fatalf("expected ErrManualStockInsufficient, got: %v", err)
	}
}

func TestGiftCardServiceBatchUpdateStatusSkipsRedeemed(t *testing.T) {
	svc, _, db := setupGiftCardServiceTest(t)
	now := time.Now()
	userID := uint(2003)
	seedGiftCardUser(t, db, userID)

	activeCard := models.GiftCard{
		Name:      "可变更礼品卡",
		Code:      "GC-BATCH-ACTIVE-001",
		Amount:    models.NewMoneyFromDecimal(decimal.RequireFromString("20.00")),
		Currency:  "CNY",
		Status:    models.GiftCardStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&activeCard).Error; err != nil {
		t.Fatalf("create active card failed: %v", err)
	}

	redeemedAt := now.Add(-10 * time.Minute)
	redeemedCard := models.GiftCard{
		Name:           "已兑换礼品卡",
		Code:           "GC-BATCH-REDEEMED-001",
		Amount:         models.NewMoneyFromDecimal(decimal.RequireFromString("30.00")),
		Currency:       "CNY",
		Status:         models.GiftCardStatusRedeemed,
		RedeemedAt:     &redeemedAt,
		RedeemedUserID: &userID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(&redeemedCard).Error; err != nil {
		t.Fatalf("create redeemed card failed: %v", err)
	}

	affected, err := svc.BatchUpdateStatus([]uint{activeCard.ID, redeemedCard.ID}, models.GiftCardStatusDisabled)
	if err != nil {
		t.Fatalf("batch update status failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected affected=1, got: %d", affected)
	}

	var checkActive models.GiftCard
	if err := db.First(&checkActive, activeCard.ID).Error; err != nil {
		t.Fatalf("query active card failed: %v", err)
	}
	if checkActive.Status != models.GiftCardStatusDisabled {
		t.Fatalf("expected active card status disabled, got: %s", checkActive.Status)
	}

	var checkRedeemed models.GiftCard
	if err := db.First(&checkRedeemed, redeemedCard.ID).Error; err != nil {
		t.Fatalf("query redeemed card failed: %v", err)
	}
	if checkRedeemed.Status != models.GiftCardStatusRedeemed {
		t.Fatalf("expected redeemed card status unchanged, got: %s", checkRedeemed.Status)
	}
}
