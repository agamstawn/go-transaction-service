//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/agamstawn/go-transaction-service/internal/postgres"
	"github.com/agamstawn/go-transaction-service/internal/service"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	e2eAPIKey           = "e2e-test-secret"
	e2eSenderBalance    = int64(500_000)
	e2eRecipientBalance = int64(200_000)
	e2eTransferAmount   = int64(25_000)
)

// openE2EDatabase connects exclusively to transactions_test.
func openE2EDatabase(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}

	parsedURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}

	if parsedURL.Path != "/transactions_test" {
		t.Fatal("E2E tests require transactions_test database")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	var databaseName string

	err = db.QueryRowContext(
		ctx,
		"SELECT current_database()",
	).Scan(&databaseName)

	if err != nil {
		t.Fatalf("verify database: %v", err)
	}

	if databaseName != "transactions_test" {
		t.Fatalf("unexpected database: %s", databaseName)
	}

	return db
}

// createE2EAccount inserts an isolated account and registers cleanup.
func createE2EAccount(
	t *testing.T,
	db *sql.DB,
	prefix string,
	balance int64,
) int64 {
	t.Helper()

	var suffix [8]byte

	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate account suffix: %v", err)
	}

	accountNumber := fmt.Sprintf(
		"%s-%x",
		prefix,
		suffix,
	)

	var accountID int64

	err := db.QueryRow(
		`INSERT INTO accounts (
			account_number,
			currency,
			balance
		)
		VALUES ($1, 'IDR', $2)
		RETURNING id`,
		accountNumber,
		balance,
	).Scan(&accountID)

	if err != nil {
		t.Fatalf("create E2E account: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		// Delete only transfers involving this test account.
		_, err := db.ExecContext(
			ctx,
			`DELETE FROM transfers
			 WHERE from_account_id = $1
			    OR to_account_id = $1`,
			accountID,
		)

		if err != nil {
			t.Errorf("cleanup E2E transfers: %v", err)
			return
		}

		_, err = db.ExecContext(
			ctx,
			"DELETE FROM accounts WHERE id = $1",
			accountID,
		)

		if err != nil {
			t.Errorf("cleanup E2E account: %v", err)
		}
	})

	return accountID
}

// sendE2ERequest performs a real HTTP request against the test server.
func sendE2ERequest(
	t *testing.T,
	client *http.Client,
	method string,
	targetURL string,
	body []byte,
	idempotencyKey string,
) (int, http.Header, []byte) {
	t.Helper()

	request, err := http.NewRequest(
		method,
		targetURL,
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("create HTTP request: %v", err)
	}

	request.Header.Set("X-API-Key", e2eAPIKey)

	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute HTTP request: %v", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(
		io.LimitReader(response.Body, 1<<20),
	)
	if err != nil {
		t.Fatalf("read HTTP response: %v", err)
	}

	return response.StatusCode, response.Header.Clone(), responseBody
}

// assertE2EState verifies balances and transfer count.
func assertE2EState(
	t *testing.T,
	db *sql.DB,
	client *http.Client,
	serverURL string,
	senderID int64,
	recipientID int64,
	expectedSenderBalance int64,
	expectedRecipientBalance int64,
	expectedTransferCount int64,
) {
	t.Helper()

	// Verify sender balance through the public API.
	status, _, body := sendE2ERequest(
		t,
		client,
		http.MethodGet,
		fmt.Sprintf("%s/v1/accounts/%d", serverURL, senderID),
		nil,
		"",
	)

	if status != http.StatusOK {
		t.Fatalf("GET sender account: status=%d body=%s", status, body)
	}

	var sender struct {
		Balance int64 `json:"balance"`
	}

	if err := json.Unmarshal(body, &sender); err != nil {
		t.Fatalf("decode sender account: %v", err)
	}

	if sender.Balance != expectedSenderBalance {
		t.Errorf(
			"sender balance = %d, want %d",
			sender.Balance,
			expectedSenderBalance,
		)
	}

	// Recipient belongs to another account scope.
	// Verify its balance directly in the isolated test database.
	var recipientBalance int64

	err := db.QueryRow(
		"SELECT balance FROM accounts WHERE id = $1",
		recipientID,
	).Scan(&recipientBalance)

	if err != nil {
		t.Fatalf("read recipient balance: %v", err)
	}

	if recipientBalance != expectedRecipientBalance {
		t.Errorf(
			"recipient balance = %d, want %d",
			recipientBalance,
			expectedRecipientBalance,
		)
	}

	var transferCount int64

	err = db.QueryRow(
		`SELECT COUNT(*)
		 FROM transfers
		 WHERE from_account_id = $1`,
		senderID,
	).Scan(&transferCount)

	if err != nil {
		t.Fatalf("count transfers: %v", err)
	}

	if transferCount != expectedTransferCount {
		t.Errorf(
			"transfer count = %d, want %d",
			transferCount,
			expectedTransferCount,
		)
	}
}

