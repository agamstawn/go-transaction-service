package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/agamstawn/go-transaction-service/internal/service"
)

const (
	defaultHistoryPage     = 1
	defaultHistoryPageSize = 10
	maxHistoryPageSize     = 100
)

// TransferHistoryReader defines the dependency required by the handler.
type TransferHistoryReader interface {
	ListByAccount(
		ctx context.Context,
		authorizedAccountID int64,
		page int,
		perPage int,
	) (service.TransferPage, error)
}

// TransferHistoryHandler handles account-scoped transaction history.
type TransferHistoryHandler struct {
	reader              TransferHistoryReader
	authorizedAccountID int64
}

// NewTransferHistoryHandler creates a history HTTP handler.
func NewTransferHistoryHandler(
	reader TransferHistoryReader,
	authorizedAccountID int64,
) http.Handler {
	return &TransferHistoryHandler{
		reader:              reader,
		authorizedAccountID: authorizedAccountID,
	}
}

// transferHistoryPagination is the pagination metadata exposed by the API.
type transferHistoryPagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

// transferHistoryResponse defines the public JSON response structure.
type transferHistoryResponse struct {
	Data       []service.TransferResult  `json:"data"`
	Pagination transferHistoryPagination `json:"pagination"`
}

// ServeHTTP handles GET /v1/transfers.
func (h *TransferHistoryHandler) ServeHTTP(
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

	// 2. Reject missing or invalid account scope.
	if h.authorizedAccountID <= 0 {
		writeJSONError(
			w,
			http.StatusServiceUnavailable,
			"service unavailable",
		)
		return
	}

	// 3. Parse page parameter.
	page, err := parsePositiveQueryInt(
		r.URL.Query(),
		"page",
		defaultHistoryPage,
	)

	if err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid pagination parameters",
		)
		return
	}

	// 4. Parse per_page parameter.
	perPage, err := parsePositiveQueryInt(
		r.URL.Query(),
		"per_page",
		defaultHistoryPageSize,
	)

	if err != nil || perPage > maxHistoryPageSize {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid pagination parameters",
		)
		return
	}

	// 5. Prevent integer overflow when calculating OFFSET.
	if int64(page-1) > math.MaxInt64/int64(perPage) {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid pagination parameters",
		)
		return
	}

	// 6. Query using the server-authorized account scope.
	result, err := h.reader.ListByAccount(
		r.Context(),
		h.authorizedAccountID,
		page,
		perPage,
	)

	if err != nil {
		// Do not expose SQL or internal error details.
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	// 7. Return an empty JSON array instead of null.
	if result.Data == nil {
		result.Data = []service.TransferResult{}
	}

	// 8. Build the public JSON response.
	response := transferHistoryResponse{
		Data: result.Data,
		Pagination: transferHistoryPagination{
			Page:       result.Page,
			PerPage:    result.PerPage,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(response)
}

// parsePositiveQueryInt reads a positive integer query parameter.
func parsePositiveQueryInt(
	query url.Values,
	name string,
	defaultValue int,
) (int, error) {
	values, exists := query[name]

	if !exists {
		return defaultValue, nil
	}

	// Reject duplicate query parameters.
	if len(values) != 1 {
		return 0, fmt.Errorf("duplicate %s parameter", name)
	}

	value, err := strconv.Atoi(values[0])

	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s parameter", name)
	}

	return value, nil
}
