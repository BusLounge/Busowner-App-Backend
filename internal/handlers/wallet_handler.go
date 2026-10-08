package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// WalletHandler handles wallet and settlement HTTP endpoints
type WalletHandler struct {
	walletService *services.WalletService
}

// NewWalletHandler creates a new wallet handler
func NewWalletHandler(walletService *services.WalletService) *WalletHandler {
	return &WalletHandler{
		walletService: walletService,
	}
}

// GetWalletStatus returns current balance and payout tracker
// GET /wallet/status or GET /api/v1/wallet/status
func (h *WalletHandler) GetWalletStatus(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	status, err := h.walletService.GetWalletStatus(c.Request.Context(), userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wallet status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetPendingEarnings returns list of earnings waiting in 14-day holding period
// GET /settlements/pending or GET /api/v1/settlements/pending
func (h *WalletHandler) GetPendingEarnings(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	earnings, err := h.walletService.GetPendingEarnings(c.Request.Context(), userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending earnings: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, earnings)
}

// GetWalletTransactions returns paginated wallet transaction history
// GET /wallet/transactions or GET /api/v1/wallet/transactions
func (h *WalletHandler) GetWalletTransactions(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	history, err := h.walletService.GetWalletTransactions(c.Request.Context(), userCtx.UserID.String(), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// RequestSpecialPayout flags an early payout request
// POST /settlements/special-request or POST /api/v1/settlements/special-request
func (h *WalletHandler) RequestSpecialPayout(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	res, err := h.walletService.RequestSpecialPayout(c.Request.Context(), userCtx.UserID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request special payout: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// Withdraw processes a bank payout withdrawal to any bank account via PayHere
// POST /wallet/withdraw or POST /api/v1/wallet/withdraw
func (h *WalletHandler) Withdraw(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid withdrawal request: " + err.Error(),
			"details": "amount (>0), bank_name, account_number, and account_holder_name are required",
		})
		return
	}

	res, err := h.walletService.Withdraw(c.Request.Context(), userCtx.UserID.String(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, res)
}
