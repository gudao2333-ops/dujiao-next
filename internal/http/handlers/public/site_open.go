package public

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type SiteOpenRequest struct {
	SiteName        string `json:"site_name" binding:"required"`
	SubdomainPrefix string `json:"subdomain_prefix" binding:"required"`
	DomainSuffix    string `json:"domain_suffix" binding:"required"`
}

// PreviewOpenSite 预览开通子站
func (h *Handler) PreviewOpenSite(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	var req SiteOpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	result, err := h.SiteService.PreviewOpen(service.SiteOpenPreviewInput{
		UserID:          uid,
		SiteName:        req.SiteName,
		SubdomainPrefix: req.SubdomainPrefix,
		SelectedSuffix:  req.DomainSuffix,
	})
	if err != nil {
		respondSiteOpenError(c, err)
		return
	}
	response.Success(c, result)
}

// OpenSite 创建开通子站订单
func (h *Handler) OpenSite(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	var req SiteOpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	result, err := h.SiteService.CreateOpenOrder(service.SiteOpenPreviewInput{
		UserID:          uid,
		SiteName:        req.SiteName,
		SubdomainPrefix: req.SubdomainPrefix,
		SelectedSuffix:  req.DomainSuffix,
	})
	if err != nil {
		respondSiteOpenError(c, err)
		return
	}
	response.Success(c, result)
}

func respondSiteOpenError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSiteOpenDisabled):
		shared.RespondError(c, response.CodeBadRequest, "error.forbidden", nil)
	case errors.Is(err, service.ErrSiteAlreadyOpened),
		errors.Is(err, service.ErrSiteNotOpened),
		errors.Is(err, service.ErrSitePrefixInvalid),
		errors.Is(err, service.ErrSitePrefixReserved),
		errors.Is(err, service.ErrSitePrefixExists),
		errors.Is(err, service.ErrSiteSuffixInvalid),
		errors.Is(err, service.ErrSiteDomainExists),
		errors.Is(err, service.ErrSitePriceInvalid),
		errors.Is(err, service.ErrSiteOpenInvalid):
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	default:
		shared.RespondError(c, response.CodeInternal, "error.save_failed", err)
	}
}

type SiteSetPriceRequest struct {
	ProductID uint   `json:"product_id" binding:"required"`
	SKUID     uint   `json:"sku_id" binding:"required"`
	SitePrice string `json:"site_price" binding:"required"`
}

// ListMySitePrices 查询我的子站定价
func (h *Handler) ListMySitePrices(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	rows, err := h.SiteService.ListMySitePrices(uid)
	if err != nil {
		respondSiteOpenError(c, err)
		return
	}
	response.Success(c, rows)
}

// SetMySitePrice 设置我的子站 SKU 售价
func (h *Handler) SetMySitePrice(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	var req SiteSetPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	price, err := decimal.NewFromString(strings.TrimSpace(req.SitePrice))
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	row, err := h.SiteService.SetSiteSKUPrice(service.SiteSetPriceInput{
		OwnerUserID: uid,
		ProductID:   req.ProductID,
		SKUID:       req.SKUID,
		SitePrice:   models.NewMoneyFromDecimal(price),
	})
	if err != nil {
		respondSiteOpenError(c, err)
		return
	}
	response.Success(c, row)
}
