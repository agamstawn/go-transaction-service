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

	"github.com/agam-vs/go-transaction-service/internal/domain"
)

type fakeAccountReader struct {
	account domain.Account
	err     error
	calls   int
	lastID  int64
}

func (f *fakeAccountReader) GetByID(
	_ context.Context,
	id int64,
) (domain.Account, error) {
	f.calls++
	f.lastID = id

	return f.account, f.err
}

func newAccountTestRouter(
	reader *fakeAccountReader,
) http.Handler {
	mux := http.NewServeMux()

	mux.Handle(
		"GET /v1/accounts/{id}",
		NewAccountHandler(reader),
	)

	return mux
}

func TestAccountHandlerSuccess(t *testing.T) {
	reader := &fakeAccountReader{
		account: domain.Account{
			ID:       7,
			Number:   "10000007",
			Currency: "IDR",
			Balance:  500_000,
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/7",
		nil,
	)

	recorder := httptest.NewRecorder()

	newAccountTestRouter(reader).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200",
			recorder.Code,
		)
	}

	var account domain.Account

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&account,
	); err != nil {
		t.Fatal(err)
	}

	if account.ID != 7 || account.Balance != 500_000 {
		t.Errorf("unexpected account: %+v", account)
	}

	if reader.calls != 1 || reader.lastID != 7 {
		t.Errorf(
			"repository calls=%d id=%d",
			reader.calls,
			reader.lastID,
		)
	}
}

func TestAccountHandlerInvalidID(t *testing.T) {
	for _, path := range []string{
		"/v1/accounts/abc",
		"/v1/accounts/0",
		"/v1/accounts/-1",
	} {
		t.Run(path, func(t *testing.T) {
			reader := &fakeAccountReader{}

			request := httptest.NewRequest(
				http.MethodGet,
				path,
				nil,
			)

			recorder := httptest.NewRecorder()

			newAccountTestRouter(reader).ServeHTTP(
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
				t.Error("repository should not be called")
			}
		})
	}
}

func TestAccountHandlerNotFound(t *testing.T) {
	reader := &fakeAccountReader{
		err: sql.ErrNoRows,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/999",
		nil,
	)

	recorder := httptest.NewRecorder()

	newAccountTestRouter(reader).ServeHTTP(
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

func TestAccountHandlerInternalError(t *testing.T) {
	reader := &fakeAccountReader{
		err: errors.New("sensitive database details"),
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/1",
		nil,
	)

	recorder := httptest.NewRecorder()

	newAccountTestRouter(reader).ServeHTTP(
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

func TestAccountHandlerForbidden(t *testing.T) {
	reader := &fakeAccountReader{
		err: domain.ErrAccountForbidden,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/8",
		nil,
	)

	recorder := httptest.NewRecorder()

	newAccountTestRouter(reader).ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusForbidden {
		t.Errorf(
			"status = %d, want 403",
			recorder.Code,
		)
	}

	if reader.calls != 1 {
		t.Errorf(
			"repository calls = %d, want 1",
			reader.calls,
		)
	}
}
