package models

import "time"

// StaffTripItem represents a scheduled, active, or completed trip for an individual staff member
type StaffTripItem struct {
	ID                       string              `json:"id"`
	TripScheduleID           *string             `json:"trip_schedule_id,omitempty"`
	DepartureDatetime        time.Time           `json:"departure_datetime"`
	EstimatedDurationMinutes *int                `json:"estimated_duration_minutes,omitempty"`
	Status                   ScheduledTripStatus `json:"status"`
	Role                     string              `json:"role"` // "driver" or "conductor"
	BusRegistrationNumber    *string             `json:"bus_registration_number,omitempty"`
	BusType                  *string             `json:"bus_type,omitempty"`
	RouteNumber              *string             `json:"route_number,omitempty"`
	OriginCity               *string             `json:"origin_city,omitempty"`
	DestinationCity          *string             `json:"destination_city,omitempty"`
	Direction                *string             `json:"direction,omitempty"`
	BaseFare                 float64             `json:"base_fare"`
	CancellationReason       *string             `json:"cancellation_reason,omitempty"`
}
