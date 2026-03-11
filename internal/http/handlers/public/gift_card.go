package public

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// RedeemGiftCardRequest 兑换礼品卡请求
type RedeemGiftCardRequest struct {
	Code           string                       `json:"code" binding:"required"`
	CaptchaPayload shared.CaptchaPayloadRequest `json:"captcha_payload"`
}

// RedeemGiftCard 用户兑换礼品卡
func (h *Handler) RedeemGiftCard(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	var req RedeemGiftCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	if h.CaptchaService != nil {
		if captchaErr := h.CaptchaService.Verify(constants.CaptchaSceneGiftCardRedeem, req.CaptchaPayload.ToServicePayload(), c.ClientIP()); captchaErr != nil {
			switch {
			case errors.Is(captchaErr, service.ErrCaptchaRequired):
				shared.RespondError(c, response.CodeBadRequest, "error.captcha_required", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaInvalid):
				shared.RespondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
				return
			case errors.Is(captchaErr, service.ErrCaptchaConfigInvalid):
				shared.RespondError(c, response.CodeInternal, "error.captcha_config_invalid", captchaErr)
				return
			default:
				shared.RespondError(c, response.CodeInternal, "error.captcha_verify_failed", captchaErr)
				return
			}
		}
	}

	result, err := h.GiftCardService.RedeemGiftCard(service.GiftCardRedeemInput{
		UserID: uid,
		Code:   strings.TrimSpace(req.Code),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGiftCardInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
		case errors.Is(err, service.ErrGiftCardNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.gift_card_not_found", nil)
		case errors.Is(err, service.ErrGiftCardExpired):
			shared.RespondError(c, response.CodeBadRequest, "error.gift_card_expired", nil)
		case errors.Is(err, service.ErrGiftCardDisabled):
			shared.RespondError(c, response.CodeBadRequest, "error.gift_card_disabled", nil)
		case errors.Is(err, service.ErrGiftCardRedeemed):
			shared.RespondError(c, response.CodeBadRequest, "error.gift_card_redeemed", nil)
		case errors.Is(err, service.ErrGiftCardRedeemTargetInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
		case errors.Is(err, service.ErrManualStockInsufficient):
			shared.RespondError(c, response.CodeBadRequest, "error.manual_stock_insufficient", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.gift_card_redeem_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"gift_card":    result.Card,
		"wallet":       result.WalletAccount,
		"transaction":  result.WalletTransaction,
		"wallet_delta": result.Card.Amount,
		"redeem_mode":  result.RedeemMode,
		"order_id":     result.OrderID,
		"order_no":     result.OrderNo,
		"order_status": result.OrderStatus,
		"redirect_url": result.RedirectURL,
	})
}
