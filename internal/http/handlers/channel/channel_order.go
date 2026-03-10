package channel

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// --- 请求/响应 ---

type orderListItem struct {
	ID           uint   `json:"id"`
	OrderNo      string `json:"order_no"`
	Status       string `json:"status"`
	Currency     string `json:"currency"`
	TotalAmount  string `json:"total_amount"`
	ProductTitle string `json:"product_title"`
	CreatedAt    string `json:"created_at"`
}

type createOrderRequest struct {
	TelegramUserID string `json:"telegram_user_id" binding:"required"`
	TelegramUser   string `json:"telegram_username"`
	ProductID      uint   `json:"product_id" binding:"required"`
	SKUID          uint   `json:"sku_id" binding:"required"`
	Quantity       int    `json:"quantity" binding:"required,min=1,max=10"`
	Locale         string `json:"locale"`
}

type createPaymentRequest struct {
	OrderID    uint `json:"order_id" binding:"required"`
	ChannelID  uint `json:"channel_id"`
	UseBalance bool `json:"use_balance"`
}

// --- 处理器 ---

// CreateOrder POST /api/v1/channel/orders
func (h *Handler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_request",
			"error_message": err.Error(),
		})
		return
	}

	// 解析或创建 Telegram 用户
	userID, err := h.resolveOrCreateTelegramUser(req.TelegramUserID, req.TelegramUser)
	if err != nil {
		logger.Errorw("channel_order_resolve_user", "telegram_user_id", req.TelegramUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "user_resolve_failed",
			"error_message": "failed to resolve telegram user",
		})
		return
	}

	// 查询商品获取 fulfillment_type
	product, err := h.ProductRepo.GetByID(fmt.Sprintf("%d", req.ProductID))
	if err != nil || product == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "product_not_found",
			"error_message": "product not found",
		})
		return
	}

	order, err := h.OrderService.CreateOrder(service.CreateOrderInput{
		UserID: userID,
		Items: []service.CreateOrderItem{{
			ProductID:       req.ProductID,
			SKUID:           req.SKUID,
			Quantity:        req.Quantity,
			FulfillmentType: product.FulfillmentType,
		}},
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		logger.Errorw("channel_order_create", "user_id", userID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "order_create_failed",
			"error_message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"order": gin.H{
			"id":              order.ID,
			"order_no":        order.OrderNo,
			"status":          order.Status,
			"currency":        order.Currency,
			"total_amount":    order.TotalAmount.StringFixed(2),
			"original_amount": order.OriginalAmount.StringFixed(2),
			"expires_at":      order.ExpiresAt,
		},
	})
}

// GetPaymentChannels GET /api/v1/channel/payment-channels
func (h *Handler) GetPaymentChannels(c *gin.Context) {
	channels, _, err := h.PaymentService.ListChannels(repository.PaymentChannelListFilter{
		ActiveOnly: true,
		Page:       1,
		PageSize:   50,
	})
	if err != nil {
		logger.Errorw("channel_order_list_payment_channels", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "internal_error",
			"error_message": "failed to list payment channels",
		})
		return
	}

	type channelItem struct {
		ID              uint   `json:"id"`
		Name            string `json:"name"`
		ProviderType    string `json:"provider_type"`
		ChannelType     string `json:"channel_type"`
		InteractionMode string `json:"interaction_mode"`
		FeeRate         string `json:"fee_rate"`
	}

	var items []channelItem
	for _, ch := range channels {
		// 排除钱包/余额支付（Bot 端独立处理）
		if ch.ProviderType == "balance" || ch.ProviderType == "wallet" {
			continue
		}
		items = append(items, channelItem{
			ID:              ch.ID,
			Name:            ch.Name,
			ProviderType:    ch.ProviderType,
			ChannelType:     ch.ChannelType,
			InteractionMode: ch.InteractionMode,
			FeeRate:         ch.FeeRate.StringFixed(2),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"channels": items,
	})
}

