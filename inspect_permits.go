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
		SELECT column_name, data_type, is_nullable 
		FROM information_schema.columns 
		WHERE table_name = 'master_route_stops'
		ORDER BY ordinal_position
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Columns in master_route_stops:")
	for rows.Next() {
		var colName, dataType, isNullable string
		if err := rows.Scan(&colName, &dataType, &isNullable); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- %s: %s (nullable: %s)\n", colName, dataType, isNullable)
	}
}
