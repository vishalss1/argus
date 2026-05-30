package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load(".env")
	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	id := "158a11ab-0011-47ac-b3a3-ef064816bf78"
	var status string
	err = db.QueryRow("SELECT status FROM commands WHERE id = $1", id).Scan(&status)
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}

	fmt.Printf("Command %s status in DB: %s\n", id, status)
}
