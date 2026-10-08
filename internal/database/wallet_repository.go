package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// WalletRepository manages wallet and transaction operations
type WalletRepository struct {
	db *sqlx.DB
}

// NewWalletRepository creates a new wallet repository
func NewWalletRepository(db *sqlx.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

// GetOrCreateWallet retrieves or initializes a wallet for a given user
func (r *WalletRepository) GetOrCreateWallet(ctx context.Context, userID string) (*models.Wallet, error) {
	var wallet models.Wallet
	query := `SELECT id, user_id, balance, status, created_at, updated_at FROM wallets_passenger WHERE user_id = $1`
	err := r.db.GetContext(ctx, &wallet, query, userID)
	if err == nil {
		return &wallet, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to fetch wallet: %w", err)
	}

	// Create wallet if it does not exist
	insertQuery := `
		INSERT INTO wallets_passenger (id, user_id, balance, status, created_at, updated_at)
		VALUES ($1, $2, 0.00, 'ACTIVE', NOW(), NOW())
		RETURNING id, user_id, balance, status, created_at, updated_at
	`
	newID := uuid.New().String()
	err = r.db.GetContext(ctx, &wallet, insertQuery, newID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	return &wallet, nil
}

// GetWalletStatus retrieves current balance and 14-day payout progression
func (r *WalletRepository) GetWalletStatus(ctx context.Context, userID string) (*models.WalletStatusResponse, error) {
	wallet, err := r.GetOrCreateWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	var daysAccumulated int = 0
	var frequencyDays int = 14
	var nextExpectedPayoutDate *string

	// Query payout_tracker
	trackerQuery := `
		SELECT days_accumulated, payout_frequency_days, next_payout_date::text
		FROM payout_tracker
		WHERE payee_user_id = $1
		LIMIT 1
	`
	var ptNextDate sql.NullString
	err = r.db.QueryRowContext(ctx, trackerQuery, userID).Scan(&daysAccumulated, &frequencyDays, &ptNextDate)
	if err == nil && ptNextDate.Valid {
		formatted := ptNextDate.String + "T00:00:00Z"
		nextExpectedPayoutDate = &formatted
	}

	// If next payout date is not set, calculate default (14 days from now)
	if nextExpectedPayoutDate == nil {
		defaultDate := time.Now().AddDate(0, 0, frequencyDays-daysAccumulated).Format("2006-01-02T15:04:05Z")
		nextExpectedPayoutDate = &defaultDate
	}

	// Query total pending earnings
	var totalPending float64 = 0.0
	pendingQuery := `
		SELECT COALESCE(SUM(net_amount), 0)
		FROM settlements
		WHERE payee_user_id = $1 AND status = 'pending' AND is_paid = false
	`
	_ = r.db.QueryRowContext(ctx, pendingQuery, userID).Scan(&totalPending)

	return &models.WalletStatusResponse{
		WalletBalance: wallet.Balance,
		Currency:      "LKR",
		PayoutTracker: models.PayoutTrackerInfo{
			DaysAccumulated:        daysAccumulated,
			FrequencyDays:          frequencyDays,
			NextExpectedPayoutDate: nextExpectedPayoutDate,
			TotalPendingAmount:     totalPending,
		},
	}, nil
}

// GetPendingEarnings retrieves individual earnings waiting in holding period
func (r *WalletRepository) GetPendingEarnings(ctx context.Context, userID string) (*models.PendingEarningsResponse, error) {
	query := `
		SELECT 
			id::text, 
			earning_date::text, 
			CASE WHEN scheduled_trip_id IS NOT NULL THEN 'bus_trip' ELSE 'lounge_booking' END AS reference_type,
			net_amount, 
			status
		FROM settlements
		WHERE payee_user_id = $1 AND status = 'pending' AND is_paid = false
		ORDER BY earning_date DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending settlements: %w", err)
	}
	defer rows.Close()

	var earnings []models.PendingEarningItem
	var totalPending float64 = 0.0

	for rows.Next() {
		var item models.PendingEarningItem
		if err := rows.Scan(&item.ID, &item.EarningDate, &item.ReferenceType, &item.NetAmount, &item.Status); err != nil {
			return nil, fmt.Errorf("failed to scan pending settlement: %w", err)
		}
		totalPending += item.NetAmount
		earnings = append(earnings, item)
	}

	if earnings == nil {
		earnings = []models.PendingEarningItem{}
	}

	return &models.PendingEarningsResponse{
		TotalPending: totalPending,
		Earnings:     earnings,
	}, nil
}

// GetWalletTransactions retrieves paginated transactions for a wallet
func (r *WalletRepository) GetWalletTransactions(ctx context.Context, walletID string, page, limit int) (*models.WalletTransactionsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var totalCount int
	countQuery := `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1`
	err := r.db.QueryRowContext(ctx, countQuery, walletID).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))
	if totalPages == 0 {
		totalPages = 1
	}

	query := `
		SELECT 
			id::text, 
			created_at::text, 
			transaction_type, 
			COALESCE(description, ''), 
			amount, 
			balance_after
		FROM wallet_transactions
		WHERE wallet_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, walletID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}
	defer rows.Close()

	var items []models.WalletTransactionItem
	for rows.Next() {
		var item models.WalletTransactionItem
		if err := rows.Scan(&item.TransactionID, &item.Date, &item.Type, &item.Description, &item.Amount, &item.BalanceAfter); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		items = append(items, item)
	}

	if items == nil {
		items = []models.WalletTransactionItem{}
	}

	return &models.WalletTransactionsResponse{
		Transactions: items,
		Pagination: models.PaginationInfo{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalCount:  totalCount,
		},
	}, nil
}

// RequestSpecialPayout flags early payout in payout_tracker
func (r *WalletRepository) RequestSpecialPayout(ctx context.Context, userID string) error {
	upsertQuery := `
		INSERT INTO payout_tracker (id, payee_user_id, payee_type, payout_frequency_days, days_accumulated, has_special_request, special_request_date, is_active, created_at, updated_at)
		VALUES ($1, $2, 'bus_owner', 14, 0, true, NOW(), true, NOW(), NOW())
		ON CONFLICT (payee_user_id, payee_type) 
		DO UPDATE SET has_special_request = true, special_request_date = NOW(), updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, upsertQuery, uuid.New().String(), userID)
	if err != nil {
		return fmt.Errorf("failed to record special payout request: %w", err)
	}
	return nil
}

// WithdrawFunds securely deducts wallet balance and records bank payout transaction
func (r *WalletRepository) WithdrawFunds(ctx context.Context, walletID string, req *models.WithdrawRequest, gatewayRef string) (*models.WithdrawResponse, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Lock wallet row and check balance
	var currentBalance float64
	lockQuery := `SELECT balance FROM wallets_passenger WHERE id = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, lockQuery, walletID).Scan(&currentBalance)
	if err != nil {
		return nil, fmt.Errorf("wallet not found or locked: %w", err)
	}

	if currentBalance < req.Amount {
		return nil, fmt.Errorf("insufficient wallet balance: available %.2f, requested %.2f", currentBalance, req.Amount)
	}

	// 2. Deduct balance
	newBalance := currentBalance - req.Amount
	updateQuery := `UPDATE wallets_passenger SET balance = $1, updated_at = NOW() WHERE id = $2`
	_, err = tx.ExecContext(ctx, updateQuery, newBalance, walletID)
	if err != nil {
		return nil, fmt.Errorf("failed to update wallet balance: %w", err)
	}

	// 3. Mask account number for description
	maskedAcc := req.AccountNumber
	if len(maskedAcc) > 4 {
		maskedAcc = maskedAcc[len(maskedAcc)-4:]
	}
	description := fmt.Sprintf("Bank Transfer to %s (•••• %s)", strings.TrimSpace(req.BankName), maskedAcc)

	// 4. Insert into wallet_transactions
	txID := uuid.New().String()
	insertTxQuery := `
		INSERT INTO wallet_transactions (
			id, wallet_id, amount, transaction_type, reference_type, 
			gateway_reference, description, status, balance_before, balance_after, created_at
		) VALUES (
			$1, $2, $3, 'debit', 'bank_payout',
			$4, $5, 'completed', $6, $7, NOW()
		)
	`
	_, err = tx.ExecContext(ctx, insertTxQuery, txID, walletID, req.Amount, gatewayRef, description, currentBalance, newBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to record withdrawal transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.WithdrawResponse{
		Success:          true,
		Message:          "Withdrawal processed successfully",
		TransactionID:    txID,
		GatewayReference: gatewayRef,
		Amount:           req.Amount,
		NewBalance:       newBalance,
		Status:           "completed",
	}, nil
}
