package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agam-vs/go-transaction-service/internal/domain"
	"github.com/agam-vs/go-transaction-service/internal/service"
)

type fakeTransferExecutor struct {
	result        service.TransferResult
	err           error
	calls         int
	receivedInput domain.TransferInput
}

func (f *fakeTransferExecutor) Transfer(
	ctx context.Context,
	input domain.TransferInput,
) (service.TransferResult, error) {
	f.calls++
	f.receivedInput = input

	return f.result, f.err
}

func TestTransferHandlerSuccess(t *testing.T) {
	executor := &fakeTransferExecutor{
		result: service.TransferResult{
			ID:            100,
			FromAccountID: 1,
			ToAccountID:   2,
			Amount:        125_000,
			Status:        "completed",
		},
	}

	body := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/transfers",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Idempotency-Key",
		"transfer-test-001",
	)

	recorder := httptest.NewRecorder()

	NewTransferHandler(executor).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want 201",
			recorder.Code,
		)
	}

	var result service.TransferResult

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&result,
	); err != nil {
		t.Fatal(err)
	}

	if result != executor.result {
		t.Errorf(
			"result = %+v, want %+v",
			result,
			executor.result,
		)
	}

	if executor.calls != 1 {
		t.Errorf(
			"service calls = %d, want 1",
			executor.calls,
		)
	}

	if executor.receivedInput.IdempotencyKey != "transfer-test-001" {
		t.Error("idempotency key was not forwarded")
	}
}

func TestTransferHandlerMissingIdempotencyKey(t *testing.T) {
	executor := &fakeTransferExecutor{}

	body := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/transfers",
		strings.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	NewTransferHandler(executor).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf(
			"status = %d, want 400",
			recorder.Code,
		)
	}

	if executor.calls != 0 {
		t.Error("service must not be called")
	}
}

func TestTransferHandlerInsufficientFunds(t *testing.T) {
	executor := &fakeTransferExecutor{
		err: domain.ErrInsufficientFunds,
	}

	body := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/transfers",
		strings.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")

	request.Header.Set(
		"Idempotency-Key",
		"insufficient-funds-001",
	)

	recorder := httptest.NewRecorder()

	NewTransferHandler(executor).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf(
			"status = %d, want 422",
			recorder.Code,
		)
	}

	if executor.calls != 1 {
		t.Errorf(
			"service calls = %d, want 1",
			executor.calls,
		)
	}
}

func TestTransferHandlerIdempotencyConflict(t *testing.T) {
	executor := &fakeTransferExecutor{
		err: domain.ErrIdempotencyConflict,
	}

	body := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/transfers",
		strings.NewReader(body),
	)

	// Required JSON content type.
	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	// Idempotency key for the conflict scenario.
	request.Header.Set(
		"Idempotency-Key",
		"conflict-001",
	)

	recorder := httptest.NewRecorder()

	NewTransferHandler(executor).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusConflict {
		t.Errorf(
			"status = %d, want 409",
			recorder.Code,
		)
	}

	if executor.calls != 1 {
		t.Errorf(
			"service calls = %d, want 1",
			executor.calls,
		)
	}
}

func TestTransferHandlerInternalError(t *testing.T) {
	executor := &fakeTransferExecutor{
		err: errors.New("sensitive database failure"),
	}

	body := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/transfers",
		strings.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Idempotency-Key",
		"internal-error-001",
	)

	recorder := httptest.NewRecorder()

	NewTransferHandler(executor).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf(
			"status = %d, want 500",
			recorder.Code,
		)
	}

	if strings.Contains(
		recorder.Body.String(),
		"sensitive database failure",
	) {
		t.Error("internal error leaked to HTTP response")
	}
}