func TestTransferLifecycleE2E(t *testing.T) {
	// 1. Initialize isolated database fixtures.
	db := openE2EDatabase(t)

	senderID := createE2EAccount(
		t,
		db,
		"e2e-sender",
		e2eSenderBalance,
	)

	recipientID := createE2EAccount(
		t,
		db,
		"e2e-recipient",
		e2eRecipientBalance,
	)

	// 2. Initialize real application dependencies.
	accountRepository := postgres.NewAccountRepository(db)

	transferProcessor := postgres.NewTransferProcessor(db)

	transferService := service.NewTransferService(
		transferProcessor,
	)

	transferRepository := postgres.NewTransferRepository(db)

	// 3. Scope the API key to the newly created sender account.
	router := newAppRouter(
		accountRepository,
		transferService,
		transferRepository,
		transferRepository,
		e2eAPIKey,
		senderID,
	)

	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	// 4. Prepare the transfer request.
	transferBody := []byte(fmt.Sprintf(
		`{"from_account_id":%d,"to_account_id":%d,"amount":%d}`,
		senderID,
		recipientID,
		e2eTransferAmount,
	))

	idempotencyKey := fmt.Sprintf(
		"e2e-lifecycle-%d",
		senderID,
	)

	// 5. Execute a new transfer.
	status, headers, body := sendE2ERequest(
		t,
		client,
		http.MethodPost,
		server.URL+"/v1/transfers",
		transferBody,
		idempotencyKey,
	)

	if status != http.StatusCreated {
		t.Fatalf(
			"create transfer: status=%d, want 201; body=%s",
			status,
			body,
		)
	}

	if headers.Get("Cache-Control") != "no-store" {
		t.Error("missing Cache-Control: no-store")
	}

	var created service.TransferResult

	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode created transfer: %v", err)
	}

	if created.ID <= 0 ||
		created.FromAccountID != senderID ||
		created.ToAccountID != recipientID ||
		created.Amount != e2eTransferAmount ||
		created.Status != "completed" {
		t.Fatalf("unexpected created transfer: %+v", created)
	}

	t.Logf("created transfer ID=%d", created.ID)

	// 6. Verify balances after the first transfer.
	assertE2EState(
		t,
		db,
		client,
		server.URL,
		senderID,
		recipientID,
		e2eSenderBalance-e2eTransferAmount,
		e2eRecipientBalance+e2eTransferAmount,
		1,
	)

	// 7. Read the transfer through the details endpoint.
	status, _, body = sendE2ERequest(
		t,
		client,
		http.MethodGet,
		fmt.Sprintf("%s/v1/transfers/%d", server.URL, created.ID),
		nil,
		"",
	)

	if status != http.StatusOK {
		t.Fatalf(
			"GET transfer details: status=%d body=%s",
			status,
			body,
		)
	}

	var details service.TransferResult

	if err := json.Unmarshal(body, &details); err != nil {
		t.Fatalf("decode transfer details: %v", err)
	}

	if details.ID != created.ID ||
		details.Amount != e2eTransferAmount {
		t.Errorf("unexpected transfer details: %+v", details)
	}

	// 8. Verify transfer history.
	status, _, body = sendE2ERequest(
		t,
		client,
		http.MethodGet,
		server.URL+"/v1/transfers?page=1&per_page=10",
		nil,
		"",
	)

	if status != http.StatusOK {
		t.Fatalf(
			"GET transfer history: status=%d body=%s",
			status,
			body,
		)
	}

	var history struct {
		Data []service.TransferResult `json:"data"`

		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}

	if err := json.Unmarshal(body, &history); err != nil {
		t.Fatalf("decode transfer history: %v", err)
	}

	if history.Pagination.Total != 1 ||
		len(history.Data) != 1 ||
		history.Data[0].ID != created.ID {
		t.Fatalf("unexpected transfer history: %+v", history)
	}

	// 9. Verify that the sender cannot access the recipient account.
	status, _, body = sendE2ERequest(
		t,
		client,
		http.MethodGet,
		fmt.Sprintf("%s/v1/accounts/%d", server.URL, recipientID),
		nil,
		"",
	)

	if status != http.StatusForbidden {
		t.Fatalf(
			"recipient account access: status=%d, want 403; body=%s",
			status,
			body,
		)
	}

	// 10. Replay exactly the same transfer request.
	status, headers, body = sendE2ERequest(
		t,
		client,
		http.MethodPost,
		server.URL+"/v1/transfers",
		transferBody,
		idempotencyKey,
	)

	if status != http.StatusOK {
		t.Fatalf(
			"idempotent replay: status=%d, want 200; body=%s",
			status,
			body,
		)
	}

	if headers.Get("Idempotent-Replayed") != "true" {
		t.Error("missing Idempotent-Replayed: true header")
	}

	var replayed service.TransferResult

	if err := json.Unmarshal(body, &replayed); err != nil {
		t.Fatalf("decode replayed transfer: %v", err)
	}

	if replayed.ID != created.ID {
		t.Errorf(
			"replayed transfer ID=%d, want %d",
			replayed.ID,
			created.ID,
		)
	}

	// 11. Confirm replay did not create a second debit.
	assertE2EState(
		t,
		db,
		client,
		server.URL,
		senderID,
		recipientID,
		e2eSenderBalance-e2eTransferAmount,
		e2eRecipientBalance+e2eTransferAmount,
		1,
	)

	// 12. Reuse the same idempotency key with a different amount.
	conflictingBody := []byte(fmt.Sprintf(
		`{"from_account_id":%d,"to_account_id":%d,"amount":%d}`,
		senderID,
		recipientID,
		e2eTransferAmount+1,
	))

	status, _, body = sendE2ERequest(
		t,
		client,
		http.MethodPost,
		server.URL+"/v1/transfers",
		conflictingBody,
		idempotencyKey,
	)

	if status != http.StatusConflict {
		t.Fatalf(
			"idempotency conflict: status=%d, want 409; body=%s",
			status,
			body,
		)
	}

	// 13. Verify conflict did not change balances or transfer count.
	assertE2EState(
		t,
		db,
		client,
		server.URL,
		senderID,
		recipientID,
		e2eSenderBalance-e2eTransferAmount,
		e2eRecipientBalance+e2eTransferAmount,
		1,
	)

	t.Log("transfer lifecycle verified successfully")
}
