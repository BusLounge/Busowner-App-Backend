package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testMockDB struct {
	db *sql.DB
}

func (m *testMockDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}

func (m *testMockDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

func (m *testMockDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return m.db.QueryRow(query, args...)
}

func (m *testMockDB) Get(dest interface{}, query string, args ...interface{}) error {
	return nil
}

func (m *testMockDB) Select(dest interface{}, query string, args ...interface{}) error {
	return nil
}

func (m *testMockDB) Ping() error {
	return m.db.Ping()
}

func (m *testMockDB) Close() error {
	return m.db.Close()
}

func setupNotificationTest(t *testing.T) (*gin.Engine, *BusOwnerNotificationHandler, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	// NewBusOwnerNotificationRepository runs EnsureTable
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS bus_owner_notifications`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mDB := &testMockDB{db: sqlDB}
	notifRepo := database.NewBusOwnerNotificationRepository(mDB)
	ownerRepo := database.NewBusOwnerRepository(mDB)

	handler := NewBusOwnerNotificationHandler(notifRepo, ownerRepo)
	router := gin.New()

	cleanup := func() {
		sqlDB.Close()
	}

	return router, handler, mock, cleanup
}

func busOwnerRows(ownerID, userID string) *sqlmock.Rows {
	columns := []string{
		"id", "user_id", "company_name", "license_number", "contact_person",
		"address", "city", "state", "country", "postal_code", "verification_status",
		"verification_documents", "business_email", "business_phone", "tax_id",
		"bank_account_details", "total_buses", "profile_completed",
		"identity_or_incorporation_no", "created_at", "updated_at",
	}
	now := time.Now()
	return sqlmock.NewRows(columns).AddRow(
		ownerID, userID, "Test Transport Ltd", "LIC-12345", "John Doe",
		"123 Main St", "Colombo", "Western", "Sri Lanka", "00100", "verified",
		[]byte("{}"), "owner@example.com", "+94771234567", "TAX-999",
		[]byte("{}"), 5, true,
		"NIC-199012345V", now, now,
	)
}

func TestGetNotifications_Unauthorized(t *testing.T) {
	router, handler, _, cleanup := setupNotificationTest(t)
	defer cleanup()

	router.GET("/notifications", handler.GetNotifications)

	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetNotifications_OwnerNotFound(t *testing.T) {
	router, handler, mock, cleanup := setupNotificationTest(t)
	defer cleanup()

	userID := uuid.New()
	router.GET("/notifications", func(c *gin.Context) {
		c.Set(middleware.UserContextKey, middleware.UserContext{UserID: userID})
		handler.GetNotifications(c)
	})

	mock.ExpectQuery(`SELECT (.+) FROM bus_owners WHERE user_id = \$1`).
		WithArgs(userID.String()).
		WillReturnError(sql.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetNotifications_Success(t *testing.T) {
	router, handler, mock, cleanup := setupNotificationTest(t)
	defer cleanup()

	userID := uuid.New()
	ownerID := uuid.New().String()
	notifID := uuid.New().String()
	now := time.Now()

	router.GET("/notifications", func(c *gin.Context) {
		c.Set(middleware.UserContextKey, middleware.UserContext{UserID: userID})
		handler.GetNotifications(c)
	})

	// 1. Bus owner lookup
	mock.ExpectQuery(`SELECT (.+) FROM bus_owners WHERE user_id = \$1`).
		WithArgs(userID.String()).
		WillReturnRows(busOwnerRows(ownerID, userID.String()))

	// 2. Count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM bus_owner_notifications WHERE bus_owner_id = \$1`).
		WithArgs(ownerID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// 3. Paginated notifications query
	notifCols := []string{
		"id", "bus_owner_id", "type", "title", "message", "metadata", "is_read", "created_at", "read_at",
	}
	mock.ExpectQuery(`SELECT (.+) FROM bus_owner_notifications WHERE bus_owner_id = \$1 ORDER BY created_at DESC LIMIT \$2 OFFSET \$3`).
		WithArgs(ownerID, 20, 0).
		WillReturnRows(sqlmock.NewRows(notifCols).AddRow(
			notifID, ownerID, models.NotificationTypePermitExpiry,
			"Permit Expiring Soon", "Permit expires in 30 days",
			[]byte(`{"days_left":30}`), false, now, nil,
		))

	// 4. Unread count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM bus_owner_notifications WHERE bus_owner_id = \$1 AND is_read = FALSE`).
		WithArgs(ownerID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	req := httptest.NewRequest(http.MethodGet, "/notifications?page=1&limit=20", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.NotificationListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.TotalCount)
	assert.Equal(t, 1, resp.UnreadCount)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.Limit)
	require.Len(t, resp.Notifications, 1)
	assert.Equal(t, notifID, resp.Notifications[0].ID)
	assert.Equal(t, models.NotificationTypePermitExpiry, resp.Notifications[0].Type)
	assert.False(t, resp.Notifications[0].IsRead)
}

func TestGetUnreadCount_Success(t *testing.T) {
	router, handler, mock, cleanup := setupNotificationTest(t)
	defer cleanup()

	userID := uuid.New()
	ownerID := uuid.New().String()

	router.GET("/notifications/unread-count", func(c *gin.Context) {
		c.Set(middleware.UserContextKey, middleware.UserContext{UserID: userID})
		handler.GetUnreadCount(c)
	})

	// 1. Bus owner lookup
	mock.ExpectQuery(`SELECT (.+) FROM bus_owners WHERE user_id = \$1`).
		WithArgs(userID.String()).
		WillReturnRows(busOwnerRows(ownerID, userID.String()))

	// 2. Unread count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM bus_owner_notifications WHERE bus_owner_id = \$1 AND is_read = FALSE`).
		WithArgs(ownerID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))

	req := httptest.NewRequest(http.MethodGet, "/notifications/unread-count", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]int
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 4, resp["unread_count"])
}

