package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agam-vs/go-transaction-service/internal/domain"
)

type stubAccountReader struct {
	account domain.Account
	calls   int
}

func (s *stubAccountReader) GetByID(
	ctx context.Context,
	id int64,
) (domain.Account, error) {
	s.calls++

	return s.account, nil
}

func TestAccountRouteAPIKeyProtection(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		wantStatus int
		wantCalls  int
	}{
		{
			name:       "valid API key",
			apiKey:     "local-test-secret",
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name:       "missing API key",
			apiKey:     "",
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
			reader := &stubAccountReader{
				account: domain.Account{
					ID:       7,
					Number:   "10000007",
					Currency: "IDR",
					Balance:  500_000,
				},
			}

			router := newAppRouter(
				reader,
				"local-test-secret",
			)

			request := httptest.NewRequest(
				http.MethodGet,
				"/v1/accounts/7",
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
		})
	}
}

func TestAccountRouteRejectsMissingServerAPIKey(t *testing.T) {
	reader := &stubAccountReader{}

	router := newAppRouter(reader, "")

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/7",
		nil,
	)

	request.Header.Set(
		"X-API-Key",
		"some-key",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code == http.StatusOK {
		t.Error("route must not allow access without server API key")
	}

	if reader.calls != 0 {
		t.Error("repository must not be called")
	}
}

func TestAppRouterHealthCheck(t *testing.T) {
	reader := &stubAccountReader{}

	router := newAppRouter(
		reader,
		"local-test-secret",
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/healthz",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf(
			"status = %d, want 200",
			recorder.Code,
		)
	}
}

func TestAccountRouteRejectsDuplicateAPIKeys(t *testing.T) {
	reader := &stubAccountReader{}

	router := newAppRouter(
		reader,
		"local-test-secret",
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/7",
		nil,
	)

	request.Header.Add(
		"X-API-Key",
		"local-test-secret",
	)

	request.Header.Add(
		"X-API-Key",
		"local-test-secret",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf(
			"status = %d, want 401",
			recorder.Code,
		)
	}

	if reader.calls != 0 {
		t.Error("repository must not be called")
	}
}