func TestTransferHandlerRequestValidation(t *testing.T) {
	validBody := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	tests := []struct {
		name          string
		body          string
		contentType   string
		key           string
		duplicateKey  bool
		duplicateType bool
		wantStatus    int
		wantCalls     int
	}{
		{
			name:        "malformed JSON",
			body:        `{"from_account_id":`,
			contentType: "application/json",
			key:         "test-001",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name: "unknown JSON field",
			body: `{
				"from_account_id": 1,
				"to_account_id": 2,
				"amount": 125000,
				"unexpected": true
			}`,
			contentType: "application/json",
			key:         "test-002",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "multiple JSON objects",
			body:        validBody + "\n" + validBody,
			contentType: "application/json",
			key:         "test-003",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name: "negative amount",
			body: `{
				"from_account_id": 1,
				"to_account_id": 2,
				"amount": -10000
			}`,
			contentType: "application/json",
			key:         "test-004",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "invalid idempotency key",
			body:        validBody,
			contentType: "application/json",
			key:         "invalid key",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:         "duplicate idempotency headers",
			body:         validBody,
			contentType:  "application/json",
			key:          "test-005",
			duplicateKey: true,
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:        "unsupported content type",
			body:        validBody,
			contentType: "text/plain",
			key:         "test-006",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:       "missing content type",
			body:       validBody,
			key:        "test-007",
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:          "duplicate content type headers",
			body:          validBody,
			contentType:   "application/json",
			key:           "test-008",
			duplicateType: true,
			wantStatus:    http.StatusUnsupportedMediaType,
		},
		{
			name:        "JSON with charset",
			body:        validBody,
			contentType: "application/json; charset=utf-8",
			key:         "test-009",
			wantStatus:  http.StatusCreated,
			wantCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeTransferExecutor{}

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/transfers",
				strings.NewReader(tt.body),
			)

			if tt.contentType != "" {
				request.Header.Set(
					"Content-Type",
					tt.contentType,
				)
			}

			if tt.duplicateType {
				request.Header.Add(
					"Content-Type",
					tt.contentType,
				)
			}

			request.Header.Set(
				"Idempotency-Key",
				tt.key,
			)

			if tt.duplicateKey {
				request.Header.Add(
					"Idempotency-Key",
					tt.key,
				)
			}

			recorder := httptest.NewRecorder()

			NewTransferHandler(executor).ServeHTTP(
				recorder,
				request,
			)

			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d",
					recorder.Code,
					tt.wantStatus,
				)
			}

			if executor.calls != tt.wantCalls {
				t.Errorf(
					"service calls = %d, want %d",
					executor.calls,
					tt.wantCalls,
				)
			}
		})
	}
}

func TestTransferHandlerRejectsOversizedBody(t *testing.T) {
	executor := &fakeTransferExecutor{}

	validBody := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	// Create a request body larger than 64 KiB.
	body := strings.Repeat(
		" ",
		maxTransferRequestBytes,
	) + validBody

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/transfers",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	request.Header.Set(
		"Idempotency-Key",
		"oversized-body-001",
	)

	recorder := httptest.NewRecorder()

	NewTransferHandler(executor).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf(
			"status = %d, want 413",
			recorder.Code,
		)
	}

	if executor.calls != 0 {
		t.Errorf(
			"service calls = %d, want 0",
			executor.calls,
		)
	}
}

func TestTransferHandlerIdempotentReplay(t *testing.T) {
	executor := &fakeTransferExecutor{
		result: service.TransferResult{
			ID:            100,
			FromAccountID: 1,
			ToAccountID:   2,
			Amount:        125_000,
			Status:        "completed",
			Replayed:      true,
		},
	}

	body := `{
		"from_account_id": 1,
		"to_account_id": 2,
		"amount": 125000
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/transfers",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	request.Header.Set(
		"Idempotency-Key",
		"replay-test-001",
	)

	recorder := httptest.NewRecorder()

	NewTransferHandler(executor).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Errorf(
			"status = %d, want 200",
			recorder.Code,
		)
	}

	if executor.calls != 1 {
		t.Errorf(
			"service calls = %d, want 1",
			executor.calls,
		)
	}

	if recorder.Header().Get("Idempotent-Replayed") != "true" {
		t.Error("missing idempotent replay header")
	}

	var response service.TransferResult

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatal(err)
	}

	if response.ID != 100 {
		t.Errorf(
			"transfer ID = %d, want 100",
			response.ID,
		)
	}
}
