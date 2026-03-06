package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/munster-bunkum/bunkum-api/internal/db"
	"github.com/munster-bunkum/bunkum-api/internal/handlers"
)

func main() {
	// Loads .env if present. In production Kamal injects env vars directly,
	// so this is silently skipped — no harm either way.
	_ = godotenv.Load()

	pool, err := db.Connect(context.Background())
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", handlers.Health)
	mux.HandleFunc("GET /up", handlers.Health)

	mux.HandleFunc("POST /api/v1/auth/register", handlers.Register(pool))
	mux.HandleFunc("POST /api/v1/auth/login", handlers.Login(pool))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
