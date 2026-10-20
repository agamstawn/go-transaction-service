package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agam-vs/go-transaction-service/internal/domain"
	"github.com/agam-vs/go-transaction-service/internal/service"
)

func TestProtectedRoutesSecurityHeaders(t *testing.T) {
	reader := &stubAccountReader{
		account: domain.Account{
			ID:       7,
			Number:   "10000007",
			Currency: "IDR",
			Balance:  500_000,
		},
	}

	executor := &stubTransferExecutor{
		result: service.TransferResult{
			ID:            100,
			FromAccountID: 7,
			ToAccountID:   8,
			Amount:        125_000,
			Status:        "completed",
		},
	}

	router := newAppRouter(
		reader,
		executor,
		&stubTransferReader{},
		&routerHistoryReaderStub{},
		"local-test-secret",
		7,
	)

	tests := []struct {
		name       string
		method     string
		path       string
		apiKey     string
		body       string
		wantStatus int
	}{
		{
			name:       "authorized account",
			method:     http.MethodGet,
			path:       "/v1/accounts/7",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusOK,
		},
		{
			name:       "forbidden account",
			method:     http.MethodGet,
			path:       "/v1/accounts/8",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unauthenticated account",
			method:     http.MethodGet,
			path:       "/v1/accounts/7",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:   "authorized transfer",
			method: http.MethodPost,
			path:   "/v1/transfers",
			apiKey: "local-test-secret",
			body: `{
				"from_account_id": 7,
				"to_account_id": 8,
				"amount": 125000
			}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:   "unauthenticated transfer",
			method: http.MethodPost,
			path:   "/v1/transfers",
			body: `{
				"from_account_id": 7,
				"to_account_id": 8,
				"amount": 125000
			}`,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				tt.method,
				tt.path,
				strings.NewReader(tt.body),
			)

			if tt.apiKey != "" {
				request.Header.Set(
					"X-API-Key",
					tt.apiKey,
				)
			}

			if tt.method == http.MethodPost {
				request.Header.Set(
					"Content-Type",
					"application/json",
				)

				request.Header.Set(
					"Idempotency-Key",
					"security-test-001",
				)
			}

			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d",
					recorder.Code,
					tt.wantStatus,
				)
			}

			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf(
					"Cache-Control = %q, want no-store",
					got,
				)
			}

			if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf(
					"X-Content-Type-Options = %q, want nosniff",
					got,
				)
			}
		})
	}
}
