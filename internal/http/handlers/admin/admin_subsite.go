package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
)

type AdminSubsiteSuffixRequest struct {
	Suffix    string `json:"suffix" binding:"required"`
	IsEnabled bool   `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
}

type AdminSubsiteStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *Handler) ListSubsites(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = shared.NormalizePagination(page, pageSize)
	rows, total, err := h.SiteService.ListAdminSites(service.AdminSiteListFilter{Page: page, PageSize: pageSize, Status: strings.TrimSpace(c.Query("status"))})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

func (h *Handler) UpdateSubsiteStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req AdminSubsiteStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	row, err := h.SiteService.UpdateAdminSiteStatus(uint(id), req.Status)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.not_found", nil)
		case errors.Is(err, service.ErrSiteStatusInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, row)
}

func (h *Handler) ListSubsiteSuffixes(c *gin.Context) {
	rows, err := h.SiteService.ListAdminSuffixes()
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, rows)
}

func (h *Handler) CreateSubsiteSuffix(c *gin.Context) {
	var req AdminSubsiteSuffixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	row, err := h.SiteService.CreateAdminSuffix(service.AdminSiteSuffixInput{Suffix: req.Suffix, IsEnabled: req.IsEnabled, SortOrder: req.SortOrder})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSiteSuffixInvalid), errors.Is(err, service.ErrSiteDomainExists):
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, row)
}

func (h *Handler) UpdateSubsiteSuffix(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req AdminSubsiteSuffixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	row, err := h.SiteService.UpdateAdminSuffix(uint(id), service.AdminSiteSuffixInput{Suffix: req.Suffix, IsEnabled: req.IsEnabled, SortOrder: req.SortOrder})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.not_found", nil)
		case errors.Is(err, service.ErrSiteSuffixInvalid), errors.Is(err, service.ErrSiteDomainExists):
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, row)
}

func (h *Handler) DeleteSubsiteSuffix(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.SiteService.DeleteAdminSuffix(uint(id)); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.save_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
