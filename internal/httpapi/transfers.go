package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/agamstawn/go-transaction-service/internal/domain"
	"github.com/agamstawn/go-transaction-service/internal/service"
)

const maxTransferRequestBytes = 64 << 10

// TransferExecutor defines the behavior required to process transfers.
type TransferExecutor interface {
	Transfer(
		ctx context.Context,
		input domain.TransferInput,
	) (service.TransferResult, error)
}

// TransferHandler handles transfer-related HTTP requests.
type TransferHandler struct {
	executor TransferExecutor
}

// NewTransferHandler creates a transfer HTTP handler.
func NewTransferHandler(executor TransferExecutor) http.Handler {
	return &TransferHandler{
		executor: executor,
	}
}

// ServeHTTP handles POST /v1/transfers.
func (h *TransferHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")

		writeJSONError(
			w,
			http.StatusMethodNotAllowed,
			"method not allowed",
		)
		return
	}

	// Require exactly one JSON Content-Type header.
	contentTypes := r.Header.Values("Content-Type")

	if len(contentTypes) != 1 {
		writeJSONError(
			w,
			http.StatusUnsupportedMediaType,
			"Content-Type must be application/json",
		)
		return
	}

	mediaType, _, err := mime.ParseMediaType(contentTypes[0])

	if err != nil || mediaType != "application/json" {
		writeJSONError(
			w,
			http.StatusUnsupportedMediaType,
			"Content-Type must be application/json",
		)
		return
	}

	// 1. Read the idempotency key.
	keys := r.Header.Values("Idempotency-Key")

	if len(keys) != 1 {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"exactly one idempotency key is required",
		)
		return
	}

	// 2. Decode the JSON request body.
	var input domain.TransferInput

	decoder := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, maxTransferRequestBytes),
	)

	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeTransferDecodeError(w, err)
		return
	}

	// Reject multiple JSON values in the same request.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			writeJSONError(
				w,
				http.StatusBadRequest,
				"only one JSON object is allowed",
			)
			return
		}

		writeTransferDecodeError(w, err)
		return
	}

	// 3. Map the HTTP header into the domain input.
	input.IdempotencyKey = keys[0]

	// 4. Validate business rules.
	if err := input.Validate(); err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	// 5. Validate the idempotency key.
	if err := input.ValidateIdempotencyKey(); err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid idempotency key",
		)
		return
	}

	// 6. Execute the transfer.
	result, err := h.executor.Transfer(
		r.Context(),
		input,
	)

	if err != nil {
		writeTransferError(w, err)
		return
	}

	// 7. Return the transaction result.
	// Determine whether the transfer was newly created or replayed.
	status := http.StatusCreated

	if result.Replayed {
		status = http.StatusOK

		w.Header().Set(
			"Idempotent-Replayed",
			"true",
		)
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(result)

}

func writeTransferDecodeError(
	w http.ResponseWriter,
	err error,
) {
	var maxBytesErr *http.MaxBytesError

	if errors.As(err, &maxBytesErr) {
		writeJSONError(
			w,
			http.StatusRequestEntityTooLarge,
			"request body too large",
		)
		return
	}

	writeJSONError(
		w,
		http.StatusBadRequest,
		"invalid JSON body",
	)
}

func writeTransferError(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(err, domain.ErrInvalidAccount),
		errors.Is(err, domain.ErrSameAccount),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrInvalidIdempotencyKey):

		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid transfer request",
		)

	case errors.Is(err, domain.ErrInsufficientFunds):
		writeJSONError(
			w,
			http.StatusUnprocessableEntity,
			"insufficient funds",
		)

	case errors.Is(err, domain.ErrIdempotencyConflict):
		writeJSONError(
			w,
			http.StatusConflict,
			"idempotency key conflict",
		)

	case errors.Is(err, sql.ErrNoRows):
		writeJSONError(
			w,
			http.StatusNotFound,
			"account not found",
		)
	case errors.Is(err, domain.ErrAccountForbidden):
		writeJSONError(
			w,
			http.StatusForbidden,
			"account access forbidden",
		)

	default:
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	}
}
