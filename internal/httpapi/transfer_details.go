package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/agam-vs/go-transaction-service/internal/service"
)

// TransferReader defines the behavior required to retrieve transfers.
type TransferReader interface {
	GetByID(
		ctx context.Context,
		transferID int64,
		authorizedAccountID int64,
	) (service.TransferResult, error)
}

// TransferDetailsHandler handles transfer detail requests.
type TransferDetailsHandler struct {
	repository          TransferReader
	authorizedAccountID int64
}

// NewTransferDetailsHandler creates a transfer details HTTP handler.
func NewTransferDetailsHandler(
	repository TransferReader,
	authorizedAccountID int64,
) http.Handler {
	return &TransferDetailsHandler{
		repository:          repository,
		authorizedAccountID: authorizedAccountID,
	}
}

// ServeHTTP handles GET /v1/transfers/{id}.
func (h *TransferDetailsHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	// 1. Only allow GET requests.
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")

		writeJSONError(
			w,
			http.StatusMethodNotAllowed,
			"method not allowed",
		)
		return
	}

	// 2. Fail closed if account scope is not configured.
	if h.authorizedAccountID <= 0 {
		writeJSONError(
			w,
			http.StatusServiceUnavailable,
			"service unavailable",
		)
		return
	}

	// 3. Parse the transfer ID from the URL.
	transferID, err := strconv.ParseInt(
		r.PathValue("id"),
		10,
		64,
	)

	if err != nil || transferID <= 0 {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid transfer ID",
		)
		return
	}

	// 4. Retrieve the transfer within the authorized account scope.
	transfer, err := h.repository.GetByID(
		r.Context(),
		transferID,
		h.authorizedAccountID,
	)

	if err != nil {
		// Hide whether the transfer exists outside the account scope.
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(
				w,
				http.StatusNotFound,
				"transfer not found",
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

	// 5. Return the transfer as JSON.
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(transfer)
}
