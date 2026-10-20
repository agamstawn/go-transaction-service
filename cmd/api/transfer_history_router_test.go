package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agamstawn/go-transaction-service/internal/service"
)

// routerHistoryReaderStub records calls made by the HTTP router.
type routerHistoryReaderStub struct {
	calls     int
	accountID int64
	page      int
	perPage   int
	err       error
}

func (s *routerHistoryReaderStub) ListByAccount(
	_ context.Context,
	authorizedAccountID int64,
	page int,
	perPage int,
) (service.TransferPage, error) {
	s.calls++
	s.accountID = authorizedAccountID
	s.page = page
	s.perPage = perPage

	if s.err != nil {
		return service.TransferPage{}, s.err
	}

	return service.TransferPage{
		Data: []service.TransferResult{
			{
				ID:            100,
				FromAccountID: 7,
				ToAccountID:   8,
				Amount:        125_000,
				Status:        "completed",
			},
		},
		Page:       page,
		PerPage:    perPage,
		Total:      1,
		TotalPages: 1,
	}, nil
}

func TestTransferHistoryRoute(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		apiKey     string
		readerErr  error
		wantStatus int
		wantCalls  int
	}{
		{
			name:       "authorized history",
			path:       "/v1/transfers?page=2&per_page=3&account_id=999",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name:       "missing API key",
			path:       "/v1/transfers",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
		{
			name:       "invalid API key",
			path:       "/v1/transfers",
			apiKey:     "wrong-secret",
			wantStatus: http.StatusUnauthorized,
			wantCalls:  0,
		},
		{
			name:       "invalid pagination",
			path:       "/v1/transfers?page=0",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "repository failure",
			path:       "/v1/transfers",
			apiKey:     "local-test-secret",
			readerErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			historyReader := &routerHistoryReaderStub{
				err: tt.readerErr,
			}

			router := newAppRouter(
				&stubAccountReader{},
				&stubTransferExecutor{},
				&stubTransferReader{},
				historyReader,
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

			// Verify HTTP status.
			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body = %s",
					recorder.Code,
					tt.wantStatus,
					recorder.Body.String(),
				)
			}

			// Authentication and validation must prevent
			// unnecessary repository calls.
			if historyReader.calls != tt.wantCalls {
				t.Errorf(
					"repository calls = %d, want %d",
					historyReader.calls,
					tt.wantCalls,
				)
			}

			// Verify the trusted server-side account scope.
			if tt.name == "authorized history" {
				if historyReader.accountID != 7 {
					t.Errorf(
						"account scope = %d, want 7",
						historyReader.accountID,
					)
				}

				if historyReader.page != 2 ||
					historyReader.perPage != 3 {
					t.Errorf(
						"pagination = (%d, %d), want (2, 3)",
						historyReader.page,
						historyReader.perPage,
					)
				}
			}

			// Protected endpoints must not be cached.
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
