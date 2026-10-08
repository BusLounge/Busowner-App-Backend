package models

import (
	"time"
)

// Wallet represents the user wallet from wallets_passenger
type Wallet struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Balance   float64   `json:"balance" db:"balance"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PayoutTrackerInfo represents the 14-day payout progression
type PayoutTrackerInfo struct {
	DaysAccumulated        int     `json:"days_accumulated"`
	FrequencyDays          int     `json:"frequency_days"`
	NextExpectedPayoutDate *string `json:"next_expected_payout_date"`
	TotalPendingAmount     float64 `json:"total_pending_amount"`
}

// WalletStatusResponse matches Admin API spec: GET /wallet/status
type WalletStatusResponse struct {
	WalletBalance float64           `json:"wallet_balance"`
	Currency      string            `json:"currency"`
	PayoutTracker PayoutTrackerInfo `json:"payout_tracker"`
}

// PendingEarningItem matches Admin API spec: GET /settlements/pending item
type PendingEarningItem struct {
	ID            string  `json:"id" db:"id"`
	EarningDate   string  `json:"earning_date" db:"earning_date"`
	ReferenceType string  `json:"reference_type" db:"reference_type"`
	NetAmount     float64 `json:"net_amount" db:"net_amount"`
	Status        string  `json:"status" db:"status"`
}

// PendingEarningsResponse matches Admin API spec: GET /settlements/pending
type PendingEarningsResponse struct {
	TotalPending float64              `json:"total_pending"`
	Earnings     []PendingEarningItem `json:"earnings"`
}

// WalletTransactionItem matches Admin API spec: GET /wallet/transactions item
type WalletTransactionItem struct {
	TransactionID string  `json:"transaction_id" db:"id"`
	Date          string  `json:"date" db:"created_at"`
	Type          string  `json:"type" db:"transaction_type"`
	Description   string  `json:"description" db:"description"`
	Amount        float64 `json:"amount" db:"amount"`
	BalanceAfter  float64 `json:"balance_after" db:"balance_after"`
}

// PaginationInfo matches pagination in Admin API spec
type PaginationInfo struct {
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
	TotalCount  int `json:"total_count,omitempty"`
}

// WalletTransactionsResponse matches Admin API spec: GET /wallet/transactions
type WalletTransactionsResponse struct {
	Transactions []WalletTransactionItem `json:"transactions"`
	Pagination   PaginationInfo          `json:"pagination"`
}

// SpecialPayoutResponse matches Admin API spec: POST /settlements/special-request
type SpecialPayoutResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// WithdrawRequest represents a bank payout withdrawal request
type WithdrawRequest struct {
	Amount            float64 `json:"amount" binding:"required,gt=0"`
	BankName          string  `json:"bank_name" binding:"required"`
	BankCode          string  `json:"bank_code"`
	BranchCode        string  `json:"branch_code"`
	AccountNumber     string  `json:"account_number" binding:"required"`
	AccountHolderName string  `json:"account_holder_name" binding:"required"`
}

// WithdrawResponse represents the outcome of a bank payout withdrawal
type WithdrawResponse struct {
	Success          bool    `json:"success"`
	Message          string  `json:"message"`
	TransactionID    string  `json:"transaction_id"`
	GatewayReference string  `json:"gateway_reference"`
	Amount           float64 `json:"amount"`
	NewBalance       float64 `json:"new_balance"`
	Status           string  `json:"status"`
}
