package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/munster-bunkum/bunkum-api/internal/db"
	"github.com/munster-bunkum/bunkum-api/internal/handlers"
	"github.com/munster-bunkum/bunkum-api/internal/auth"
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
	mux.HandleFunc("POST /api/v1/auth/logout", handlers.Logout)
	mux.HandleFunc("GET /api/v1/me", auth.Middleware(handlers.Me(pool)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	origin := os.Getenv("ALLOWED_ORIGIN")
	if origin == "" {
		origin = "*"
	}

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(origin, mux)))
}
