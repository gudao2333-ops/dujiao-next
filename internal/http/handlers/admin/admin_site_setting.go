package admin

import (
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetSiteOpenSettings(c *gin.Context) {
	setting, err := h.SettingService.GetSiteOpenSetting()
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, setting)
}

func (h *Handler) UpdateSiteOpenSettings(c *gin.Context) {
	var req service.SiteOpenSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	setting, err := h.SettingService.UpdateSiteOpenSetting(req)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}
	response.Success(c, setting)
}