// CreatePayment POST /api/v1/channel/payments
func (h *Handler) CreatePayment(c *gin.Context) {
	var req createPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_request",
			"error_message": err.Error(),
		})
		return
	}

	result, err := h.PaymentService.CreatePayment(service.CreatePaymentInput{
		OrderID:    req.OrderID,
		ChannelID:  req.ChannelID,
		UseBalance: req.UseBalance,
		ClientIP:   c.ClientIP(),
		Context:    c.Request.Context(),
	})
	if err != nil {
		logger.Errorw("channel_order_create_payment", "order_id", req.OrderID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "payment_create_failed",
			"error_message": err.Error(),
		})
		return
	}

	resp := gin.H{
		"ok":         true,
		"order_paid": result.OrderPaid,
	}
	if result.Payment != nil {
		resp["payment"] = gin.H{
			"id":         result.Payment.ID,
			"amount":     result.Payment.Amount.StringFixed(2),
			"fee_amount": result.Payment.FeeAmount.StringFixed(2),
			"currency":   result.Payment.Currency,
			"status":     result.Payment.Status,
			"pay_url":    result.Payment.PayURL,
			"qr_code":    result.Payment.QRCode,
		}
	}
	if result.Channel != nil {
		resp["channel_name"] = result.Channel.Name
	}

	c.JSON(http.StatusOK, resp)
}

// GetOrderStatus GET /api/v1/channel/orders/:id
func (h *Handler) GetOrderStatus(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_order_id",
			"error_message": "invalid order id",
		})
		return
	}

	order, err := h.OrderRepo.GetByID(uint(orderID))
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":            false,
			"error_code":    "order_not_found",
			"error_message": "order not found",
		})
		return
	}

	order.MaskUpstreamFulfillmentType()

	resp := gin.H{
		"id":              order.ID,
		"order_no":        order.OrderNo,
		"status":          order.Status,
		"currency":        order.Currency,
		"total_amount":    order.TotalAmount.StringFixed(2),
		"original_amount": order.OriginalAmount.StringFixed(2),
		"paid_at":         order.PaidAt,
		"expires_at":      order.ExpiresAt,
	}

	// 发货信息（直接子单）
	if order.Fulfillment != nil && order.Fulfillment.Status == "delivered" {
		resp["fulfillment"] = gin.H{
			"status":       order.Fulfillment.Status,
			"type":         order.Fulfillment.Type,
			"payload":      order.Fulfillment.Payload,
			"delivered_at": order.Fulfillment.DeliveredAt,
		}
	}

	// 子订单发货（多商品场景）
	if len(order.Children) > 0 {
		var children []gin.H
		for _, child := range order.Children {
			childH := gin.H{
				"id":     child.ID,
				"status": child.Status,
			}
			if child.Fulfillment != nil && child.Fulfillment.Status == "delivered" {
				childH["fulfillment"] = gin.H{
					"status":       child.Fulfillment.Status,
					"type":         child.Fulfillment.Type,
					"payload":      child.Fulfillment.Payload,
					"delivered_at": child.Fulfillment.DeliveredAt,
				}
			}
			children = append(children, childH)
		}
		resp["children"] = children
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":    true,
		"order": resp,
	})
}

