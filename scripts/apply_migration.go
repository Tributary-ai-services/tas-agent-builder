package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("Applying database migration for retry/fallback support...")

	// Connect to database
	dsn := "host=localhost port=5432 user=tasuser password=taspassword dbname=tas_shared sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✅ Connected to database")

	// Read migration file
	migrationSQL, err := ioutil.ReadFile("database/migrations/002_add_retry_fallback_support.sql")
	if err != nil {
		log.Fatalf("Failed to read migration file: %v", err)
	}

	// Execute migration
	fmt.Println("Executing migration...")
	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	fmt.Println("✅ Migration applied successfully!")
	fmt.Println("🎉 Database is ready for enhanced reliability features!")
}