package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// TripSeatRepository handles trip_seats database operations
type TripSeatRepository struct {
	db *sqlx.DB
}

// NewTripSeatRepository creates a new TripSeatRepository
func NewTripSeatRepository(db *sqlx.DB) *TripSeatRepository {
	return &TripSeatRepository{db: db}
}

// CreateTripSeatsFromLayout creates trip_seats from a seat layout template
// This is called when assigning a seat layout to a scheduled trip
func (r *TripSeatRepository) CreateTripSeatsFromLayout(scheduledTripID, seatLayoutID string, baseFare float64) (int, error) {
	// First, delete any existing trip seats for this trip
	_, err := r.db.Exec(`DELETE FROM trip_seats WHERE scheduled_trip_id = $1`, scheduledTripID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete existing trip seats: %w", err)
	}

	// Get seats from the layout template
	query := `
		SELECT 
			seat_number,
			row_number,
			position,
			CASE 
				WHEN is_window_seat THEN 'window'
				WHEN is_aisle_seat THEN 'aisle'
				ELSE 'standard'
			END as seat_type
		FROM bus_seat_layout_seats
		WHERE template_id = $1
		ORDER BY row_number, position
	`

	type layoutSeat struct {
		SeatNumber string `db:"seat_number"`
		RowNumber  int    `db:"row_number"`
		Position   int    `db:"position"`
		SeatType   string `db:"seat_type"`
	}

	var seats []layoutSeat
	err = r.db.Select(&seats, query, seatLayoutID)
	if err != nil {
		return 0, fmt.Errorf("failed to get layout seats: %w", err)
	}

	if len(seats) == 0 {
		return 0, fmt.Errorf("no seats found in layout template")
	}

	// Insert trip seats
	insertQuery := `
		INSERT INTO trip_seats (
			scheduled_trip_id, seat_number, seat_type, row_number, position,
			seat_price, status, booking_type
		) VALUES ($1, $2, $3, $4, $5, $6, 'available', NULL)
	`

	count := 0
	for _, seat := range seats {
		_, err := r.db.Exec(insertQuery,
			scheduledTripID,
			seat.SeatNumber,
			seat.SeatType,
			seat.RowNumber,
			seat.Position,
			baseFare,
		)
		if err != nil {
			return count, fmt.Errorf("failed to insert trip seat %s: %w", seat.SeatNumber, err)
		}
		count++
	}

	return count, nil
}

// GetByScheduledTripID returns all seats for a scheduled trip
func (r *TripSeatRepository) GetByScheduledTripID(scheduledTripID string) ([]models.TripSeat, error) {
	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
			   block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE scheduled_trip_id = $1
		ORDER BY row_number, position
	`

	var seats []models.TripSeat
	err := r.db.Select(&seats, query, scheduledTripID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// GetByScheduledTripIDWithBookingInfo returns seats with booking details
func (r *TripSeatRepository) GetByScheduledTripIDWithBookingInfo(scheduledTripID string) ([]models.TripSeatWithBookingInfo, error) {
	query := `
		SELECT 
			ts.id, ts.scheduled_trip_id, ts.seat_number, ts.seat_type, ts.row_number, ts.position,
			ts.seat_price, ts.status, ts.booking_type, ts.bus_booking_seat_id, ts.manual_booking_id,
			ts.block_reason, ts.blocked_by_user_id, ts.blocked_at, ts.created_at, ts.updated_at,
			COALESCE(mb.passenger_name, bbs.passenger_name) as passenger_name,
			COALESCE(mb.passenger_phone, bbs.passenger_phone) as passenger_phone,
			COALESCE(mb.booking_reference, b.booking_reference) as booking_reference,
			COALESCE(mb.payment_status, b.payment_status::text) as payment_status
		FROM trip_seats ts
		LEFT JOIN manual_seat_bookings mb ON ts.manual_booking_id = mb.id
		LEFT JOIN bus_booking_seats bbs ON ts.bus_booking_seat_id = bbs.id
		LEFT JOIN bus_bookings bb ON bbs.bus_booking_id = bb.id
		LEFT JOIN bookings b ON bb.booking_id = b.id
		WHERE ts.scheduled_trip_id = $1
		ORDER BY ts.row_number, ts.position
	`

	var seats []models.TripSeatWithBookingInfo
	err := r.db.Select(&seats, query, scheduledTripID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// GetByID returns a single trip seat by ID
func (r *TripSeatRepository) GetByID(id string) (*models.TripSeat, error) {
	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
			   block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE id = $1
	`

	var seat models.TripSeat
	err := r.db.Get(&seat, query, id)
	if err != nil {
		return nil, err
	}

	return &seat, nil
}

