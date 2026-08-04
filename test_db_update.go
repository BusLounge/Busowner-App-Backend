package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:5432/postgres")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Let's find a trip to test
	var tripID string
	var currentFare float64
	var currentStatus string
	var currentDeparture string
	
	err = db.QueryRow("SELECT id, base_fare, status, departure_datetime FROM scheduled_trips LIMIT 1").Scan(&tripID, &currentFare, &currentStatus, &currentDeparture)
	if err != nil {
		log.Fatalf("Error finding test trip: %v", err)
	}
	
	fmt.Printf("Testing on Trip ID: %s\n- Current Fare: %.2f\n- Current Status: %s\n- Current Departure: %s\n", 
		tripID, currentFare, currentStatus, currentDeparture)

	// Attempt update
	newFare := currentFare + 5.50
	newStatus := "confirmed"
	newDeparture := "2026-08-20T12:00:00Z"
	
	fmt.Printf("Updating via SQL...\n")
	query := `
		UPDATE scheduled_trips
		SET base_fare = $2, status = $3, departure_datetime = $4, updated_at = NOW()
		WHERE id = $1
	`
	res, err := db.Exec(query, tripID, newFare, newStatus, newDeparture)
	if err != nil {
		log.Fatalf("Error executing update: %v", err)
	}
	
	affected, err := res.RowsAffected()
	if err != nil {
		log.Fatalf("Error getting rows affected: %v", err)
	}
	fmt.Printf("Rows affected: %d\n", affected)

	// Fetch again to verify
	var updatedFare float64
	var updatedStatus string
	var updatedDeparture string
	err = db.QueryRow("SELECT base_fare, status, departure_datetime FROM scheduled_trips WHERE id = $1", tripID).Scan(&updatedFare, &updatedStatus, &updatedDeparture)
	if err != nil {
		log.Fatalf("Error fetching updated trip: %v", err)
	}
	
	fmt.Printf("Verification:\n- Updated Fare: %.2f (expected: %.2f)\n- Updated Status: %s (expected: %s)\n- Updated Departure: %s (expected: %s)\n", 
		updatedFare, newFare, updatedStatus, newStatus, updatedDeparture, newDeparture)
		
	// Revert the changes
	_, err = db.Exec(query, tripID, currentFare, currentStatus, currentDeparture)
	if err != nil {
		log.Printf("Warning: Failed to revert changes: %v", err)
	} else {
		fmt.Println("Changes reverted successfully!")
	}
}
