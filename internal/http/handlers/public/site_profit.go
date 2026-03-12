package public

import (
	"errors"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type SiteWithdrawApplyRequest struct {
	Amount  string `json:"amount" binding:"required"`
	Channel string `json:"channel" binding:"required"`
	Account string `json:"account" binding:"required"`
}

func (h *Handler) GetMySiteSummary(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	row, err := h.SiteProfitService.GetMySummary(uid)
	if err != nil {
		if errors.Is(err, service.ErrSiteNotOpened) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, row)
}

func (h *Handler) ListMySiteOrderSummaries(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)
	rows, total, err := h.SiteProfitService.ListMyOrders(uid, page, pageSize, strings.TrimSpace(c.Query("status")))
	if err != nil {
		if errors.Is(err, service.ErrSiteNotOpened) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

func (h *Handler) ListMySiteProfitLedgers(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)
	rows, total, err := h.SiteProfitService.ListMyLedgers(uid, page, pageSize, strings.TrimSpace(c.Query("status")))
	if err != nil {
		if errors.Is(err, service.ErrSiteNotOpened) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

func (h *Handler) ApplySiteWithdraw(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	var req SiteWithdrawApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	row, err := h.SiteProfitService.ApplyWithdraw(uid, service.SiteWithdrawApplyInput{Amount: amount, Channel: req.Channel, Account: req.Account})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSiteNotOpened), errors.Is(err, service.ErrSiteWithdrawAmountInvalid), errors.Is(err, service.ErrSiteWithdrawChannelInvalid), errors.Is(err, service.ErrSiteWithdrawInsufficient):
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, row)
}

func (h *Handler) ListMySiteWithdraws(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)
	rows, total, err := h.SiteProfitService.ListMyWithdraws(uid, page, pageSize, strings.TrimSpace(c.Query("status")))
	if err != nil {
		if errors.Is(err, service.ErrSiteNotOpened) {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}
