package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/agam-vs/go-transaction-service/internal/domain"
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
	// 1. Extract and validate account ID.
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

	// 2. Retrieve account using the repository.
	account, err := h.repository.GetByID(
		r.Context(),
		id,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(
				w,
				http.StatusNotFound,
				"account not found",
			)
			return
		}

		// Never expose internal database errors.
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	// 3. Return account as JSON.
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
