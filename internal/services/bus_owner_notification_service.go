package services

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/pkg/onesignal"
)

// BusOwnerNotificationService coordinates notifications for bus owners
type BusOwnerNotificationService struct {
	notificationRepo *database.BusOwnerNotificationRepository
	permitRepo       *database.RoutePermitRepository
	busOwnerRepo     *database.BusOwnerRepository
	userSessionRepo  *database.UserSessionRepository
	oneSignalService *onesignal.OneSignalService
	stopCh           chan struct{}
}

// NewBusOwnerNotificationService creates a new BusOwnerNotificationService
func NewBusOwnerNotificationService(
	notificationRepo *database.BusOwnerNotificationRepository,
	permitRepo *database.RoutePermitRepository,
	busOwnerRepo *database.BusOwnerRepository,
	userSessionRepo *database.UserSessionRepository,
	oneSignalService *onesignal.OneSignalService,
) *BusOwnerNotificationService {
	return &BusOwnerNotificationService{
		notificationRepo: notificationRepo,
		permitRepo:       permitRepo,
		busOwnerRepo:     busOwnerRepo,
		userSessionRepo:  userSessionRepo,
		oneSignalService: oneSignalService,
		stopCh:           make(chan struct{}),
	}
}

// ----------------------------------------------------------------------------
// Background Worker for Permit Expiry Checks
// ----------------------------------------------------------------------------

// StartExpiryScheduler starts the background worker for permit expiry checks
func (s *BusOwnerNotificationService) StartExpiryScheduler(interval time.Duration) {
	fmt.Printf("🕐 Starting Permit Expiry Notification Scheduler (interval: %v)\n", interval)
	go s.runExpiryWorker(interval)
}

// StopExpiryScheduler stops the background worker
func (s *BusOwnerNotificationService) StopExpiryScheduler() {
	fmt.Println("🛑 Stopping Permit Expiry Notification Scheduler")
	close(s.stopCh)
}

func (s *BusOwnerNotificationService) runExpiryWorker(interval time.Duration) {
	// Startup run after 15 seconds to allow full system startup
	select {
	case <-time.After(15 * time.Second):
		s.checkAndDispatchPermitExpiries()
	case <-s.stopCh:
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndDispatchPermitExpiries()
		case <-s.stopCh:
			return
		}
	}
}

// checkAndDispatchPermitExpiries checks thresholds: 30 days, 7 days, and 0 days (today)
func (s *BusOwnerNotificationService) checkAndDispatchPermitExpiries() {
	thresholds := []int{30, 7, 0}

	for _, days := range thresholds {
		permits, err := s.permitRepo.GetPermitsExpiringInDays(days)
		if err != nil {
			log.Printf("ERROR: Failed to query permits expiring in %d days: %v", days, err)
			continue
		}

		for _, permit := range permits {
			// Deduplication check: verify if notification was already sent today for this permit + threshold
			alreadySent, err := s.notificationRepo.HasPermitExpiryNotificationBeenSentToday(permit.BusOwnerID, permit.ID, days)
			if err != nil {
				log.Printf("WARNING: Failed to check deduplication for permit %s: %v", permit.ID, err)
			}
			if alreadySent {
				continue
			}

			var title, message string
			if days == 30 {
				title = "Permit Expiring Soon"
				message = fmt.Sprintf("Heads up: Route Permit #%s for Bus %s expires in 30 days. Prepare your renewal.",
					permit.PermitNumber, permit.BusRegistrationNumber)
			} else if days == 7 {
				title = "Action Required: Permit Expiry"
				message = fmt.Sprintf("Action Required: Route Permit #%s for Bus %s expires in 7 days.",
					permit.PermitNumber, permit.BusRegistrationNumber)
			} else { // 0 days
				title = "URGENT: Permit Expired"
				message = fmt.Sprintf("URGENT: Route Permit #%s for Bus %s expired today! Trips cannot legally operate under this permit.",
					permit.PermitNumber, permit.BusRegistrationNumber)
			}

			metadata := models.NotificationMetadata{
				"permit_id":               permit.ID,
				"permit_number":           permit.PermitNumber,
				"bus_registration_number": permit.BusRegistrationNumber,
				"days_left":               days,
				"expiry_date":             permit.ExpiryDate.Format("2006-01-02"),
			}

			notification := &models.BusOwnerNotification{
				ID:         uuid.New().String(),
				BusOwnerID: permit.BusOwnerID,
				Type:       models.NotificationTypePermitExpiry,
				Title:      title,
				Message:    message,
				Metadata:   metadata,
				IsRead:     false,
				CreatedAt:  time.Now(),
			}

			if err := s.notificationRepo.Create(notification); err != nil {
				log.Printf("ERROR: Failed to create permit expiry notification for owner %s: %v", permit.BusOwnerID, err)
				continue
			}

			// Push notification to active sessions
			s.pushToOneSignal(permit.BusOwnerID, title, message, metadata)
		}
	}
}

// ----------------------------------------------------------------------------
// Event Trigger 2: Permit Verification & Approvals (Approved / Rejected)
// ----------------------------------------------------------------------------

