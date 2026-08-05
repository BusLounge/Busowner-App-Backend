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

	// Query the most recently added bus
	row := db.QueryRow(`
		SELECT id, bus_number, license_plate, permit_id, seat_layout_id, created_at
		FROM buses
		ORDER BY created_at DESC
		LIMIT 1
	`)

	var id string
	var busNumber string
	var licensePlate string
	var permitID sql.NullString
	var seatLayoutID sql.NullString
	var createdAt string

	err = row.Scan(&id, &busNumber, &licensePlate, &permitID, &seatLayoutID, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No buses found.")
			return
		}
		log.Fatal(err)
	}

	pID := "NULL"
	if permitID.Valid {
		pID = permitID.String
	}

	sLayoutID := "NULL"
	if seatLayoutID.Valid {
		sLayoutID = seatLayoutID.String
	}

	fmt.Println("Most Recently Created Bus:")
	fmt.Printf("- ID: %s\n- Number: %s\n- Plate: %s\n- Permit: %s\n- SeatLayout: %s\n- CreatedAt: %s\n",
		id, busNumber, licensePlate, pID, sLayoutID, createdAt)
}
