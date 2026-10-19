//go:build integration

package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/agam-vs/go-transaction-service/internal/domain"
)

func createTestAccount(
	t *testing.T,
	db *sql.DB,
	prefix string,
	balance int64,
) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	var suffix [8]byte

	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate account number: %v", err)
	}

	accountNumber := fmt.Sprintf(
		"%.12s-%x",
		prefix,
		suffix,
	)

	var id int64

	err := db.QueryRowContext(
		ctx,
		`INSERT INTO accounts (
			account_number,
			currency,
			balance
		)
		VALUES ($1, 'IDR', $2)
		RETURNING id`,
		accountNumber,
		balance,
	).Scan(&id)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		// Remove transfer records before deleting the account.
		_, err := db.ExecContext(
			cleanupCtx,
			`DELETE FROM transfers
			 WHERE from_account_id = $1
			    OR to_account_id = $1`,
			id,
		)

		if err != nil {
			t.Errorf("cleanup transfers: %v", err)
			return
		}

		_, err = db.ExecContext(
			cleanupCtx,
			"DELETE FROM accounts WHERE id = $1",
			id,
		)

		if err != nil {
			t.Errorf("cleanup account: %v", err)
		}
	})

	return id
}

func TestTransferProcessorExecuteTransfer(t *testing.T) {
	db := openTestDB(t)

	senderID := createTestAccount(
		t, db, "sender", 500_000,
	)

	recipientID := createTestAccount(
		t, db, "recipient", 200_000,
	)

	processor := NewTransferProcessor(db)

	input := domain.TransferInput{
		FromAccountID: senderID,
		ToAccountID:   recipientID,
		Amount:        125_000,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	result, err := processor.ExecuteTransfer(ctx, input)

	if err != nil {
		t.Fatalf("ExecuteTransfer() error = %v", err)
	}

	if result.ID <= 0 {
		t.Error("expected a positive transfer ID")
	}

	if result.Status != "completed" {
		t.Errorf(
			"status = %s, want completed",
			result.Status,
		)
	}

	if result.FromAccountID != senderID ||
		result.ToAccountID != recipientID ||
		result.Amount != input.Amount {
		t.Errorf("unexpected transfer result: %+v", result)
	}

	// Verify the sender's final balance.
	var senderBalance int64

	err = db.QueryRowContext(
		ctx,
		"SELECT balance FROM accounts WHERE id = $1",
		senderID,
	).Scan(&senderBalance)

	if err != nil {
		t.Fatal(err)
	}

	if senderBalance != 375_000 {
		t.Errorf(
			"sender balance = %d, want 375000",
			senderBalance,
		)
	}

	// Verify the recipient's final balance.
	var recipientBalance int64

	err = db.QueryRowContext(
		ctx,
		"SELECT balance FROM accounts WHERE id = $1",
		recipientID,
	).Scan(&recipientBalance)

	if err != nil {
		t.Fatal(err)
	}

	if recipientBalance != 325_000 {
		t.Errorf(
			"recipient balance = %d, want 325000",
			recipientBalance,
		)
	}

	// Verify that the transfer was recorded.
	var storedAmount int64
	var storedStatus string

	err = db.QueryRowContext(
		ctx,
		`SELECT amount, status
		 FROM transfers
		 WHERE id = $1`,
		result.ID,
	).Scan(&storedAmount, &storedStatus)

	if err != nil {
		t.Fatal(err)
	}

	if storedAmount != 125_000 {
		t.Errorf(
			"stored amount = %d, want 125000",
			storedAmount,
		)
	}

	if storedStatus != "completed" {
		t.Errorf(
			"stored status = %s, want completed",
			storedStatus,
		)
	}
}

func readTestBalance(t *testing.T, db *sql.DB, id int64) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	var balance int64

	err := db.QueryRowContext(
		ctx,
		"SELECT balance FROM accounts WHERE id = $1",
		id,
	).Scan(&balance)

	if err != nil {
		t.Fatal(err)
	}

	return balance
}

