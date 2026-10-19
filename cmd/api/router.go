package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/agam-vs/go-transaction-service/internal/httpapi"
)

// newAppRouter configures the application HTTP routes.
func newAppRouter(
	accountReader httpapi.AccountReader,
	transferExecutor httpapi.TransferExecutor,
	apiKey string,
) http.Handler {
	mux := http.NewServeMux()

	// Public health check.
	mux.Handle(
		"GET /healthz",
		newRouter(),
	)

	// Protected account details endpoint.
	mux.Handle(
		"GET /v1/accounts/{id}",
		apiKeyMiddleware(
			apiKey,
			httpapi.NewAccountHandler(accountReader),
		),
	)

	// Protected transfer creation endpoint.
	mux.Handle(
		"POST /v1/transfers",
		apiKeyMiddleware(
			apiKey,
			httpapi.NewTransferHandler(transferExecutor),
		),
	)

	return mux
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