// GetByIDs returns multiple trip seats by IDs
func (r *TripSeatRepository) GetByIDs(ids []string) ([]models.TripSeat, error) {
	if len(ids) == 0 {
		return []models.TripSeat{}, nil
	}

	query, args, err := sqlx.In(`
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
			   block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE id IN (?)
		ORDER BY row_number, position
	`, ids)
	if err != nil {
		return nil, err
	}

	query = r.db.Rebind(query)

	var seats []models.TripSeat
	err = r.db.Select(&seats, query, args...)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// GetSummary returns seat availability summary for a trip
func (r *TripSeatRepository) GetSummary(scheduledTripID string) (*models.TripSeatSummary, error) {
	query := `
		SELECT 
			scheduled_trip_id,
			COUNT(*) as total_seats,
			COUNT(*) FILTER (WHERE status = 'available') as available_seats,
			COUNT(*) FILTER (WHERE status = 'booked') as booked_seats,
			COUNT(*) FILTER (WHERE status = 'blocked') as blocked_seats,
			COUNT(*) FILTER (WHERE status = 'reserved') as reserved_seats,
			COUNT(*) FILTER (WHERE booking_type = 'app') as app_bookings,
			COUNT(*) FILTER (WHERE booking_type = 'phone') as phone_bookings,
			COUNT(*) FILTER (WHERE booking_type = 'agent') as agent_bookings,
			COUNT(*) FILTER (WHERE booking_type = 'walk_in') as walk_in_bookings
		FROM trip_seats
		WHERE scheduled_trip_id = $1
		GROUP BY scheduled_trip_id
	`

	var summary models.TripSeatSummary
	err := r.db.Get(&summary, query, scheduledTripID)
	if err != nil {
		if err == sql.ErrNoRows {
			// No seats exist yet
			return &models.TripSeatSummary{ScheduledTripID: scheduledTripID}, nil
		}
		return nil, err
	}

	return &summary, nil
}

// BlockSeats blocks one or more seats
func (r *TripSeatRepository) BlockSeats(seatIDs []string, blockedByUserID, reason string) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET status = 'blocked',
			booking_type = 'blocked',
			block_reason = ?,
			blocked_by_user_id = ?,
			blocked_at = ?,
			updated_at = ?
		WHERE id IN (?) AND status = 'available'
	`, reason, blockedByUserID, time.Now(), time.Now(), seatIDs)
	if err != nil {
		return 0, err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// UnblockSeats unblocks one or more seats
func (r *TripSeatRepository) UnblockSeats(seatIDs []string) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET status = 'available',
			booking_type = NULL,
			block_reason = NULL,
			blocked_by_user_id = NULL,
			blocked_at = NULL,
			updated_at = ?
		WHERE id IN (?) AND status = 'blocked'
	`, time.Now(), seatIDs)
	if err != nil {
		return 0, err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// UpdateSeatPrices updates the price for multiple seats
func (r *TripSeatRepository) UpdateSeatPrices(seatIDs []string, newPrice float64) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET seat_price = ?,
			updated_at = ?
		WHERE id IN (?)
	`, newPrice, time.Now(), seatIDs)
	if err != nil {
		return 0, err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// BookSeatsForManualBooking marks seats as booked for a manual booking
func (r *TripSeatRepository) BookSeatsForManualBooking(seatIDs []string, manualBookingID string, bookingType models.TripSeatBookingType) error {
	if len(seatIDs) == 0 {
		return nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats
		SET status = 'booked',
			booking_type = ?,
			manual_booking_id = ?,
			updated_at = ?
		WHERE id IN (?) AND status = 'available'
	`, string(bookingType), manualBookingID, time.Now(), seatIDs)
	if err != nil {
		return err
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if int(rowsAffected) != len(seatIDs) {
		return fmt.Errorf("some seats are not available, expected %d, updated %d", len(seatIDs), rowsAffected)
	}

	return nil
}

