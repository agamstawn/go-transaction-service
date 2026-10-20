//go:build integration

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agamstawn/go-transaction-service/internal/postgres"
	"github.com/agamstawn/go-transaction-service/internal/service"
)

func TestRouterPostgresE2EHarness(t *testing.T) {
	// 1. Connect to the isolated PostgreSQL test database.
	db := openE2EDatabase(t)

	// 2. Initialize real application dependencies.
	accountRepository := postgres.NewAccountRepository(db)

	transferProcessor := postgres.NewTransferProcessor(db)

	transferService := service.NewTransferService(
		transferProcessor,
	)

	transferRepository := postgres.NewTransferRepository(db)

	// 3. Build the real application router.
	router := newAppRouter(
		accountRepository,
		transferService,
		transferRepository,
		transferRepository,
		"e2e-test-secret",
		7,
	)

	// 4. Start a local HTTP test server.
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	t.Run("unauthenticated request rejected", func(t *testing.T) {
		response, err := client.Get(
			server.URL + "/v1/transfers",
		)
		if err != nil {
			t.Fatalf("HTTP request: %v", err)
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf(
				"status = %d, want 401",
				response.StatusCode,
			)
		}
	})

	t.Run("authenticated history request", func(t *testing.T) {
		request, err := http.NewRequest(
			http.MethodGet,
			server.URL+"/v1/transfers?page=1&per_page=10",
			nil,
		)
		if err != nil {
			t.Fatalf("create request: %v", err)
		}

		request.Header.Set(
			"X-API-Key",
			"e2e-test-secret",
		)

		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("HTTP request: %v", err)
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(
				io.LimitReader(response.Body, 2048),
			)

			t.Fatalf(
				"status = %d, want 200; body = %s",
				response.StatusCode,
				body,
			)
		}

		if got := response.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf(
				"Cache-Control = %q, want no-store",
				got,
			)
		}

		if got := response.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf(
				"X-Content-Type-Options = %q, want nosniff",
				got,
			)
		}
	})
}
