// Command migrate applies the database schema. Run it as a deploy step against
// a direct (or session-mode) connection, e.g. `go run ./cmd/migrate`.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"myapp/internal/store"
)

func main() {
	url := os.Getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		log.Fatal("set MIGRATE_DATABASE_URL (or DATABASE_URL)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	db, err := store.Open(ctx, url)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations applied")
}