// ReleaseSeatsFromManualBooking releases seats when a manual booking is cancelled
func (r *TripSeatRepository) ReleaseSeatsFromManualBooking(manualBookingID string) error {
	query := `
		UPDATE trip_seats
		SET status = 'available',
			booking_type = NULL,
			manual_booking_id = NULL,
			updated_at = $1
		WHERE manual_booking_id = $2
	`

	_, err := r.db.Exec(query, time.Now(), manualBookingID)
	return err
}

// CheckSeatsAvailable checks if all specified seats are available
func (r *TripSeatRepository) CheckSeatsAvailable(seatIDs []string) (bool, error) {
	if len(seatIDs) == 0 {
		return true, nil
	}

	query, args, err := sqlx.In(`
		SELECT COUNT(*) FROM trip_seats
		WHERE id IN (?) AND status = 'available'
	`, seatIDs)
	if err != nil {
		return false, err
	}

	query = r.db.Rebind(query)

	var count int
	err = r.db.Get(&count, query, args...)
	if err != nil {
		return false, err
	}

	return count == len(seatIDs), nil
}

// DeleteByScheduledTripID deletes all trip seats for a scheduled trip
func (r *TripSeatRepository) DeleteByScheduledTripID(scheduledTripID string) error {
	_, err := r.db.Exec(`DELETE FROM trip_seats WHERE scheduled_trip_id = $1`, scheduledTripID)
	return err
}

