package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/pkg/payhere"
)

// WalletService coordinates wallet operations and PayHere payouts
type WalletService struct {
	walletRepo    *database.WalletRepository
	payhereClient *payhere.Client
	logger        *logrus.Logger
}

// NewWalletService creates a new wallet service
func NewWalletService(repo *database.WalletRepository, payhereClient *payhere.Client, logger *logrus.Logger) *WalletService {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &WalletService{
		walletRepo:    repo,
		payhereClient: payhereClient,
		logger:        logger,
	}
}

// GetWalletStatus returns current balance and payout tracker status
func (s *WalletService) GetWalletStatus(ctx context.Context, userID string) (*models.WalletStatusResponse, error) {
	return s.walletRepo.GetWalletStatus(ctx, userID)
}

// GetPendingEarnings returns individual pending earnings in holding period
func (s *WalletService) GetPendingEarnings(ctx context.Context, userID string) (*models.PendingEarningsResponse, error) {
	return s.walletRepo.GetPendingEarnings(ctx, userID)
}

// GetWalletTransactions returns paginated transactions
func (s *WalletService) GetWalletTransactions(ctx context.Context, userID string, page, limit int) (*models.WalletTransactionsResponse, error) {
	wallet, err := s.walletRepo.GetOrCreateWallet(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.walletRepo.GetWalletTransactions(ctx, wallet.ID, page, limit)
}

// RequestSpecialPayout flags early payout in tracker
func (s *WalletService) RequestSpecialPayout(ctx context.Context, userID string) (*models.SpecialPayoutResponse, error) {
	err := s.walletRepo.RequestSpecialPayout(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &models.SpecialPayoutResponse{
		Message: "Special payout requested successfully. Funds will be deposited in the next 24 hours.",
		Status:  "success",
	}, nil
}

// Withdraw processes a bank payout withdrawal via PayHere Sandbox
func (s *WalletService) Withdraw(ctx context.Context, userID string, req *models.WithdrawRequest) (*models.WithdrawResponse, error) {
	if req.Amount <= 0 {
		return nil, fmt.Errorf("invalid withdrawal amount: must be greater than zero")
	}

	wallet, err := s.walletRepo.GetOrCreateWallet(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load wallet: %w", err)
	}

	if wallet.Balance < req.Amount {
		return nil, fmt.Errorf("insufficient wallet balance: available %.2f, requested %.2f", wallet.Balance, req.Amount)
	}

	// 1. Process payout via PayHere
	refID := fmt.Sprintf("%d", time.Now().UnixNano()%1000000000)
	payoutReq := payhere.PayoutRequest{
		ReferenceID:       refID,
		Amount:            req.Amount,
		Currency:          "LKR",
		BankName:          req.BankName,
		BankCode:          req.BankCode,
		BranchCode:        req.BranchCode,
		AccountNumber:     req.AccountNumber,
		AccountHolderName: req.AccountHolderName,
		Description:       fmt.Sprintf("Bus Owner Payout to %s", req.AccountHolderName),
	}

	payoutRes, err := s.payhereClient.ProcessPayout(ctx, payoutReq)
	if err != nil {
		s.logger.WithError(err).Error("PayHere payout failed")
		return nil, fmt.Errorf("payment processor failed: %w", err)
	}

	if !payoutRes.Success {
		return nil, fmt.Errorf("payout rejected by gateway: %s", payoutRes.Message)
	}

	// 2. Safely deduct balance and record in database
	res, err := s.walletRepo.WithdrawFunds(ctx, wallet.ID, req, payoutRes.GatewayReference)
	if err != nil {
		s.logger.WithError(err).Error("Database deduction failed after gateway approval")
		return nil, err
	}

	res.Message = fmt.Sprintf("Withdrawal of LKR %.2f processed successfully via PayHere (%s)", req.Amount, payoutRes.GatewayReference)
	return res, nil
}