func TestMarkAsRead_Success(t *testing.T) {
	router, handler, mock, cleanup := setupNotificationTest(t)
	defer cleanup()

	userID := uuid.New()
	ownerID := uuid.New().String()
	notifID := uuid.New().String()

	router.PATCH("/notifications/:id/read", func(c *gin.Context) {
		c.Set(middleware.UserContextKey, middleware.UserContext{UserID: userID})
		handler.MarkAsRead(c)
	})

	// 1. Bus owner lookup
	mock.ExpectQuery(`SELECT (.+) FROM bus_owners WHERE user_id = \$1`).
		WithArgs(userID.String()).
		WillReturnRows(busOwnerRows(ownerID, userID.String()))

	// 2. Mark as read UPDATE
	mock.ExpectExec(`UPDATE bus_owner_notifications SET is_read = TRUE, read_at = NOW\(\) WHERE id = \$1 AND bus_owner_id = \$2`).
		WithArgs(notifID, ownerID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/notifications/%s/read", notifID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMarkAsRead_InvalidUUID(t *testing.T) {
	router, handler, mock, cleanup := setupNotificationTest(t)
	defer cleanup()

	userID := uuid.New()
	ownerID := uuid.New().String()

	router.PATCH("/notifications/:id/read", func(c *gin.Context) {
		c.Set(middleware.UserContextKey, middleware.UserContext{UserID: userID})
		handler.MarkAsRead(c)
	})

	mock.ExpectQuery(`SELECT (.+) FROM bus_owners WHERE user_id = \$1`).
		WithArgs(userID.String()).
		WillReturnRows(busOwnerRows(ownerID, userID.String()))

	req := httptest.NewRequest(http.MethodPatch, "/notifications/not-a-valid-uuid/read", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMarkAsRead_NotFound(t *testing.T) {
	router, handler, mock, cleanup := setupNotificationTest(t)
	defer cleanup()

	userID := uuid.New()
	ownerID := uuid.New().String()
	notifID := uuid.New().String()

	router.PATCH("/notifications/:id/read", func(c *gin.Context) {
		c.Set(middleware.UserContextKey, middleware.UserContext{UserID: userID})
		handler.MarkAsRead(c)
	})

	mock.ExpectQuery(`SELECT (.+) FROM bus_owners WHERE user_id = \$1`).
		WithArgs(userID.String()).
		WillReturnRows(busOwnerRows(ownerID, userID.String()))

	// 0 rows affected => sql.ErrNoRows
	mock.ExpectExec(`UPDATE bus_owner_notifications SET is_read = TRUE, read_at = NOW\(\) WHERE id = \$1 AND bus_owner_id = \$2`).
		WithArgs(notifID, ownerID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/notifications/%s/read", notifID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMarkAllAsRead_Success(t *testing.T) {
	router, handler, mock, cleanup := setupNotificationTest(t)
	defer cleanup()

	userID := uuid.New()
	ownerID := uuid.New().String()

	router.PATCH("/notifications/read-all", func(c *gin.Context) {
		c.Set(middleware.UserContextKey, middleware.UserContext{UserID: userID})
		handler.MarkAllAsRead(c)
	})

	// 1. Bus owner lookup
	mock.ExpectQuery(`SELECT (.+) FROM bus_owners WHERE user_id = \$1`).
		WithArgs(userID.String()).
		WillReturnRows(busOwnerRows(ownerID, userID.String()))

	// 2. Mark all as read UPDATE
	mock.ExpectExec(`UPDATE bus_owner_notifications SET is_read = TRUE, read_at = NOW\(\) WHERE bus_owner_id = \$1 AND is_read = FALSE`).
		WithArgs(ownerID).
		WillReturnResult(sqlmock.NewResult(0, 3))

	req := httptest.NewRequest(http.MethodPatch, "/notifications/read-all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