// GetAvailableSeats returns only available seats for a trip
func (r *TripSeatRepository) GetAvailableSeats(scheduledTripID string) ([]models.TripSeat, error) {
	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
			   seat_price, status, booking_type, created_at, updated_at
		FROM trip_seats
		WHERE scheduled_trip_id = $1 AND status = 'available'
		ORDER BY row_number, position
	`

	var seats []models.TripSeat
	err := r.db.Select(&seats, query, scheduledTripID)
	if err != nil {
		return nil, err
	}

	return seats, nil
}

// HasBookedSeats checks if any seats have already been booked or reserved for a scheduled trip
func (r *TripSeatRepository) HasBookedSeats(scheduledTripID string) (bool, error) {
	var count int
	err := r.db.Get(&count, `SELECT COUNT(*) FROM trip_seats WHERE scheduled_trip_id = $1 AND status IN ('booked', 'reserved')`, scheduledTripID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetTripIDsWithBookings returns a list of trip IDs from the provided slice that currently have booked or reserved seats
func (r *TripSeatRepository) GetTripIDsWithBookings(tripIDs []string) ([]string, error) {
	if len(tripIDs) == 0 {
		return []string{}, nil
	}
	query := `
		SELECT DISTINCT scheduled_trip_id 
		FROM trip_seats 
		WHERE scheduled_trip_id = ANY($1::text[]) 
		  AND status IN ('booked', 'reserved')
	`
	var bookedTripIDs []string
	err := r.db.Select(&bookedTripIDs, query, pq.Array(tripIDs))
	if err != nil {
		return nil, err
	}
	return bookedTripIDs, nil
}

// UpdateAvailableSeatPrices updates the price of all available (unbooked) seats for a scheduled trip
func (r *TripSeatRepository) UpdateAvailableSeatPrices(scheduledTripID string, newPrice float64) (int, error) {
	query := `
		UPDATE trip_seats
		SET seat_price = $1, updated_at = NOW()
		WHERE scheduled_trip_id = $2 AND status = 'available'
	`
	result, err := r.db.Exec(query, newPrice, scheduledTripID)
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// ChangeAppBookingSeatResult contains information about a changed app booking seat
type ChangeAppBookingSeatResult struct {
	NewSeat         *models.TripSeat
	OldSeatNumber   string
	NewSeatNumber   string
	PassengerName   string
	PlayerIDs       []string
	RouteName       string
	ScheduledTripID string
}

// ChangeAppBookingSeat performs an atomic transaction to change an app-booked seat
func (r *TripSeatRepository) ChangeAppBookingSeat(currentTripSeatID string, newTripSeatID string, reason string) (*ChangeAppBookingSeatResult, error) {
	if currentTripSeatID == newTripSeatID {
		return nil, fmt.Errorf("current seat and new seat cannot be the same")
	}

	tx, err := r.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// 1. Lock and check current seat
	var currentSeat models.TripSeat
	getCurrentSeatQuery := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
		       seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
		       block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE id = $1
		FOR UPDATE
	`
	err = tx.Get(&currentSeat, getCurrentSeatQuery, currentTripSeatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("current seat not found")
		}
		return nil, fmt.Errorf("failed to get current seat: %w", err)
	}

	if currentSeat.Status != models.TripSeatStatusBooked {
		return nil, fmt.Errorf("seat %s is not booked", currentSeat.SeatNumber)
	}

	if currentSeat.BookingType == nil || *currentSeat.BookingType != models.TripSeatBookingTypeApp || currentSeat.BusBookingSeatID == nil {
		return nil, fmt.Errorf("seat %s is not an app booking", currentSeat.SeatNumber)
	}

	// 2. Lock and check bus_booking_seats
	var bookingSeat struct {
		ID             string  `db:"id"`
		BusBookingID   string  `db:"bus_booking_id"`
		PassengerName  string  `db:"passenger_name"`
		PassengerPhone *string `db:"passenger_phone"`
		Status         string  `db:"status"`
	}
	getBookingSeatQuery := `
		SELECT id, bus_booking_id, passenger_name, passenger_phone, status
		FROM bus_booking_seats
		WHERE id = $1
		FOR UPDATE
	`
	err = tx.Get(&bookingSeat, getBookingSeatQuery, *currentSeat.BusBookingSeatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("passenger booking seat record not found")
		}
		return nil, fmt.Errorf("failed to get booking seat: %w", err)
	}

	if bookingSeat.Status == "cancelled" {
		return nil, fmt.Errorf("cannot change seat for a cancelled booking")
	}

	// 3. Lock and check new seat
	var newSeat models.TripSeat
	getNewSeatQuery := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
		       seat_price, status, booking_type, bus_booking_seat_id, manual_booking_id,
		       block_reason, blocked_by_user_id, blocked_at, created_at, updated_at
		FROM trip_seats
		WHERE id = $1
		FOR UPDATE
	`
	err = tx.Get(&newSeat, getNewSeatQuery, newTripSeatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("new seat not found")
		}
		return nil, fmt.Errorf("failed to get new seat: %w", err)
	}

	if newSeat.ScheduledTripID != currentSeat.ScheduledTripID {
		return nil, fmt.Errorf("new seat does not belong to the same trip")
	}

	if newSeat.Status != models.TripSeatStatusAvailable {
		return nil, fmt.Errorf("seat %s is no longer available (status: %s)", newSeat.SeatNumber, newSeat.Status)
	}

	// 4. Release old seat
	releaseOldSeatQuery := `
		UPDATE trip_seats
		SET status = 'available',
		    booking_type = NULL,
		    bus_booking_seat_id = NULL,
		    updated_at = $1
		WHERE id = $2 AND bus_booking_seat_id = $3
	`
	res, err := tx.Exec(releaseOldSeatQuery, now, currentTripSeatID, bookingSeat.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to release current seat: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("failed to release current seat: seat was not assigned to this booking")
	}

	// 5. Claim new seat
	claimNewSeatQuery := `
		UPDATE trip_seats
		SET status = 'booked',
		    booking_type = 'app',
		    bus_booking_seat_id = $1,
		    updated_at = $2
		WHERE id = $3 AND status = 'available'
	`
	res, err = tx.Exec(claimNewSeatQuery, bookingSeat.ID, now, newTripSeatID)
	if err != nil {
		return nil, fmt.Errorf("failed to claim new seat: %w", err)
	}
	rowsAffected, _ = res.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("new seat %s is no longer available", newSeat.SeatNumber)
	}

	// 6. Update bus_booking_seats record
	updateBookingSeatQuery := `
		UPDATE bus_booking_seats
		SET trip_seat_id = $1,
		    updated_at = $2
		WHERE id = $3
	`
	_, err = tx.Exec(updateBookingSeatQuery, newSeat.ID, now, bookingSeat.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update bus booking seat: %w", err)
	}

	// 7. Append audit log in bus_bookings.special_requests
	auditNote := fmt.Sprintf("[Seat Change %s] Moved from Seat %s to Seat %s", now.Format("2006-01-02 15:04"), currentSeat.SeatNumber, newSeat.SeatNumber)
	if reason != "" {
		auditNote += fmt.Sprintf(". Reason: %s", reason)
	}
	updateBookingAuditQuery := `
		UPDATE bus_bookings
		SET special_requests = CASE 
		    WHEN special_requests IS NULL OR special_requests = '' THEN $1
		    ELSE special_requests || E'\n' || $1
		END,
		updated_at = $2
		WHERE id = $3
	`
	_, _ = tx.Exec(updateBookingAuditQuery, auditNote, now, bookingSeat.BusBookingID)

	// 8. Fetch passenger notification target (player ID)
	fetchPlayerIDsQuery := `
		SELECT DISTINCT COALESCE(NULLIF(us.onesignal_player_id, ''), NULLIF(us.fcm_token, '')) as player_id
		FROM user_sessions us
		JOIN bookings b ON b.user_id::text = us.user_id::text
		JOIN bus_bookings bb ON bb.booking_id = b.id
		WHERE bb.id = $1
		  AND ((us.onesignal_player_id IS NOT NULL AND us.onesignal_player_id != '') OR (us.fcm_token IS NOT NULL AND us.fcm_token != ''))
		  AND us.is_active = true
	`
	var playerIDs []string
	_ = tx.Select(&playerIDs, fetchPlayerIDsQuery, bookingSeat.BusBookingID)

	// 9. Fetch route name for notification
	var routeName string
	fetchRouteQuery := `
		SELECT COALESCE(bor.custom_route_name, mr.route_name, 'your trip') as route_name
		FROM scheduled_trips st
		LEFT JOIN bus_owner_routes bor ON st.bus_owner_route_id = bor.id
		LEFT JOIN master_routes mr ON bor.master_route_id = mr.id
		WHERE st.id = $1
	`
	_ = tx.Get(&routeName, fetchRouteQuery, currentSeat.ScheduledTripID)

	// 10. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	newSeat.Status = models.TripSeatStatusBooked
	bookingTypeApp := models.TripSeatBookingTypeApp
	newSeat.BookingType = &bookingTypeApp
	newSeat.BusBookingSeatID = &bookingSeat.ID

	return &ChangeAppBookingSeatResult{
		NewSeat:         &newSeat,
		OldSeatNumber:   currentSeat.SeatNumber,
		NewSeatNumber:   newSeat.SeatNumber,
		PassengerName:   bookingSeat.PassengerName,
		PlayerIDs:       playerIDs,
		RouteName:       routeName,
		ScheduledTripID: currentSeat.ScheduledTripID,
	}, nil
}
