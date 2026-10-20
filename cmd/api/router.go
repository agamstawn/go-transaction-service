package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/agam-vs/go-transaction-service/internal/domain"
	"github.com/agam-vs/go-transaction-service/internal/httpapi"
	"github.com/agam-vs/go-transaction-service/internal/service"
)

// newAppRouter configures the application HTTP routes.
func newAppRouter(
	accountReader httpapi.AccountReader,
	transferExecutor httpapi.TransferExecutor,
	apiKey string,
	authorizedAccountID int64,
) http.Handler {

	mux := http.NewServeMux()

	// Public health endpoint.
	mux.Handle(
		"GET /healthz",
		newRouter(),
	)

	// Apply account authorization.
	protectedAccountReader := scopedAccountReader{
		reader:              accountReader,
		authorizedAccountID: authorizedAccountID,
	}

	// Apply transfer authorization.
	protectedTransferExecutor := scopedTransferExecutor{
		executor:            transferExecutor,
		authorizedAccountID: authorizedAccountID,
	}

	// Protected account endpoint.
	mux.Handle(
		"GET /v1/accounts/{id}",
		apiKeyMiddleware(
			apiKey,
			httpapi.NewAccountHandler(protectedAccountReader),
		),
	)

	// Protected transfer endpoint.
	mux.Handle(
		"POST /v1/transfers",
		apiKeyMiddleware(
			apiKey,
			httpapi.NewTransferHandler(protectedTransferExecutor),
		),
	)

	return mux
}

// scopedAccountReader restricts account access to the configured account.
type scopedAccountReader struct {
	reader              httpapi.AccountReader
	authorizedAccountID int64
}

func (s scopedAccountReader) GetByID(
	ctx context.Context,
	id int64,
) (domain.Account, error) {

	if s.authorizedAccountID <= 0 ||
		id != s.authorizedAccountID {
		return domain.Account{}, domain.ErrAccountForbidden
	}

	return s.reader.GetByID(ctx, id)
}

// apiKeyMiddleware protects routes that require an API key.
func apiKeyMiddleware(
	expectedKey string,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Fail closed when the server is not configured.
			if expectedKey == "" {
				writeAuthError(
					w,
					http.StatusServiceUnavailable,
					"service unavailable",
				)
				return
			}

			keys := r.Header.Values("X-API-Key")

			// Reject missing or duplicate API key headers.
			if len(keys) != 1 {
				writeAuthError(
					w,
					http.StatusUnauthorized,
					"unauthorized",
				)
				return
			}

			// Compare API keys without ordinary string comparison.
			if subtle.ConstantTimeCompare(
				[]byte(keys[0]),
				[]byte(expectedKey),
			) != 1 {
				writeAuthError(
					w,
					http.StatusUnauthorized,
					"unauthorized",
				)
				return
			}

			// Authentication passed. Continue to the handler.
			next.ServeHTTP(w, r)
		},
	)
}

// writeAuthError returns a JSON authentication error.
func writeAuthError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(
		map[string]string{
			"error": message,
		},
	)
}

// scopedTransferExecutor restricts the source account of a transfer.
type scopedTransferExecutor struct {
	executor            httpapi.TransferExecutor
	authorizedAccountID int64
}

func (s scopedTransferExecutor) Transfer(
	ctx context.Context,
	input domain.TransferInput,
) (service.TransferResult, error) {

	if s.authorizedAccountID <= 0 ||
		input.FromAccountID != s.authorizedAccountID {
		return service.TransferResult{}, domain.ErrAccountForbidden
	}

	return s.executor.Transfer(ctx, input)
}
