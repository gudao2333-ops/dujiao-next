package channel

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

// GetWallet GET /api/v1/channel/wallet?telegram_user_id=xxx
func (h *Handler) GetWallet(c *gin.Context) {
	telegramUserID := c.Query("telegram_user_id")
	if telegramUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_request",
			"error_message": "telegram_user_id is required",
		})
		return
	}

	userID, err := h.resolveOrCreateTelegramUser(telegramUserID, "")
	if err != nil {
		logger.Errorw("channel_wallet_resolve_user", "telegram_user_id", telegramUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "user_resolve_failed",
			"error_message": "failed to resolve telegram user",
		})
		return
	}

	account, err := h.WalletService.GetAccount(userID)
	if err != nil {
		logger.Errorw("channel_wallet_get_account", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "wallet_error",
			"error_message": "failed to get wallet account",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"balance":  account.Balance.StringFixed(2),
		"currency": "CNY",
	})
}

// GetWalletTransactions GET /api/v1/channel/wallet/transactions?telegram_user_id=xxx&page=1&page_size=5
func (h *Handler) GetWalletTransactions(c *gin.Context) {
	telegramUserID := c.Query("telegram_user_id")
	if telegramUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_request",
			"error_message": "telegram_user_id is required",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "5"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 20 {
		pageSize = 5
	}

	userID, err := h.resolveOrCreateTelegramUser(telegramUserID, "")
	if err != nil {
		logger.Errorw("channel_wallet_txns_resolve_user", "telegram_user_id", telegramUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "user_resolve_failed",
			"error_message": "failed to resolve telegram user",
		})
		return
	}

	txns, total, err := h.WalletService.ListTransactions(repository.WalletTransactionListFilter{
		UserID:   userID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		logger.Errorw("channel_wallet_list_txns", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "wallet_error",
			"error_message": "failed to list transactions",
		})
		return
	}

	type txnItem struct {
		Type         string `json:"type"`
		Direction    string `json:"direction"`
		Amount       string `json:"amount"`
		BalanceAfter string `json:"balance_after"`
		Remark       string `json:"remark"`
		CreatedAt    string `json:"created_at"`
	}

	items := make([]txnItem, 0, len(txns))
	for _, t := range txns {
		items = append(items, txnItem{
			Type:         t.Type,
			Direction:    t.Direction,
			Amount:       t.Amount.StringFixed(2),
			BalanceAfter: t.BalanceAfter.StringFixed(2),
			Remark:       t.Remark,
			CreatedAt:    t.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"transactions": items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// CreateWalletRecharge POST /api/v1/channel/wallet/recharge
func (h *Handler) CreateWalletRecharge(c *gin.Context) {
	var req struct {
		TelegramUserID string `json:"telegram_user_id" binding:"required"`
		Amount         string `json:"amount" binding:"required"`
		ChannelID      uint   `json:"channel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_request",
			"error_message": err.Error(),
		})
		return
	}

	userID, err := h.resolveOrCreateTelegramUser(req.TelegramUserID, "")
	if err != nil {
		logger.Errorw("channel_wallet_recharge_resolve_user", "telegram_user_id", req.TelegramUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "user_resolve_failed",
			"error_message": "failed to resolve telegram user",
		})
		return
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_amount",
			"error_message": "invalid amount format",
		})
		return
	}

	currency, _ := h.SettingService.GetSiteCurrency(constants.SiteCurrencyDefault)

	result, err := h.PaymentService.CreateWalletRechargePayment(service.CreateWalletRechargePaymentInput{
		UserID:    userID,
		ChannelID: req.ChannelID,
		Amount:    models.NewMoneyFromDecimal(amount),
		Currency:  currency,
		ClientIP:  c.ClientIP(),
		Context:   c.Request.Context(),
	})
	if err != nil {
		logger.Errorw("channel_wallet_recharge_create", "user_id", userID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "recharge_failed",
			"error_message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"recharge_no": result.Recharge.RechargeNo,
		"payment": gin.H{
			"id":         result.Payment.ID,
			"amount":     result.Payment.Amount.StringFixed(2),
			"fee_amount": result.Payment.FeeAmount.StringFixed(2),
			"currency":   result.Payment.Currency,
			"status":     result.Payment.Status,
			"pay_url":    result.Payment.PayURL,
			"qr_code":    result.Payment.QRCode,
		},
	})
}
