package service

import (
	"errors"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SiteService struct {
	repo           repository.SiteRepository
	suffixRepo     repository.SiteDomainSuffixRepository
	priceRepo      repository.SiteProductPriceRepository
	orderRepo      repository.OrderRepository
	productRepo    repository.ProductRepository
	productSKURepo repository.ProductSKURepository
	settingSvc     *SettingService
}

type SiteOpenPreviewInput struct {
	UserID          uint
	SiteName        string
	SubdomainPrefix string
	SelectedSuffix  string
}

type SiteOpenPreviewResult struct {
	SiteName        string       `json:"site_name"`
	SubdomainPrefix string       `json:"subdomain_prefix"`
	Suffix          string       `json:"suffix"`
	FullDomain      string       `json:"full_domain"`
	OpeningPrice    models.Money `json:"opening_price"`
	Currency        string       `json:"currency"`
}

type SiteOpenCreateResult struct {
	Preview SiteOpenPreviewResult `json:"preview"`
	OrderID uint                  `json:"order_id"`
	OrderNo string                `json:"order_no"`
}

type SiteAttributionResult struct {
	SiteID     *uint
	FullDomain string
}

type SiteSetPriceInput struct {
	OwnerUserID uint
	ProductID   uint
	SKUID       uint
	SitePrice   models.Money
}

type SitePriceItem struct {
	SiteID    uint         `json:"site_id"`
	ProductID uint         `json:"product_id"`
	SKUID     uint         `json:"sku_id"`
	SitePrice models.Money `json:"site_price"`
}

func NewSiteService(repo repository.SiteRepository, suffixRepo repository.SiteDomainSuffixRepository, priceRepo repository.SiteProductPriceRepository, orderRepo repository.OrderRepository, productRepo repository.ProductRepository, productSKURepo repository.ProductSKURepository, settingSvc *SettingService) *SiteService {
	return &SiteService{repo: repo, suffixRepo: suffixRepo, priceRepo: priceRepo, orderRepo: orderRepo, productRepo: productRepo, productSKURepo: productSKURepo, settingSvc: settingSvc}
}

func (s *SiteService) PreviewOpen(input SiteOpenPreviewInput) (*SiteOpenPreviewResult, error) {
	if input.UserID == 0 || s.repo == nil {
		return nil, ErrNotFound
	}
	cfg, err := s.settingSvc.GetSiteOpenSetting()
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrSiteOpenDisabled
	}
	if _, err := s.validateOpenInput(input.UserID, input.SiteName, input.SubdomainPrefix, input.SelectedSuffix, cfg); err != nil {
		return nil, err
	}
	prefix := strings.ToLower(strings.TrimSpace(input.SubdomainPrefix))
	suffix := normalizeSuffix(input.SelectedSuffix)
	return &SiteOpenPreviewResult{
		SiteName:        strings.TrimSpace(input.SiteName),
		SubdomainPrefix: prefix,
		Suffix:          suffix,
		FullDomain:      buildFullDomain(prefix, suffix),
		OpeningPrice:    cfg.OpeningPrice,
		Currency:        s.resolveSiteCurrency(),
	}, nil
}

