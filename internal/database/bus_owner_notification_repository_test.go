package database

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNotificationDB struct {
	db *sql.DB
}

func (m *mockNotificationDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}

func (m *mockNotificationDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

func (m *mockNotificationDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return m.db.QueryRow(query, args...)
}

func (m *mockNotificationDB) Get(dest interface{}, query string, args ...interface{}) error {
	return nil
}

func (m *mockNotificationDB) Select(dest interface{}, query string, args ...interface{}) error {
	return nil
}

func (m *mockNotificationDB) Ping() error {
	return m.db.Ping()
}

func (m *mockNotificationDB) Close() error {
	return m.db.Close()
}

func TestBusOwnerNotificationRepository(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	mockDB := &mockNotificationDB{db: sqlDB}

	// Expect EnsureTable during NewBusOwnerNotificationRepository
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS bus_owner_notifications`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewBusOwnerNotificationRepository(mockDB)

	ownerID := uuid.New().String()

	t.Run("Create Notification", func(t *testing.T) {
		notificationID := uuid.New().String()
		now := time.Now()

		mock.ExpectQuery(`INSERT INTO bus_owner_notifications`).
			WithArgs(
				notificationID,
				ownerID,
				models.NotificationTypePermitExpiry,
				"Permit Expiring Soon",
				"Expires in 30 days",
				sqlmock.AnyArg(),
				false,
				sqlmock.AnyArg(),
				nil,
			).
			WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

		notification := &models.BusOwnerNotification{
			ID:         notificationID,
			BusOwnerID: ownerID,
			Type:       models.NotificationTypePermitExpiry,
			Title:      "Permit Expiring Soon",
			Message:    "Expires in 30 days",
			Metadata:   models.NotificationMetadata{"days_left": 30},
			IsRead:     false,
			CreatedAt:  now,
		}

		err := repo.Create(notification)
		assert.NoError(t, err)
	})

	t.Run("GetUnreadCount", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM bus_owner_notifications WHERE bus_owner_id = \$1 AND is_read = FALSE`).
			WithArgs(ownerID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

		count, err := repo.GetUnreadCount(ownerID)
		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("MarkAsRead", func(t *testing.T) {
		notificationID := uuid.New().String()
		mock.ExpectExec(`UPDATE bus_owner_notifications SET is_read = TRUE, read_at = NOW\(\) WHERE id = \$1 AND bus_owner_id = \$2`).
			WithArgs(notificationID, ownerID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.MarkAsRead(notificationID, ownerID)
		assert.NoError(t, err)
	})

	t.Run("MarkAllAsRead", func(t *testing.T) {
		mock.ExpectExec(`UPDATE bus_owner_notifications SET is_read = TRUE, read_at = NOW\(\) WHERE bus_owner_id = \$1 AND is_read = FALSE`).
			WithArgs(ownerID).
			WillReturnResult(sqlmock.NewResult(0, 5))

		err := repo.MarkAllAsRead(ownerID)
		assert.NoError(t, err)
	})

	t.Run("HasPermitExpiryNotificationBeenSentToday", func(t *testing.T) {
		permitID := uuid.New().String()
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs(ownerID, models.NotificationTypePermitExpiry, permitID, 30).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := repo.HasPermitExpiryNotificationBeenSentToday(ownerID, permitID, 30)
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}
