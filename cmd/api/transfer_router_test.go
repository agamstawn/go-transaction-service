package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agam-vs/go-transaction-service/internal/domain"
	"github.com/agam-vs/go-transaction-service/internal/service"
)

type stubTransferExecutor struct {
	calls         int
	receivedInput domain.TransferInput
	result        service.TransferResult
}

func (s *stubTransferExecutor) Transfer(
	_ context.Context,
	input domain.TransferInput,
) (service.TransferResult, error) {
	s.calls++
	s.receivedInput = input

	return s.result, nil
}

func TestTransferRouteAPIKeyProtection(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		replayed   bool
		wantStatus int
		wantCalls  int
	}{
		{
			name:       "new transfer",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusCreated,
			wantCalls:  1,
		},
		{
			name:       "idempotent replay",
			apiKey:     "local-test-secret",
			replayed:   true,
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name:       "missing API key",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
		{
			name:       "invalid API key",
			apiKey:     "wrong-secret",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &stubTransferExecutor{
				result: service.TransferResult{
					ID:            100,
					FromAccountID: 1,
					ToAccountID:   2,
					Amount:        125_000,
					Status:        "completed",
					Replayed:      tt.replayed,
				},
			}

			router := newAppRouter(
				&stubAccountReader{},
				executor,
				"local-test-secret",
			)

			body := `{
				"from_account_id": 1,
				"to_account_id": 2,
				"amount": 125000
			}`

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/transfers",
				strings.NewReader(body),
			)

			request.Header.Set(
				"Content-Type",
				"application/json",
			)

			request.Header.Set(
				"Idempotency-Key",
				"router-test-001",
			)

			if tt.apiKey != "" {
				request.Header.Set(
					"X-API-Key",
					tt.apiKey,
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

			if executor.calls != tt.wantCalls {
				t.Errorf(
					"service calls = %d, want %d",
					executor.calls,
					tt.wantCalls,
				)
			}

			if tt.wantCalls == 1 {
				if executor.receivedInput.IdempotencyKey != "router-test-001" {
					t.Error("idempotency key not forwarded")
				}
			}

			if tt.replayed {
				if recorder.Header().Get("Idempotent-Replayed") != "true" {
					t.Error("missing replay response header")
				}
			}
		})
	}
}
