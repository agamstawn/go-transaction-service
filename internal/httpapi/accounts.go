package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/agamstawn/go-transaction-service/internal/domain"
)

// AccountReader defines the behavior required to retrieve accounts.
type AccountReader interface {
	GetByID(
		ctx context.Context,
		id int64,
	) (domain.Account, error)
}

// AccountHandler handles account-related HTTP requests.
type AccountHandler struct {
	repository AccountReader
}

// NewAccountHandler creates an account HTTP handler.
func NewAccountHandler(repository AccountReader) http.Handler {
	return &AccountHandler{
		repository: repository,
	}
}

// ServeHTTP handles GET /v1/accounts/{id}.
func (h *AccountHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	// Parse account ID.
	id, err := strconv.ParseInt(
		r.PathValue("id"),
		10,
		64,
	)

	if err != nil || id <= 0 {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid account ID",
		)
		return
	}

	// Retrieve account.
	account, err := h.repository.GetByID(
		r.Context(),
		id,
	)

	if err != nil {
		// Authorization failure.
		if errors.Is(err, domain.ErrAccountForbidden) {
			writeJSONError(
				w,
				http.StatusForbidden,
				"account access forbidden",
			)
			return
		}

		// Account not found.
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(
				w,
				http.StatusNotFound,
				"account not found",
			)
			return
		}

		// Unexpected internal error.
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	// Successful response.
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(account)
}

// writeJSONError returns a standardized JSON error response.
func writeJSONError(
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