func countTestTransfers(
	t *testing.T,
	db *sql.DB,
	senderID, recipientID int64,
) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	var count int

	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM transfers
		 WHERE from_account_id = $1
		   AND to_account_id = $2`,
		senderID,
		recipientID,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	return count
}

func TestTransferProcessorInsufficientFunds(t *testing.T) {
	db := openTestDB(t)

	senderID := createTestAccount(
		t, db, "insufficient-src", 100_000,
	)

	recipientID := createTestAccount(
		t, db, "insufficient-dst", 200_000,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	_, err := NewTransferProcessor(db).ExecuteTransfer(
		ctx,
		domain.TransferInput{
			FromAccountID: senderID,
			ToAccountID:   recipientID,
			Amount:        125_000,
		},
	)

	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf(
			"error = %v, want ErrInsufficientFunds",
			err,
		)
	}

	if got := readTestBalance(t, db, senderID); got != 100_000 {
		t.Errorf("sender balance = %d, want 100000", got)
	}

	if got := readTestBalance(t, db, recipientID); got != 200_000 {
		t.Errorf("recipient balance = %d, want 200000", got)
	}

	if got := countTestTransfers(
		t, db, senderID, recipientID,
	); got != 0 {
		t.Errorf("transfer records = %d, want 0", got)
	}
}

func TestTransferProcessorRollbackOnCreditFailure(t *testing.T) {
	db := openTestDB(t)

	senderID := createTestAccount(
		t, db, "rollback-src", 500_000,
	)

	recipientID := createTestAccount(
		t, db, "rollback-dst", math.MaxInt64,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	_, err := NewTransferProcessor(db).ExecuteTransfer(
		ctx,
		domain.TransferInput{
			FromAccountID: senderID,
			ToAccountID:   recipientID,
			Amount:        125_000,
		},
	)

	if err == nil {
		t.Fatal(
			"expected error when recipient balance overflows BIGINT",
		)
	}

	if got := readTestBalance(t, db, senderID); got != 500_000 {
		t.Errorf(
			"sender balance = %d, want 500000 (rollback)",
			got,
		)
	}

	if got := readTestBalance(t, db, recipientID); got != math.MaxInt64 {
		t.Errorf(
			"recipient balance = %d, want MaxInt64",
			got,
		)
	}

	if got := countTestTransfers(
		t, db, senderID, recipientID,
	); got != 0 {
		t.Errorf("transfer records = %d, want 0", got)
	}
}

func TestTransferProcessorConcurrentTransfers(t *testing.T) {
	db := openTestDB(t)

	senderID := createTestAccount(
		t, db, "concurrent-src", 500_000,
	)

	recipientA := createTestAccount(
		t, db, "concurrent-a", 0,
	)

	recipientB := createTestAccount(
		t, db, "concurrent-b", 0,
	)

	processor := NewTransferProcessor(db)

	type attempt struct {
		err error
	}

	results := make(chan attempt, 2)
	start := make(chan struct{})

	var wg sync.WaitGroup

	for _, destination := range []int64{recipientA, recipientB} {
		wg.Add(1)

		go func(recipientID int64) {
			defer wg.Done()

			<-start

			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			_, err := processor.ExecuteTransfer(
				ctx,
				domain.TransferInput{
					FromAccountID: senderID,
					ToAccountID:   recipientID,
					Amount:        350_000,
				},
			)

			results <- attempt{err: err}
		}(destination)
	}

	close(start)

	wg.Wait()
	close(results)

	successes := 0
	insufficient := 0

	for result := range results {
		switch {
		case result.err == nil:
			successes++

		case errors.Is(
			result.err,
			domain.ErrInsufficientFunds,
		):
			insufficient++

		default:
			t.Errorf(
				"unexpected transfer error: %v",
				result.err,
			)
		}
	}

	if successes != 1 || insufficient != 1 {
		t.Errorf(
			"successes=%d insufficient=%d; want 1 each",
			successes,
			insufficient,
		)
	}

	senderBalance := readTestBalance(t, db, senderID)
	aBalance := readTestBalance(t, db, recipientA)
	bBalance := readTestBalance(t, db, recipientB)

	if senderBalance != 150_000 {
		t.Errorf(
			"sender balance = %d, want 150000",
			senderBalance,
		)
	}

	if !((aBalance == 350_000 && bBalance == 0) ||
		(aBalance == 0 && bBalance == 350_000)) {
		t.Errorf(
			"recipient balances = %d and %d; want 350000 and 0",
			aBalance,
			bBalance,
		)
	}

	if total := senderBalance + aBalance + bBalance; total != 500_000 {
		t.Errorf(
			"total balance = %d, want 500000",
			total,
		)
	}

	recorded := countTestTransfers(
		t, db, senderID, recipientA,
	) + countTestTransfers(
		t, db, senderID, recipientB,
	)

	if recorded != 1 {
		t.Errorf(
			"transfer records = %d, want 1",
			recorded,
		)
	}
}
