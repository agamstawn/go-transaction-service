package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/agam-vs/go-transaction-service/internal/postgres"
	"github.com/agam-vs/go-transaction-service/internal/service"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// newRouter preserves the original health-check handler.
func newRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return mux
}

func main() {
	// 1. Load and validate application configuration.
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	// 2. Initialize the PostgreSQL connection pool.
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// 3. Configure connection pooling.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// 4. Verify database connectivity before starting.
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	err = db.PingContext(ctx)
	cancel()

	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	log.Println("PostgreSQL connection established")

	// 5. Initialize application dependencies.
	accountRepository := postgres.NewAccountRepository(db)

	// Initialize PostgreSQL transfer processor.
	transferProcessor := postgres.NewTransferProcessor(db)

	// Inject the processor into the transfer service.
	transferService := service.NewTransferService(
		transferProcessor,
	)

	// 6. Configure HTTP routes and authentication.
	router := newAppRouter(
		accountRepository,
		transferService,
		cfg.APIKey,
	)

	// 7. Initialize the HTTP server.
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 8. Start accepting HTTP requests.
	log.Printf(
		"API listening on http://%s",
		cfg.ListenAddress,
	)

	log.Fatal(server.ListenAndServe())
}
