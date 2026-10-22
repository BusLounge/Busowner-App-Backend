package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Standard notification types for bus owners
const (
	NotificationTypePermitExpiry     = "permit_expiry"
	NotificationTypePermitStatus     = "permit_status"
	NotificationTypeTripSummary      = "trip_summary"
	NotificationTypePayoutSettlement = "payout_settlement"
)

// NotificationMetadata stores additional context as JSONB
type NotificationMetadata map[string]interface{}

// Value implements the driver.Valuer interface
func (m NotificationMetadata) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// Scan implements the sql.Scanner interface
func (m *NotificationMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = make(map[string]interface{})
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	default:
		return errors.New("type assertion failed for NotificationMetadata")
	}
}

// BusOwnerNotification represents a persistent notification record for a bus owner
type BusOwnerNotification struct {
	ID         string               `json:"id" db:"id"`
	BusOwnerID string               `json:"bus_owner_id" db:"bus_owner_id"`
	Type       string               `json:"type" db:"type"`
	Title      string               `json:"title" db:"title"`
	Message    string               `json:"message" db:"message"`
	Metadata   NotificationMetadata `json:"metadata" db:"metadata"`
	IsRead     bool                 `json:"is_read" db:"is_read"`
	CreatedAt  time.Time            `json:"created_at" db:"created_at"`
	ReadAt     *time.Time           `json:"read_at,omitempty" db:"read_at"`
}

// NotificationListResponse represents the response when listing notifications
type NotificationListResponse struct {
	Notifications []BusOwnerNotification `json:"notifications"`
	UnreadCount   int                    `json:"unread_count"`
	TotalCount    int                    `json:"total_count"`
	Page          int                    `json:"page"`
	Limit         int                    `json:"limit"`
}
