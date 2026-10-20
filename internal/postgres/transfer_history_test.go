//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agam-vs/go-transaction-service/internal/domain"
)

func TestTransferRepositoryListByAccount(t *testing.T) {
	db := openTestDB(t)

	// Create accounts for history testing.
	senderID := createTestAccount(
		t, db, "list-src", 500_000,
	)

	recipientID := createTestAccount(
		t, db, "list-dst", 0,
	)

	// Create unrelated accounts to test authorization.
	otherSenderID := createTestAccount(
		t, db, "other-src", 500_000,
	)

	otherRecipientID := createTestAccount(
		t, db, "other-dst", 0,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	processor := NewTransferProcessor(db)

	var createdIDs []int64

	// Create three transfers for the authorized account.
	for i := 0; i < 3; i++ {
		result, err := processor.ExecuteTransfer(
			ctx,
			domain.TransferInput{
				FromAccountID: senderID,
				ToAccountID:   recipientID,
				Amount:        int64(1000 + i),
				IdempotencyKey: fmt.Sprintf(
					"history-list-%d",
					i,
				),
			},
		)

		if err != nil {
			t.Fatalf(
				"create transfer %d: %v",
				i,
				err,
			)
		}

		createdIDs = append(createdIDs, result.ID)
	}

	// Create a transfer unrelated to the authorized account.
	_, err := processor.ExecuteTransfer(
		ctx,
		domain.TransferInput{
			FromAccountID:  otherSenderID,
			ToAccountID:    otherRecipientID,
			Amount:         5000,
			IdempotencyKey: "unrelated-history",
		},
	)

	if err != nil {
		t.Fatalf("create unrelated transfer: %v", err)
	}

	repository := NewTransferRepository(db)

	t.Run("first page", func(t *testing.T) {
		page, err := repository.ListByAccount(
			ctx,
			senderID,
			1,
			2,
		)

		if err != nil {
			t.Fatal(err)
		}

		if page.Total != 3 {
			t.Errorf(
				"total = %d, want 3",
				page.Total,
			)
		}

		if len(page.Data) != 2 {
			t.Fatalf(
				"items = %d, want 2",
				len(page.Data),
			)
		}

		if page.Data[0].ID != createdIDs[2] ||
			page.Data[1].ID != createdIDs[1] {
			t.Errorf(
				"unexpected first page ordering",
			)
		}
	})

	t.Run("second page", func(t *testing.T) {
		page, err := repository.ListByAccount(
			ctx,
			senderID,
			2,
			2,
		)

		if err != nil {
			t.Fatal(err)
		}

		if page.Total != 3 {
			t.Errorf(
				"total = %d, want 3",
				page.Total,
			)
		}

		if len(page.Data) != 1 {
			t.Fatalf(
				"items = %d, want 1",
				len(page.Data),
			)
		}

		if page.Data[0].ID != createdIDs[0] {
			t.Errorf(
				"unexpected second page result",
			)
		}
	})

	t.Run("unrelated transfers excluded", func(t *testing.T) {
		page, err := repository.ListByAccount(
			ctx,
			senderID,
			1,
			10,
		)

		if err != nil {
			t.Fatal(err)
		}

		if page.Total != 3 {
			t.Errorf(
				"total = %d, want 3",
				page.Total,
			)
		}

		if len(page.Data) != 3 {
			t.Errorf(
				"items = %d, want 3",
				len(page.Data),
			)
		}
	})
}

func TestTransferRepositoryRejectsInvalidPagination(t *testing.T) {
	db := openTestDB(t)

	repository := NewTransferRepository(db)

	tests := []struct {
		name      string
		accountID int64
		page      int
		perPage   int
	}{
		{
			name:      "invalid account scope",
			accountID: 0,
			page:      1,
			perPage:   10,
		},
		{
			name:      "zero page",
			accountID: 1,
			page:      0,
			perPage:   10,
		},
		{
			name:      "negative page",
			accountID: 1,
			page:      -1,
			perPage:   10,
		},
		{
			name:      "zero page size",
			accountID: 1,
			page:      1,
			perPage:   0,
		},
		{
			name:      "page size exceeds limit",
			accountID: 1,
			page:      1,
			perPage:   101,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repository.ListByAccount(
				context.Background(),
				tt.accountID,
				tt.page,
				tt.perPage,
			)

			if err == nil {
				t.Fatal("expected pagination validation error")
			}
		})
	}
}
