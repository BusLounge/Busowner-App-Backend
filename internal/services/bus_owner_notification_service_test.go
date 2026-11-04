package services

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type svcMockDB struct {
	db *sql.DB
}

func (m *svcMockDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}

func (m *svcMockDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

func (m *svcMockDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return m.db.QueryRow(query, args...)
}

func (m *svcMockDB) Get(dest interface{}, query string, args ...interface{}) error {
	return nil
}

func (m *svcMockDB) Select(dest interface{}, query string, args ...interface{}) error {
	return nil
}

func (m *svcMockDB) Ping() error {
	return m.db.Ping()
}

func (m *svcMockDB) Close() error {
	return m.db.Close()
}

func setupNotificationServiceTest(t *testing.T) (*BusOwnerNotificationService, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	// EnsureTable runs on NewBusOwnerNotificationRepository
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS bus_owner_notifications`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mDB := &svcMockDB{db: sqlDB}
	notifRepo := database.NewBusOwnerNotificationRepository(mDB)
	permitRepo := database.NewRoutePermitRepository(mDB)
	ownerRepo := database.NewBusOwnerRepository(mDB)
	sessionRepo := database.NewUserSessionRepository(mDB)

	svc := NewBusOwnerNotificationService(notifRepo, permitRepo, ownerRepo, sessionRepo, nil)

	cleanup := func() {
		sqlDB.Close()
	}

	return svc, mock, cleanup
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0"},
		{50, "50"},
		{500, "500"},
		{1000, "1,000"},
		{15400, "15,400"},
		{84500, "84,500"},
		{1000000, "1,000,000"},
		{1234567, "1,234,567"},
		{-5000, "-5,000"},
	}

	for _, tt := range tests {
		result := formatCurrency(tt.input)
		assert.Equal(t, tt.expected, result, "for input %f", tt.input)
	}
}

func TestNotifyPermitStatus_Approved(t *testing.T) {
	svc, mock, cleanup := setupNotificationServiceTest(t)
	defer cleanup()

	ownerID := uuid.New().String()
	permitID := uuid.New().String()
	now := time.Now()

	mock.ExpectQuery(`INSERT INTO bus_owner_notifications`).
		WithArgs(
			sqlmock.AnyArg(),
			ownerID,
			models.NotificationTypePermitStatus,
			"Permit Approved",
			"Route Permit #WP-1234 for Bus ND-9988 has been approved.",
			sqlmock.AnyArg(),
			false,
			sqlmock.AnyArg(),
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

	err := svc.NotifyPermitStatus(ownerID, permitID, "WP-1234", "ND-9988", "approved", "")
	assert.NoError(t, err)
}

func TestNotifyPermitStatus_Rejected(t *testing.T) {
	svc, mock, cleanup := setupNotificationServiceTest(t)
	defer cleanup()

	ownerID := uuid.New().String()
	permitID := uuid.New().String()
	now := time.Now()

	mock.ExpectQuery(`INSERT INTO bus_owner_notifications`).
		WithArgs(
			sqlmock.AnyArg(),
			ownerID,
			models.NotificationTypePermitStatus,
			"Permit Rejected",
			"Route Permit #WP-1234 for Bus ND-9988 was rejected. Reason: Invalid insurance document",
			sqlmock.AnyArg(),
			false,
			sqlmock.AnyArg(),
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

	err := svc.NotifyPermitStatus(ownerID, permitID, "WP-1234", "ND-9988", "rejected", "Invalid insurance document")
	assert.NoError(t, err)
}

func TestNotifyTripCompleted(t *testing.T) {
	svc, mock, cleanup := setupNotificationServiceTest(t)
	defer cleanup()

	ownerID := uuid.New().String()
	tripID := uuid.New().String()
	now := time.Now()

	mock.ExpectQuery(`INSERT INTO bus_owner_notifications`).
		WithArgs(
			sqlmock.AnyArg(),
			ownerID,
			models.NotificationTypeTripSummary,
			"Trip Completed",
			"Trip completed: Colombo → Kandy. Total passengers: 42. Total revenue: LKR 46,200.",
			sqlmock.AnyArg(),
			false,
			sqlmock.AnyArg(),
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

	err := svc.NotifyTripCompleted(ownerID, tripID, "Colombo", "Kandy", 42, 46200.0)
	assert.NoError(t, err)
}

func TestNotifyPayoutSettlement(t *testing.T) {
	svc, mock, cleanup := setupNotificationServiceTest(t)
	defer cleanup()

	ownerID := uuid.New().String()
	now := time.Now()

	mock.ExpectQuery(`INSERT INTO bus_owner_notifications`).
		WithArgs(
			sqlmock.AnyArg(),
			ownerID,
			models.NotificationTypePayoutSettlement,
			"Payout Settled",
			"LKR 125,000 has been transferred to your Commercial Bank account ending in 4521.",
			sqlmock.AnyArg(),
			false,
			sqlmock.AnyArg(),
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

	err := svc.NotifyPayoutSettlement(ownerID, 125000.0, "Commercial Bank", "4521", "TXN-987654")
	assert.NoError(t, err)
}
