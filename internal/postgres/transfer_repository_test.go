//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/agam-vs/go-transaction-service/internal/domain"
)

func TestTransferRepositoryGetByID(t *testing.T) {
	db := openTestDB(t)

	senderID := createTestAccount(
		t, db, "history-src", 500_000,
	)

	recipientID := createTestAccount(
		t, db, "history-dst", 200_000,
	)

	unrelatedID := createTestAccount(
		t, db, "history-other", 0,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	// Create a completed transfer.
	processor := NewTransferProcessor(db)

	created, err := processor.ExecuteTransfer(
		ctx,
		domain.TransferInput{
			FromAccountID:  senderID,
			ToAccountID:    recipientID,
			Amount:         125_000,
			IdempotencyKey: "history-test-001",
		},
	)

	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	repository := NewTransferRepository(db)

	t.Run("sender can view transfer", func(t *testing.T) {
		transfer, err := repository.GetByID(
			ctx,
			created.ID,
			senderID,
		)

		if err != nil {
			t.Fatalf("GetByID() error: %v", err)
		}

		if transfer.ID != created.ID {
			t.Errorf(
				"ID = %d, want %d",
				transfer.ID,
				created.ID,
			)
		}

		if transfer.Amount != 125_000 {
			t.Errorf(
				"Amount = %d, want 125000",
				transfer.Amount,
			)
		}

		if transfer.Status != "completed" {
			t.Errorf(
				"Status = %s, want completed",
				transfer.Status,
			)
		}
	})

	t.Run("recipient can view transfer", func(t *testing.T) {
		transfer, err := repository.GetByID(
			ctx,
			created.ID,
			recipientID,
		)

		if err != nil {
			t.Fatalf("GetByID() error: %v", err)
		}

		if transfer.ID != created.ID {
			t.Errorf(
				"ID = %d, want %d",
				transfer.ID,
				created.ID,
			)
		}
	})

	t.Run("unrelated account cannot view transfer", func(t *testing.T) {
		_, err := repository.GetByID(
			ctx,
			created.ID,
			unrelatedID,
		)

		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf(
				"error = %v, want sql.ErrNoRows",
				err,
			)
		}
	})

	t.Run("unknown transfer returns not found", func(t *testing.T) {
		_, err := repository.GetByID(
			ctx,
			-1,
			senderID,
		)

		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf(
				"error = %v, want sql.ErrNoRows",
				err,
			)
		}
	})
}
