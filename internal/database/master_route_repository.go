package database

import (
	"database/sql"
	"regexp"
	"strings"

	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// MasterRouteRepository handles database operations for master_routes table
type MasterRouteRepository struct {
	db DB
}

// NewMasterRouteRepository creates a new MasterRouteRepository
func NewMasterRouteRepository(db DB) *MasterRouteRepository {
	return &MasterRouteRepository{db: db}
}

// GetByID retrieves a master route by ID
func (r *MasterRouteRepository) GetByID(routeID string) (*models.MasterRoute, error) {
	query := `
		SELECT id, route_number, route_name, origin_city, destination_city,
			   total_distance_km, estimated_duration_minutes, encoded_polyline,
			   is_active, created_at, updated_at
		FROM master_routes
		WHERE id = $1
	`

	route := &models.MasterRoute{}
	var totalDistanceKm sql.NullFloat64
	var estimatedDurationMinutes sql.NullInt64
	var encodedPolyline sql.NullString

	err := r.db.QueryRow(query, routeID).Scan(
		&route.ID, &route.RouteNumber, &route.RouteName, &route.OriginCity, &route.DestinationCity,
		&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
		&route.IsActive, &route.CreatedAt, &route.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if totalDistanceKm.Valid {
		route.TotalDistanceKm = &totalDistanceKm.Float64
	}
	if estimatedDurationMinutes.Valid {
		minutes := int(estimatedDurationMinutes.Int64)
		route.EstimatedDurationMinutes = &minutes
	}
	if encodedPolyline.Valid {
		route.EncodedPolyline = &encodedPolyline.String
	}

	return route, nil
}

// GetByRouteNumber retrieves a master route by route number
func (r *MasterRouteRepository) GetByRouteNumber(routeNumber string) (*models.MasterRoute, error) {
	query := `
		SELECT id, route_number, route_name, origin_city, destination_city,
			   total_distance_km, estimated_duration_minutes, encoded_polyline,
			   is_active, created_at, updated_at
		FROM master_routes
		WHERE route_number = $1
	`

	route := &models.MasterRoute{}
	var totalDistanceKm sql.NullFloat64
	var estimatedDurationMinutes sql.NullInt64
	var encodedPolyline sql.NullString

	err := r.db.QueryRow(query, routeNumber).Scan(
		&route.ID, &route.RouteNumber, &route.RouteName, &route.OriginCity, &route.DestinationCity,
		&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
		&route.IsActive, &route.CreatedAt, &route.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if totalDistanceKm.Valid {
		route.TotalDistanceKm = &totalDistanceKm.Float64
	}
	if estimatedDurationMinutes.Valid {
		minutes := int(estimatedDurationMinutes.Int64)
		route.EstimatedDurationMinutes = &minutes
	}
	if encodedPolyline.Valid {
		route.EncodedPolyline = &encodedPolyline.String
	}

	return route, nil
}

// GetAll retrieves all master routes
func (r *MasterRouteRepository) GetAll(activeOnly bool) ([]models.MasterRoute, error) {
	query := `
		SELECT id, route_number, route_name, origin_city, destination_city,
			   total_distance_km, estimated_duration_minutes, encoded_polyline,
			   is_active, created_at, updated_at
		FROM master_routes
	`

	if activeOnly {
		query += " WHERE is_active = true"
	}

	query += " ORDER BY route_number"

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := []models.MasterRoute{}
	for rows.Next() {
		var route models.MasterRoute
		var totalDistanceKm sql.NullFloat64
		var estimatedDurationMinutes sql.NullInt64
		var encodedPolyline sql.NullString

		err := rows.Scan(
			&route.ID, &route.RouteNumber, &route.RouteName, &route.OriginCity, &route.DestinationCity,
			&totalDistanceKm, &estimatedDurationMinutes, &encodedPolyline,
			&route.IsActive, &route.CreatedAt, &route.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if totalDistanceKm.Valid {
			route.TotalDistanceKm = &totalDistanceKm.Float64
		}
		if estimatedDurationMinutes.Valid {
			minutes := int(estimatedDurationMinutes.Int64)
			route.EstimatedDurationMinutes = &minutes
		}
		if encodedPolyline.Valid {
			route.EncodedPolyline = &encodedPolyline.String
		}

		routes = append(routes, route)
	}

	return routes, rows.Err()
}

// GetStopsByRouteID retrieves all stops for a master route
func (r *MasterRouteRepository) GetStopsByRouteID(routeID string) ([]models.MasterRouteStop, error) {
	query := `
		SELECT id, master_route_id, stop_name, stop_order,
			   latitude, longitude, arrival_time_offset_minutes,
			   is_major_stop, created_at
		FROM master_route_stops
		WHERE master_route_id = $1
		ORDER BY stop_order
	`

	rows, err := r.db.Query(query, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stops := []models.MasterRouteStop{}
	for rows.Next() {
		var stop models.MasterRouteStop
		var latitude sql.NullFloat64
		var longitude sql.NullFloat64
		var arrivalTimeOffsetMinutes sql.NullInt64

		err := rows.Scan(
			&stop.ID, &stop.MasterRouteID, &stop.StopName, &stop.StopOrder,
			&latitude, &longitude, &arrivalTimeOffsetMinutes,
			&stop.IsMajorStop, &stop.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.Null* types
		if latitude.Valid {
			stop.Latitude = &latitude.Float64
		}
		if longitude.Valid {
			stop.Longitude = &longitude.Float64
		}
		if arrivalTimeOffsetMinutes.Valid {
			minutes := int(arrivalTimeOffsetMinutes.Int64)
			stop.ArrivalTimeOffsetMinutes = &minutes
		}

		stops = append(stops, stop)
	}

	return stops, rows.Err()
}

// GetStopByID retrieves a specific stop by ID
func (r *MasterRouteRepository) GetStopByID(stopID string) (*models.MasterRouteStop, error) {
	query := `
		SELECT id, master_route_id, stop_name, stop_order,
			   latitude, longitude, arrival_time_offset_minutes,
			   is_major_stop, created_at
		FROM master_route_stops
		WHERE id = $1
	`

	stop := &models.MasterRouteStop{}
	var latitude sql.NullFloat64
	var longitude sql.NullFloat64
	var arrivalTimeOffsetMinutes sql.NullInt64

	err := r.db.QueryRow(query, stopID).Scan(
		&stop.ID, &stop.MasterRouteID, &stop.StopName, &stop.StopOrder,
		&latitude, &longitude, &arrivalTimeOffsetMinutes,
		&stop.IsMajorStop, &stop.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert sql.Null* types
	if latitude.Valid {
		stop.Latitude = &latitude.Float64
	}
	if longitude.Valid {
		stop.Longitude = &longitude.Float64
	}
	if arrivalTimeOffsetMinutes.Valid {
		minutes := int(arrivalTimeOffsetMinutes.Int64)
		stop.ArrivalTimeOffsetMinutes = &minutes
	}

	return stop, nil
}

// extractBaseRouteNumber extracts the route code without direction suffix.
// Examples:
// "17 kan-pana" -> "17"
// "17 panadu-kan" -> "17"
// "1/col-kandy" -> "1"
// "1/kandy-col" -> "1"
// "EX - 03/kandy-mahara" -> "ex03"
func extractBaseRouteNumber(routeNum string) string {
	s := strings.TrimSpace(routeNum)
	if idx := strings.Index(s, "/"); idx != -1 {
		prefix := strings.TrimSpace(s[:idx])
		if prefix != "" {
			return cleanRouteNumber(prefix)
		}
	}
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		return cleanRouteNumber(parts[0])
	}
	return cleanRouteNumber(s)
}

func cleanRouteNumber(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	return strings.ToLower(reg.ReplaceAllString(s, ""))
}

func extractVia(routeName string) string {
	reg := regexp.MustCompile(`(?i)\(via\s+([^)]+)\)`)
	matches := reg.FindStringSubmatch(routeName)
	if len(matches) > 1 {
		clean := regexp.MustCompile(`[^a-zA-Z0-9]`)
		return strings.ToLower(clean.ReplaceAllString(matches[1], ""))
	}
	return ""
}

// AreRoutesPaired checks if two master routes are identical or are reverse directions of the same corridor.
func (r *MasterRouteRepository) AreRoutesPaired(routeID1, routeID2 string) (bool, error) {
	if routeID1 == "" || routeID2 == "" {
		return false, nil
	}
	if routeID1 == routeID2 {
		return true, nil
	}

	query := `
		SELECT 
			r1.route_number, r1.origin_city, r1.destination_city, r1.route_name,
			r2.route_number, r2.origin_city, r2.destination_city, r2.route_name
		FROM master_routes r1, master_routes r2
		WHERE r1.id = $1 AND r2.id = $2
	`
	var r1Num, r1Origin, r1Dest, r1Name string
	var r2Num, r2Origin, r2Dest, r2Name string

	err := r.db.QueryRow(query, routeID1, routeID2).Scan(
		&r1Num, &r1Origin, &r1Dest, &r1Name,
		&r2Num, &r2Origin, &r2Dest, &r2Name,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	o1 := strings.ToLower(strings.TrimSpace(r1Origin))
	d1 := strings.ToLower(strings.TrimSpace(r1Dest))
	o2 := strings.ToLower(strings.TrimSpace(r2Origin))
	d2 := strings.ToLower(strings.TrimSpace(r2Dest))

	// Cities must be swapped
	if o1 == d2 && d1 == o2 {
		// If both route names specify a via path (e.g. "(via Puttalam)" vs "(via Kurunegala)"), they must match
		via1 := extractVia(r1Name)
		via2 := extractVia(r2Name)
		if via1 != "" && via2 != "" && via1 != via2 {
			return false, nil
		}

		// Exact route number match
		if strings.EqualFold(strings.TrimSpace(r1Num), strings.TrimSpace(r2Num)) {
			return true, nil
		}
		// Base route number match (e.g. "17 kan-pana" vs "17 panadu-kan")
		b1 := extractBaseRouteNumber(r1Num)
		b2 := extractBaseRouteNumber(r2Num)
		if b1 != "" && b1 == b2 {
			return true, nil
		}
		if b1 == "" || b2 == "" {
			return true, nil
		}
	}

	return false, nil
}
