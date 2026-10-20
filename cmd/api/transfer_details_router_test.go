package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agam-vs/go-transaction-service/internal/service"
)

type stubTransferReader struct {
	result            service.TransferResult
	err               error
	calls             int
	receivedID        int64
	receivedAccountID int64
}

func (s *stubTransferReader) GetByID(
	_ context.Context,
	transferID int64,
	authorizedAccountID int64,
) (service.TransferResult, error) {
	s.calls++
	s.receivedID = transferID
	s.receivedAccountID = authorizedAccountID

	return s.result, s.err
}

func TestTransferDetailsRoute(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		apiKey     string
		readerErr  error
		wantStatus int
		wantCalls  int
	}{
		{
			name:       "authorized transfer",
			path:       "/v1/transfers/100",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name:       "missing API key",
			path:       "/v1/transfers/100",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
		{
			name:       "invalid API key",
			path:       "/v1/transfers/100",
			apiKey:     "wrong-secret",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
		{
			name:       "transfer not visible",
			path:       "/v1/transfers/100",
			apiKey:     "local-test-secret",
			readerErr:  sql.ErrNoRows,
			wantStatus: http.StatusNotFound,
			wantCalls:  1,
		},
		{
			name:       "invalid transfer ID",
			path:       "/v1/transfers/abc",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &stubTransferReader{
				result: service.TransferResult{
					ID:            100,
					FromAccountID: 7,
					ToAccountID:   8,
					Amount:        125_000,
					Status:        "completed",
				},
				err: tt.readerErr,
			}

			router := newAppRouter(
				&stubAccountReader{},
				&stubTransferExecutor{},
				reader,
				"local-test-secret",
				7,
			)

			request := httptest.NewRequest(
				http.MethodGet,
				tt.path,
				nil,
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

			if reader.calls != tt.wantCalls {
				t.Errorf(
					"repository calls = %d, want %d",
					reader.calls,
					tt.wantCalls,
				)
			}

			if tt.wantCalls == 1 {
				if reader.receivedID != 100 {
					t.Errorf(
						"transfer ID = %d, want 100",
						reader.receivedID,
					)
				}

				if reader.receivedAccountID != 7 {
					t.Errorf(
						"account scope = %d, want 7",
						reader.receivedAccountID,
					)
				}
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
