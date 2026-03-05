package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// GetProcurementOrders 采购单列表
func (h *Handler) GetProcurementOrders(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)

	filter := repository.ProcurementOrderListFilter{
		Pagination: repository.Pagination{Page: page, PageSize: pageSize},
	}
	if connID := strings.TrimSpace(c.Query("connection_id")); connID != "" {
		if id, err := strconv.ParseUint(connID, 10, 64); err == nil {
			filter.ConnectionID = uint(id)
		}
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filter.Status = status
	}
	if orderNo := strings.TrimSpace(c.Query("order_no")); orderNo != "" {
		filter.LocalOrderNo = orderNo
	}

	orders, total, err := h.ProcurementOrderService.List(filter)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, orders, pagination)
}

// GetProcurementOrder 采购单详情
func (h *Handler) GetProcurementOrder(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, err := h.ProcurementOrderService.GetByID(uint(id))
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
		return
	}
	if order == nil {
		shared.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
		return
	}
	response.Success(c, order)
}

// RetryProcurementOrder 手动重试采购单
func (h *Handler) RetryProcurementOrder(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.ProcurementOrderService.RetryManual(uint(id)); err != nil {
		if errors.Is(err, service.ErrProcurementNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrProcurementStatusInvalid) {
			shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.procurement_retry_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// CancelProcurementOrder 手动取消采购单
func (h *Handler) CancelProcurementOrder(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.ProcurementOrderService.CancelManual(uint(id)); err != nil {
		if errors.Is(err, service.ErrProcurementNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrProcurementStatusInvalid) {
			shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.procurement_cancel_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
