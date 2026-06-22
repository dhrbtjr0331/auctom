package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"auctom/go-api/internal/auth"
	"auctom/go-api/internal/handlers"
	"auctom/go-api/internal/sse"
)

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Fetch environment variables
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/auctom?sslmode=disable"
	}

	matchingURL := os.Getenv("MATCHING_SERVICE_URL")
	if matchingURL == "" {
		matchingURL = "http://localhost:9091"
	}

	quoteURL := os.Getenv("QUOTE_SERVICE_URL")
	if quoteURL == "" {
		quoteURL = "http://localhost:9092"
	}

	recommenderURL := os.Getenv("RECOMMENDER_SERVICE_URL")
	if recommenderURL == "" {
		recommenderURL = "http://localhost:9093"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	} else if port[0] != ':' {
		port = ":" + port
	}

	// Database Connection
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		slog.Error("Failed to open database connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Configure pool limit rules
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		slog.Warn("Failed to ping database on startup (DB might be offline)", "error", err)
	} else {
		slog.Info("Successfully connected to the database")
	}

	// Initialize SSE Broker
	broker := sse.NewBroker()
	go broker.Start()

	// Initialize Handlers
	h := &handlers.Handlers{
		DB:                    db,
		Broker:                broker,
		MatchingServiceURL:    matchingURL,
		QuoteServiceURL:       quoteURL,
		RecommenderServiceURL: recommenderURL,
	}

	// Setup Router
	r := chi.NewRouter()

	// Global Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(CORSMiddleware)

	// Public API Endpoints
	r.Post("/api/auth/login", h.Login)
	r.Post("/api/auth/register", h.Register)
	r.Post("/api/webhooks", h.WebhookDispatcher)
	r.Post("/api/webhooks/status-callback", h.StatusCallback)
	r.Handle("/api/sse", broker)

	// Protected API Endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware)

		r.Post("/api/rfqs", h.CreateRFQ)
		r.Get("/api/rfqs", h.GetRFQs)
		r.Get("/api/rfqs/{id}", h.GetRFQByID)
		r.Post("/api/rfqs/{id}/award", h.AwardRFQ)

		r.Post("/api/quotes/{id}/submit", h.SubmitQuote)

		r.Get("/api/supplier/profile", h.GetSupplierProfile)
		r.Put("/api/supplier/profile", h.UpdateSupplierProfile)
	})

	// Setup static files directory & fallback index.html if not present
	staticDir := "./static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		if err := os.MkdirAll(staticDir, 0755); err == nil {
			slog.Info("Created static directory", "path", staticDir)
			welcomeHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Auctom Procurement Platform</title>
</head>
<body>
    <h1>Welcome to Auctom Procurement Platform</h1>
    <p>Go API server is running and ready to serve.</p>
</body>
</html>`
			_ = os.WriteFile(staticDir+"/index.html", []byte(welcomeHTML), 0644)
		}
	}

	// Serve static files at root
	r.Handle("/*", http.FileServer(http.Dir(staticDir)))

	slog.Info("Starting server", "port", port)
	if err := http.ListenAndServe(port, r); err != nil {
		slog.Error("Server crashed", "error", err)
		os.Exit(1)
	}
}
