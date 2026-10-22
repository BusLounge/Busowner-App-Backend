package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BusOwnerNotificationRepository handles database operations for bus_owner_notifications table
type BusOwnerNotificationRepository struct {
	db DB
}

// NewBusOwnerNotificationRepository creates a new BusOwnerNotificationRepository
func NewBusOwnerNotificationRepository(db DB) *BusOwnerNotificationRepository {
	repo := &BusOwnerNotificationRepository{db: db}
	// Automatically ensure table and indices exist
	_ = repo.EnsureTable()
	return repo
}

// EnsureTable creates the table and indices if they do not exist
func (r *BusOwnerNotificationRepository) EnsureTable() error {
	ddl := `
		CREATE TABLE IF NOT EXISTS bus_owner_notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			bus_owner_id UUID NOT NULL REFERENCES bus_owners(id) ON DELETE CASCADE,
			type VARCHAR(50) NOT NULL,
			title VARCHAR(255) NOT NULL,
			message TEXT NOT NULL,
			metadata JSONB DEFAULT '{}',
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			read_at TIMESTAMP WITH TIME ZONE
		);

		CREATE INDEX IF NOT EXISTS idx_bus_owner_notifications_owner 
			ON bus_owner_notifications(bus_owner_id, is_read, created_at DESC);

		CREATE INDEX IF NOT EXISTS idx_bus_owner_notifications_type 
			ON bus_owner_notifications(bus_owner_id, type, created_at DESC);

		CREATE INDEX IF NOT EXISTS idx_bus_owner_notifications_created 
			ON bus_owner_notifications(created_at DESC);
	`
	_, err := r.db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("failed to ensure bus_owner_notifications table: %w", err)
	}
	return nil
}

// Create inserts a new notification for a bus owner
func (r *BusOwnerNotificationRepository) Create(n *models.BusOwnerNotification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO bus_owner_notifications (
			id, bus_owner_id, type, title, message, metadata, is_read, created_at, read_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		RETURNING created_at
	`

	err := r.db.QueryRow(
		query,
		n.ID,
		n.BusOwnerID,
		n.Type,
		n.Title,
		n.Message,
		n.Metadata,
		n.IsRead,
		n.CreatedAt,
		n.ReadAt,
	).Scan(&n.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}

	return nil
}

// GetByOwnerID retrieves paginated notifications for a bus owner
func (r *BusOwnerNotificationRepository) GetByOwnerID(busOwnerID string, limit, offset int) ([]models.BusOwnerNotification, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// 1. Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM bus_owner_notifications WHERE bus_owner_id = $1`
	if err := r.db.QueryRow(countQuery, busOwnerID).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to get notifications count: %w", err)
	}

	// 2. Fetch notifications
	query := `
		SELECT id, bus_owner_id, type, title, message, metadata, is_read, created_at, read_at
		FROM bus_owner_notifications
		WHERE bus_owner_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, busOwnerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]models.BusOwnerNotification, 0)
	for rows.Next() {
		var n models.BusOwnerNotification
		var readAt sql.NullTime

		err := rows.Scan(
			&n.ID,
			&n.BusOwnerID,
			&n.Type,
			&n.Title,
			&n.Message,
			&n.Metadata,
			&n.IsRead,
			&n.CreatedAt,
			&readAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}

		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}

		notifications = append(notifications, n)
	}

	return notifications, totalCount, nil
}

// GetUnreadCount retrieves the number of unread notifications for a bus owner
func (r *BusOwnerNotificationRepository) GetUnreadCount(busOwnerID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM bus_owner_notifications WHERE bus_owner_id = $1 AND is_read = FALSE`
	err := r.db.QueryRow(query, busOwnerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread notifications count: %w", err)
	}
	return count, nil
}

// MarkAsRead marks a single notification as read
func (r *BusOwnerNotificationRepository) MarkAsRead(notificationID, busOwnerID string) error {
	query := `
		UPDATE bus_owner_notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE id = $1 AND bus_owner_id = $2
	`
	res, err := r.db.Exec(query, notificationID, busOwnerID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// MarkAllAsRead marks all unread notifications as read for a bus owner
func (r *BusOwnerNotificationRepository) MarkAllAsRead(busOwnerID string) error {
	query := `
		UPDATE bus_owner_notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE bus_owner_id = $1 AND is_read = FALSE
	`
	_, err := r.db.Exec(query, busOwnerID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}
	return nil
}

// HasPermitExpiryNotificationBeenSentToday checks if an expiry warning for a permit at a given threshold
// has already been recorded today to avoid duplicate notifications.
func (r *BusOwnerNotificationRepository) HasPermitExpiryNotificationBeenSentToday(busOwnerID, permitID string, daysLeft int) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM bus_owner_notifications
			WHERE bus_owner_id = $1
			  AND type = $2
			  AND metadata->>'permit_id' = $3
			  AND (metadata->>'days_left')::int = $4
			  AND created_at >= CURRENT_DATE
		)
	`
	err := r.db.QueryRow(query, busOwnerID, models.NotificationTypePermitExpiry, permitID, daysLeft).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check duplicate permit expiry notification: %w", err)
	}
	return exists, nil
}