// NotifyPermitStatus creates notification when a route permit is approved or rejected
func (s *BusOwnerNotificationService) NotifyPermitStatus(
	busOwnerID string,
	permitID string,
	permitNumber string,
	busRegistrationNumber string,
	status string, // "approved" or "rejected"
	reason string,
) error {
	var title, message string
	if status == "approved" || status == "verified" {
		title = "Permit Approved"
		message = fmt.Sprintf("Route Permit #%s for Bus %s has been approved.", permitNumber, busRegistrationNumber)
	} else {
		title = "Permit Rejected"
		if reason != "" {
			message = fmt.Sprintf("Route Permit #%s for Bus %s was rejected. Reason: %s", permitNumber, busRegistrationNumber, reason)
		} else {
			message = fmt.Sprintf("Route Permit #%s for Bus %s was rejected.", permitNumber, busRegistrationNumber)
		}
	}

	metadata := models.NotificationMetadata{
		"permit_id":               permitID,
		"permit_number":           permitNumber,
		"bus_registration_number": busRegistrationNumber,
		"status":                  status,
		"rejection_reason":        reason,
	}

	notification := &models.BusOwnerNotification{
		ID:         uuid.New().String(),
		BusOwnerID: busOwnerID,
		Type:       models.NotificationTypePermitStatus,
		Title:      title,
		Message:    message,
		Metadata:   metadata,
		IsRead:     false,
		CreatedAt:  time.Now(),
	}

	if err := s.notificationRepo.Create(notification); err != nil {
		return fmt.Errorf("failed to save permit status notification: %w", err)
	}

	s.pushToOneSignal(busOwnerID, title, message, metadata)
	return nil
}

// ----------------------------------------------------------------------------
// Event Trigger 3: Trip Completion & Revenue Summary
// ----------------------------------------------------------------------------

// NotifyTripCompleted creates notification when a trip finishes
func (s *BusOwnerNotificationService) NotifyTripCompleted(
	busOwnerID string,
	tripID string,
	originCity string,
	destinationCity string,
	passengerCount int,
	totalRevenue float64,
) error {
	title := "Trip Completed"
	message := fmt.Sprintf("Trip completed: %s → %s. Total passengers: %d. Total revenue: LKR %s.",
		originCity, destinationCity, passengerCount, formatCurrency(totalRevenue))

	metadata := models.NotificationMetadata{
		"trip_id":          tripID,
		"origin_city":      originCity,
		"destination_city": destinationCity,
		"passenger_count":  passengerCount,
		"total_revenue":    totalRevenue,
	}

	notification := &models.BusOwnerNotification{
		ID:         uuid.New().String(),
		BusOwnerID: busOwnerID,
		Type:       models.NotificationTypeTripSummary,
		Title:      title,
		Message:    message,
		Metadata:   metadata,
		IsRead:     false,
		CreatedAt:  time.Now(),
	}

	if err := s.notificationRepo.Create(notification); err != nil {
		return fmt.Errorf("failed to save trip summary notification: %w", err)
	}

	s.pushToOneSignal(busOwnerID, title, message, metadata)
	return nil
}

// ----------------------------------------------------------------------------
// Event Trigger 4: Daily / Weekly Payout Settlement
// ----------------------------------------------------------------------------

// NotifyPayoutSettlement creates notification when a payout is transferred
func (s *BusOwnerNotificationService) NotifyPayoutSettlement(
	busOwnerID string,
	amount float64,
	bankName string,
	accountEnding string,
	reference string,
) error {
	title := "Payout Settled"
	message := fmt.Sprintf("LKR %s has been transferred to your %s account ending in %s.",
		formatCurrency(amount), bankName, accountEnding)

	metadata := models.NotificationMetadata{
		"amount":         amount,
		"bank_name":      bankName,
		"account_ending": accountEnding,
		"reference":      reference,
	}

	notification := &models.BusOwnerNotification{
		ID:         uuid.New().String(),
		BusOwnerID: busOwnerID,
		Type:       models.NotificationTypePayoutSettlement,
		Title:      title,
		Message:    message,
		Metadata:   metadata,
		IsRead:     false,
		CreatedAt:  time.Now(),
	}

	if err := s.notificationRepo.Create(notification); err != nil {
		return fmt.Errorf("failed to save payout settlement notification: %w", err)
	}

	s.pushToOneSignal(busOwnerID, title, message, metadata)
	return nil
}

// ----------------------------------------------------------------------------
// OneSignal Push Helper
// ----------------------------------------------------------------------------

func (s *BusOwnerNotificationService) pushToOneSignal(busOwnerID, title, message string, data map[string]interface{}) {
	if s.oneSignalService == nil {
		return
	}

	// 1. Get bus owner to find user_id
	owner, err := s.busOwnerRepo.GetByID(busOwnerID)
	if err != nil || owner == nil {
		return
	}

	// 2. Get active sessions for user_id
	ownerUUID, err := uuid.Parse(owner.UserID)
	if err != nil {
		return
	}

	sessions, err := s.userSessionRepo.GetActiveSessionsByUserID(ownerUUID)
	if err != nil || len(sessions) == 0 {
		return
	}

	var playerIDs []string
	for _, session := range sessions {
		if session.OneSignalPlayerID.Valid && session.OneSignalPlayerID.String != "" {
			playerIDs = append(playerIDs, session.OneSignalPlayerID.String)
		}
	}

	if len(playerIDs) == 0 {
		return
	}

	_, _ = s.oneSignalService.SendToPlayers(playerIDs, title, message, data)
}

func formatCurrency(amount float64) string {
	// Format with commas: e.g. 84,500
	intPart := int64(amount)
	sign := ""
	if intPart < 0 {
		sign = "-"
		intPart = -intPart
	}

	str := fmt.Sprintf("%d", intPart)
	n := len(str)
	if n <= 3 {
		return sign + str
	}

	var result []byte
	rem := n % 3
	if rem > 0 {
		result = append(result, str[:rem]...)
		if n > rem {
			result = append(result, ',')
		}
	}

	for i := rem; i < n; i += 3 {
		result = append(result, str[i:i+3]...)
		if i+3 < n {
			result = append(result, ',')
		}
	}

	return sign + string(result)
}
