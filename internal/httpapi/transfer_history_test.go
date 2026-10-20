package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agamstawn/go-transaction-service/internal/service"
)

type stubTransferHistoryReader struct {
	calls     int
	accountID int64
	page      int
	perPage   int
	err       error
}

func (s *stubTransferHistoryReader) ListByAccount(
	_ context.Context,
	accountID int64,
	page int,
	perPage int,
) (service.TransferPage, error) {
	s.calls++
	s.accountID = accountID
	s.page = page
	s.perPage = perPage

	if s.err != nil {
		return service.TransferPage{}, s.err
	}

	return service.TransferPage{
		Data: []service.TransferResult{
			{
				ID:            42,
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

func TestTransferHistoryHandlerScopedPagination(t *testing.T) {
	reader := &stubTransferHistoryReader{}

	handler := NewTransferHistoryHandler(reader, 7)

	// The client attempts to override the authorized account.
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers?page=1&per_page=10&account_id=999",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	// Verify HTTP status.
	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200; body = %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	// Verify repository invocation.
	if reader.calls != 1 {
		t.Fatalf(
			"repository calls = %d, want 1",
			reader.calls,
		)
	}

	// The account scope must come from the server, not the client.
	if reader.accountID != 7 {
		t.Errorf(
			"account ID = %d, want 7",
			reader.accountID,
		)
	}

	// Verify pagination parameters.
	if reader.page != 1 || reader.perPage != 10 {
		t.Errorf(
			"pagination = (%d, %d), want (1, 10)",
			reader.page,
			reader.perPage,
		)
	}

	// Decode the HTTP JSON response.
	var response struct {
		Data []service.TransferResult `json:"data"`

		Pagination struct {
			Page       int   `json:"page"`
			PerPage    int   `json:"per_page"`
			Total      int64 `json:"total"`
			TotalPages int64 `json:"total_pages"`
		} `json:"pagination"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	// Verify transfer data.
	if len(response.Data) != 1 {
		t.Fatalf(
			"transfers = %d, want 1",
			len(response.Data),
		)
	}

	if response.Data[0].ID != 42 {
		t.Errorf(
			"transfer ID = %d, want 42",
			response.Data[0].ID,
		)
	}

	// Verify pagination metadata.
	if response.Pagination.Page != 1 ||
		response.Pagination.PerPage != 10 ||
		response.Pagination.Total != 1 ||
		response.Pagination.TotalPages != 1 {
		t.Errorf(
			"unexpected pagination: %+v",
			response.Pagination,
		)
	}
}

func TestTransferHistoryHandlerDefaults(t *testing.T) {
	reader := &stubTransferHistoryReader{}

	handler := NewTransferHistoryHandler(reader, 7)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200",
			recorder.Code,
		)
	}

	if reader.page != 1 {
		t.Errorf("page = %d, want 1", reader.page)
	}

	if reader.perPage != 10 {
		t.Errorf("per_page = %d, want 10", reader.perPage)
	}

	if reader.accountID != 7 {
		t.Errorf(
			"account ID = %d, want 7",
			reader.accountID,
		)
	}
}

func TestTransferHistoryHandlerInvalidPagination(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "zero page",
			query: "page=0",
		},
		{
			name:  "negative page",
			query: "page=-1",
		},
		{
			name:  "non numeric page",
			query: "page=abc",
		},
		{
			name:  "empty page",
			query: "page=",
		},
		{
			name:  "zero page size",
			query: "per_page=0",
		},
		{
			name:  "negative page size",
			query: "per_page=-5",
		},
		{
			name:  "page size exceeds maximum",
			query: "per_page=101",
		},
		{
			name:  "duplicate page",
			query: "page=1&page=2",
		},
		{
			name:  "duplicate page size",
			query: "per_page=10&per_page=20",
		},
		{
			name:  "very large page",
			query: "page=999999999999999999999999999",
		},
		{
			name:  "offset overflow",
			query: "page=9223372036854775807&per_page=100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &stubTransferHistoryReader{}

			handler := NewTransferHistoryHandler(reader, 7)

			request := httptest.NewRequest(
				http.MethodGet,
				"/v1/transfers?"+tt.query,
				nil,
			)

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf(
					"status = %d, want 400",
					recorder.Code,
				)
			}

			if reader.calls != 0 {
				t.Errorf(
					"repository calls = %d, want 0",
					reader.calls,
				)
			}
		})
	}
}

func TestTransferHistoryHandlerRejectsMissingScope(t *testing.T) {
	reader := &stubTransferHistoryReader{}

	handler := NewTransferHistoryHandler(reader, 0)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf(
			"status = %d, want 503",
			recorder.Code,
		)
	}

	if reader.calls != 0 {
		t.Errorf(
			"repository calls = %d, want 0",
			reader.calls,
		)
	}
}

func TestTransferHistoryHandlerRepositoryError(t *testing.T) {
	reader := &stubTransferHistoryReader{
		err: errors.New("sensitive database connection details"),
	}

	handler := NewTransferHistoryHandler(reader, 7)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf(
			"status = %d, want 500",
			recorder.Code,
		)
	}

	if reader.calls != 1 {
		t.Errorf(
			"repository calls = %d, want 1",
			reader.calls,
		)
	}

	if strings.Contains(
		recorder.Body.String(),
		"sensitive database connection details",
	) {
		t.Error("internal database error leaked to client")
	}
}
