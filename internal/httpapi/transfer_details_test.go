package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agam-vs/go-transaction-service/internal/service"
)

type fakeTransferReader struct {
	result            service.TransferResult
	err               error
	calls             int
	receivedID        int64
	receivedAccountID int64
}

func (f *fakeTransferReader) GetByID(
	_ context.Context,
	transferID int64,
	authorizedAccountID int64,
) (service.TransferResult, error) {
	f.calls++
	f.receivedID = transferID
	f.receivedAccountID = authorizedAccountID

	return f.result, f.err
}

func newTransferDetailsTestRouter(
	reader *fakeTransferReader,
) http.Handler {
	mux := http.NewServeMux()

	mux.Handle(
		"GET /v1/transfers/{id}",
		NewTransferDetailsHandler(reader, 7),
	)

	return mux
}

func TestTransferDetailsHandlerSuccess(t *testing.T) {
	reader := &fakeTransferReader{
		result: service.TransferResult{
			ID:            100,
			FromAccountID: 7,
			ToAccountID:   8,
			Amount:        125_000,
			Status:        "completed",
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers/100",
		nil,
	)

	recorder := httptest.NewRecorder()

	newTransferDetailsTestRouter(reader).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200",
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

	if result != reader.result {
		t.Errorf(
			"result = %+v, want %+v",
			result,
			reader.result,
		)
	}

	if reader.calls != 1 ||
		reader.receivedID != 100 ||
		reader.receivedAccountID != 7 {
		t.Errorf(
			"unexpected repository call: calls=%d id=%d account=%d",
			reader.calls,
			reader.receivedID,
			reader.receivedAccountID,
		)
	}
}

func TestTransferDetailsHandlerInvalidID(t *testing.T) {
	for _, path := range []string{
		"/v1/transfers/abc",
		"/v1/transfers/0",
		"/v1/transfers/-1",
	} {
		t.Run(path, func(t *testing.T) {
			reader := &fakeTransferReader{}

			request := httptest.NewRequest(
				http.MethodGet,
				path,
				nil,
			)

			recorder := httptest.NewRecorder()

			newTransferDetailsTestRouter(reader).ServeHTTP(
				recorder,
				request,
			)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf(
					"status = %d, want 400",
					recorder.Code,
				)
			}

			if reader.calls != 0 {
				t.Error("repository must not be called")
			}
		})
	}
}

func TestTransferDetailsHandlerNotFound(t *testing.T) {
	reader := &fakeTransferReader{
		err: sql.ErrNoRows,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers/999",
		nil,
	)

	recorder := httptest.NewRecorder()

	newTransferDetailsTestRouter(reader).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNotFound {
		t.Errorf(
			"status = %d, want 404",
			recorder.Code,
		)
	}
}

func TestTransferDetailsHandlerInternalError(t *testing.T) {
	reader := &fakeTransferReader{
		err: errors.New("sensitive database details"),
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers/100",
		nil,
	)

	recorder := httptest.NewRecorder()

	newTransferDetailsTestRouter(reader).ServeHTTP(
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
		"sensitive database details",
	) {
		t.Error("internal database error leaked to client")
	}
}

func TestTransferDetailsHandlerRejectsMissingAccountScope(t *testing.T) {
	reader := &fakeTransferReader{}

	handler := NewTransferDetailsHandler(
		reader,
		0,
	)

	mux := http.NewServeMux()

	mux.Handle(
		"GET /v1/transfers/{id}",
		handler,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/transfers/100",
		nil,
	)

	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf(
			"status = %d, want 503",
			recorder.Code,
		)
	}

	if reader.calls != 0 {
		t.Error("repository must not be called without account scope")
	}
}