// CancelOrder POST /api/v1/channel/orders/:id/cancel
func (h *Handler) CancelOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "invalid_order_id",
			"error_message": "invalid order id",
		})
		return
	}

	// 查询订单获取 UserID
	order, err := h.OrderRepo.GetByID(uint(orderID))
	if err != nil || order == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":            false,
			"error_code":    "order_not_found",
			"error_message": "order not found",
		})
		return
	}

	_, err = h.OrderService.CancelOrder(uint(orderID), order.UserID)
	if err != nil {
		logger.Errorw("channel_order_cancel", "order_id", orderID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"error_code":    "cancel_failed",
			"error_message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListOrders GET /api/v1/channel/orders?telegram_user_id=xxx&page=1&page_size=5&status=
func (h *Handler) ListOrders(c *gin.Context) {
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
		logger.Errorw("channel_order_list_resolve_user", "telegram_user_id", telegramUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "user_resolve_failed",
			"error_message": "failed to resolve telegram user",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "5"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 20 {
		pageSize = 5
	}
	status := c.Query("status")

	orders, total, err := h.OrderService.ListOrdersByUser(repository.OrderListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
		Status:   status,
	})
	if err != nil {
		logger.Errorw("channel_order_list", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":            false,
			"error_code":    "order_list_failed",
			"error_message": "failed to list orders",
		})
		return
	}

	items := make([]orderListItem, 0, len(orders))
	for _, o := range orders {
		productTitle := ""
		if len(o.Items) > 0 {
			titleJSON := o.Items[0].TitleJSON
			if t, ok := titleJSON["zh-CN"].(string); ok && t != "" {
				productTitle = t
			} else if t, ok := titleJSON["en-US"].(string); ok && t != "" {
				productTitle = t
			} else {
				for _, v := range titleJSON {
					if s, ok := v.(string); ok && s != "" {
						productTitle = s
						break
					}
				}
			}
		}
		items = append(items, orderListItem{
			ID:           o.ID,
			OrderNo:      o.OrderNo,
			Status:       o.Status,
			Currency:     o.Currency,
			TotalAmount:  o.TotalAmount.StringFixed(2),
			ProductTitle: productTitle,
			CreatedAt:    o.CreatedAt.Format(time.RFC3339),
		})
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	c.JSON(http.StatusOK, gin.H{
		"ok":     true,
		"orders": items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// resolveOrCreateTelegramUser 解析或创建 Telegram 用户，返回本地 UserID
func (h *Handler) resolveOrCreateTelegramUser(telegramUserID, username string) (uint, error) {
	// 1. 查找已有 OAuth 绑定
	identity, err := h.UserOAuthIdentityRepo.GetByProviderUserID("telegram", telegramUserID)
	if err != nil {
		return 0, fmt.Errorf("lookup oauth identity: %w", err)
	}
	if identity != nil {
		return identity.UserID, nil
	}

	// 2. 无绑定 — 查找/创建 Telegram 用户（复用 UserAuthService 的逻辑模式）
	email := fmt.Sprintf("tg_%s@telegram.placeholder", telegramUserID)
	user, err := h.UserRepo.GetByEmail(email)
	if err != nil {
		return 0, fmt.Errorf("lookup user by email: %w", err)
	}
	if user != nil {
		// 用户已存在但无 OAuth 绑定 → 创建绑定
		now := time.Now()
		oauthIdentity := &models.UserOAuthIdentity{
			UserID:         user.ID,
			Provider:       "telegram",
			ProviderUserID: telegramUserID,
			Username:       username,
			AuthAt:         &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := h.UserOAuthIdentityRepo.Create(oauthIdentity); err != nil {
			logger.Warnw("channel_order_create_oauth_identity", "user_id", user.ID, "error", err)
		}
		return user.ID, nil
	}

	// 3. 完全新用户 — 通过 UserAuthService 创建
	// 复用 UserAuthService 的 LoginWithTelegram 底层逻辑:
	// 构造 TelegramIdentityVerified 并调用 findOrCreateTelegramUser
	// 但该方法是未导出的，因此我们在此手动复制核心流程

	displayName := username
	if displayName == "" {
		displayName = fmt.Sprintf("telegram_%s", telegramUserID)
	}

	// 创建用户
	now := time.Now()
	randomPassword := fmt.Sprintf("tg_%s_%d", telegramUserID, now.UnixNano())
	newUser := &models.User{
		Email:                 email,
		PasswordHash:          randomPassword, // 占位，用户不知道
		PasswordSetupRequired: true,
		DisplayName:           displayName,
		Status:                "active",
		LastLoginAt:           &now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := h.UserRepo.Create(newUser); err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}

	// 创建 OAuth 绑定
	oauthIdentity := &models.UserOAuthIdentity{
		UserID:         newUser.ID,
		Provider:       "telegram",
		ProviderUserID: telegramUserID,
		Username:       username,
		AuthAt:         &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := h.UserOAuthIdentityRepo.Create(oauthIdentity); err != nil {
		logger.Warnw("channel_order_create_oauth_identity_new", "user_id", newUser.ID, "error", err)
	}

	return newUser.ID, nil
}