func (s *SiteService) CreateOpenOrder(input SiteOpenPreviewInput) (*SiteOpenCreateResult, error) {
	preview, err := s.PreviewOpen(input)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expireMinutes := 15
	if s.settingSvc != nil {
		if minutes, e := s.settingSvc.GetOrderPaymentExpireMinutes(expireMinutes); e == nil && minutes > 0 {
			expireMinutes = minutes
		}
	}
	expiresAt := now.Add(time.Duration(expireMinutes) * time.Minute)
	order := &models.Order{
		OrderNo:                 generateOrderNo(),
		UserID:                  input.UserID,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                preview.Currency,
		OriginalAmount:          preview.OpeningPrice,
		DiscountAmount:          models.NewMoneyFromDecimal(decimal.Zero),
		PromotionDiscountAmount: models.NewMoneyFromDecimal(decimal.Zero),
		TotalAmount:             preview.OpeningPrice,
		WalletPaidAmount:        models.NewMoneyFromDecimal(decimal.Zero),
		OnlinePaidAmount:        preview.OpeningPrice,
		RefundedAmount:          models.NewMoneyFromDecimal(decimal.Zero),
		OrderScene:              constants.OrderSceneSiteOpening,
		GuestLocale:             strings.TrimSpace(input.SiteName),
		GuestPassword:           preview.SubdomainPrefix,
		GuestEmail:              preview.FullDomain,
		ClientIP:                preview.Suffix,
		ExpiresAt:               &expiresAt,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := s.orderRepo.Create(order, nil); err != nil {
		return nil, ErrOrderCreateFailed
	}
	return &SiteOpenCreateResult{Preview: *preview, OrderID: order.ID, OrderNo: order.OrderNo}, nil
}

// HandleOrderPaid 订单支付后创建子站（幂等）
func (s *SiteService) HandleOrderPaid(orderID uint) error {
	if orderID == 0 || s.orderRepo == nil || s.repo == nil {
		return nil
	}
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil || order == nil {
		return err
	}
	if strings.TrimSpace(order.OrderScene) != constants.OrderSceneSiteOpening {
		return nil
	}
	if strings.TrimSpace(order.Status) != constants.OrderStatusPaid && strings.TrimSpace(order.Status) != constants.OrderStatusFulfilling {
		return nil
	}
	if order.UserID == 0 {
		return nil
	}

	cfg, err := s.settingSvc.GetSiteOpenSetting()
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}

	name := strings.TrimSpace(order.GuestLocale)
	fullDomain := strings.ToLower(strings.TrimSpace(order.GuestEmail))
	prefix := strings.ToLower(strings.TrimSpace(order.GuestPassword))
	suffix := normalizeSuffix(order.ClientIP)
	if name == "" || fullDomain == "" || prefix == "" || suffix == "" {
		return nil
	}

	return s.orderRepo.Transaction(func(tx *gorm.DB) error {
		siteRepo := s.repo.WithTx(tx)
		orderRepo := s.orderRepo.WithTx(tx)

		locked, err := orderRepo.GetByID(orderID)
		if err != nil || locked == nil {
			return err
		}
		if strings.TrimSpace(locked.OrderScene) != constants.OrderSceneSiteOpening {
			return nil
		}

		existingByOrder, err := siteRepo.GetByOpenedOrderIDForUpdate(orderID)
		if err != nil {
			return err
		}
		if existingByOrder != nil {
			return nil
		}
		existingByOwner, err := siteRepo.GetByOwnerUserID(locked.UserID)
		if err != nil {
			return err
		}
		if existingByOwner != nil {
			return nil
		}

		site := &models.Site{
			OwnerUserID:     locked.UserID,
			Name:            name,
			SubdomainPrefix: prefix,
			Suffix:          suffix,
			FullDomain:      fullDomain,
			OpenedOrderID:   &locked.ID,
			Status:          models.SiteStatusActive,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(site).Error; err != nil {
			return err
		}
		if site.ID == 0 {
			// 幂等：并发下已创建
			_, _ = siteRepo.GetByOpenedOrderID(orderID)
		}
		return nil
	})
}

func (s *SiteService) validateOpenInput(userID uint, siteName, rawPrefix, rawSuffix string, cfg SiteOpenSetting) (string, error) {
	if strings.TrimSpace(siteName) == "" {
		return "", ErrSiteOpenInvalid
	}
	if userID == 0 {
		return "", ErrSiteOpenInvalid
	}
	existing, err := s.repo.GetByOwnerUserID(userID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", ErrSiteAlreadyOpened
	}
	prefix := strings.ToLower(strings.TrimSpace(rawPrefix))
	if prefix == "" {
		return "", ErrSitePrefixInvalid
	}
	pattern := strings.TrimSpace(cfg.PrefixRegex)
	if pattern == "" {
		pattern = defaultSitePrefixRegex
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		re = regexp.MustCompile(defaultSitePrefixRegex)
	}
	if !re.MatchString(prefix) {
		return "", ErrSitePrefixInvalid
	}
	for _, reserved := range cfg.ReservedPrefixes {
		if prefix == reserved {
			return "", ErrSitePrefixReserved
		}
	}
	if row, err := s.repo.GetByPrefix(prefix); err != nil {
		return "", err
	} else if row != nil {
		return "", ErrSitePrefixExists
	}
	suffix := normalizeSuffix(rawSuffix)
	if suffix == "" || !containsString(cfg.DomainSuffixes, suffix) {
		return "", ErrSiteSuffixInvalid
	}
	fullDomain := buildFullDomain(prefix, suffix)
	if row, err := s.repo.GetByFullDomain(fullDomain); err != nil {
		return "", err
	} else if row != nil {
		return "", ErrSiteDomainExists
	}
	return fullDomain, nil
}

func normalizeSuffix(raw string) string {
	suffix := strings.ToLower(strings.TrimSpace(raw))
	if suffix == "" {
		return ""
	}
	if !strings.HasPrefix(suffix, ".") {
		suffix = "." + suffix
	}
	return suffix
}

func buildFullDomain(prefix, suffix string) string {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	suffix = normalizeSuffix(suffix)
	return prefix + suffix
}

func containsString(values []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}

func (s *SiteService) resolveSiteCurrency() string {
	if s == nil || s.settingSvc == nil {
		return constants.SiteCurrencyDefault
	}
	currency, err := s.settingSvc.GetSiteCurrency(constants.SiteCurrencyDefault)
	if err != nil {
		return constants.SiteCurrencyDefault
	}
	return currency
}

// ResolveSiteByHost 基于请求 Host 做站点归因（不使用 Cookie）
func (s *SiteService) ResolveSiteByHost(rawHost string) (*SiteAttributionResult, error) {
	if s == nil || s.repo == nil {
		return &SiteAttributionResult{}, nil
	}
	host := normalizeRequestHost(rawHost)
	if host == "" {
		return &SiteAttributionResult{}, nil
	}
	site, err := s.repo.GetByFullDomain(host)
	if err != nil || site == nil {
		return &SiteAttributionResult{}, err
	}
	if strings.TrimSpace(site.Status) != models.SiteStatusActive {
		return &SiteAttributionResult{}, nil
	}
	id := site.ID
	return &SiteAttributionResult{SiteID: &id, FullDomain: site.FullDomain}, nil
}

// ResolveSiteSKUPrice 解析站点 SKU 定价，返回站点价（缺省返回基准价）
func (s *SiteService) ResolveSiteSKUPrice(siteID, productID, skuID uint, basePrice models.Money) (models.Money, error) {
	if siteID == 0 || skuID == 0 || s == nil || s.priceRepo == nil {
		return basePrice, nil
	}
	row, err := s.priceRepo.GetBySiteAndSKU(siteID, skuID)
	if err != nil || row == nil {
		return basePrice, err
	}
	price := row.SitePrice
	if price.Decimal.LessThan(basePrice.Decimal) {
		return basePrice, nil
	}
	if row.ProductID != 0 && productID != 0 && row.ProductID != productID {
		return basePrice, nil
	}
	return price, nil
}

func (s *SiteService) SetSiteSKUPrice(input SiteSetPriceInput) (*models.SiteProductPrice, error) {
	if s == nil || s.repo == nil || s.priceRepo == nil || s.productSKURepo == nil {
		return nil, ErrNotFound
	}
	if input.OwnerUserID == 0 || input.SKUID == 0 {
		return nil, ErrSitePriceInvalid
	}
	site, err := s.repo.GetByOwnerUserID(input.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, ErrSiteNotOpened
	}
	sku, err := s.productSKURepo.GetByID(input.SKUID)
	if err != nil || sku == nil || !sku.IsActive {
		return nil, ErrProductSKUInvalid
	}
	if input.ProductID > 0 && sku.ProductID != input.ProductID {
		return nil, ErrProductSKUInvalid
	}
	base := sku.PriceAmount.Decimal.Round(2)
	price := input.SitePrice.Decimal.Round(2)
	if price.LessThan(base) || price.LessThanOrEqual(decimal.Zero) {
		return nil, ErrSitePriceInvalid
	}
	row := &models.SiteProductPrice{
		SiteID:    site.ID,
		ProductID: sku.ProductID,
		SKUID:     sku.ID,
		SitePrice: models.NewMoneyFromDecimal(price),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.priceRepo.Upsert(row); err != nil {
		return nil, err
	}
	return s.priceRepo.GetBySiteAndSKU(site.ID, sku.ID)
}

func (s *SiteService) ListMySitePrices(ownerUserID uint) ([]SitePriceItem, error) {
	if s == nil || s.repo == nil || s.priceRepo == nil {
		return []SitePriceItem{}, nil
	}
	if ownerUserID == 0 {
		return []SitePriceItem{}, ErrSiteNotOpened
	}
	site, err := s.repo.GetByOwnerUserID(ownerUserID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return []SitePriceItem{}, nil
	}
	rows, err := s.priceRepo.ListBySite(site.ID)
	if err != nil {
		return nil, err
	}
	result := make([]SitePriceItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, SitePriceItem{SiteID: row.SiteID, ProductID: row.ProductID, SKUID: row.SKUID, SitePrice: row.SitePrice})
	}
	return result, nil
}

func normalizeRequestHost(rawHost string) string {
	host := strings.ToLower(strings.TrimSpace(rawHost))
	if host == "" {
		return ""
	}
	if strings.Contains(host, ",") {
		parts := strings.Split(host, ",")
		host = strings.TrimSpace(parts[0])
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimSuffix(host, "/")
	return strings.TrimSpace(host)
}

var (
	ErrSiteOpenDisabled   = errors.New("site open disabled")
	ErrSiteOpenInvalid    = errors.New("site open invalid")
	ErrSiteAlreadyOpened  = errors.New("site already opened")
	ErrSiteNotOpened      = errors.New("site not opened")
	ErrSitePrefixInvalid  = errors.New("site prefix invalid")
	ErrSitePrefixReserved = errors.New("site prefix reserved")
	ErrSitePrefixExists   = errors.New("site prefix exists")
	ErrSiteSuffixInvalid  = errors.New("site suffix invalid")
	ErrSiteDomainExists   = errors.New("site domain exists")
	ErrSitePriceInvalid   = errors.New("site price invalid")
)
