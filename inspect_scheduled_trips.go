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

	rows, err := db.Query(`
		SELECT id, departure_datetime, base_fare, status, permit_id
		FROM scheduled_trips
		ORDER BY departure_datetime DESC
		LIMIT 10
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Recent 10 Scheduled Trips in DB:")
	for rows.Next() {
		var id string
		var departureDatetime string
		var baseFare float64
		var status string
		var permitID sql.NullString
		
		if err := rows.Scan(&id, &departureDatetime, &baseFare, &status, &permitID); err != nil {
			log.Fatal(err)
		}
		
		permit := "NULL"
		if permitID.Valid {
			permit = permitID.String
		}
		
		fmt.Printf("- ID: %s | Date: %s | Fare: %.2f | Status: %s | Permit: %s\n", 
			id, departureDatetime, baseFare, status, permit)
	}
}
