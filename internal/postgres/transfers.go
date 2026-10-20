package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/agamstawn/go-transaction-service/internal/domain"
	"github.com/agamstawn/go-transaction-service/internal/service"
)

// TransferProcessor processes transfers using PostgreSQL.
type TransferProcessor struct {
	db *sql.DB
}

// NewTransferProcessor creates a PostgreSQL transfer processor.
func NewTransferProcessor(db *sql.DB) *TransferProcessor {
	return &TransferProcessor{
		db: db,
	}
}

func idempotencyLockID(senderID int64, key string) int64 {
	value := fmt.Sprintf("%d:%s", senderID, key)

	hash := sha256.Sum256([]byte(value))

	return int64(binary.BigEndian.Uint64(hash[:8]))
}

// ExecuteTransfer moves funds atomically between two accounts.
func (p *TransferProcessor) ExecuteTransfer(
	ctx context.Context,
	input domain.TransferInput,
) (service.TransferResult, error) {

	// 1. Validate the transfer input.
	if err := input.Validate(); err != nil {
		return service.TransferResult{}, err
	}

	// Validate a supplied idempotency key.
	if input.IdempotencyKey != "" {
		if err := input.ValidateIdempotencyKey(); err != nil {
			return service.TransferResult{}, err
		}
	}

	// 2. Start a database transaction.
	tx, err := p.db.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
		},
	)

	if err != nil {
		return service.TransferResult{},
			fmt.Errorf("begin transfer: %w", err)
	}

	// Roll back unless the transaction is committed.
	defer tx.Rollback()

	// Handle idempotent transfer requests.
	if input.IdempotencyKey != "" {
		lockID := idempotencyLockID(
			input.FromAccountID,
			input.IdempotencyKey,
		)

		// Serialize requests using the same sender and key.
		_, err := tx.ExecContext(
			ctx,
			"SELECT pg_advisory_xact_lock($1::bigint)",
			lockID,
		)

		if err != nil {
			return service.TransferResult{},
				fmt.Errorf("acquire idempotency lock: %w", err)
		}

		// Check whether this transfer was already processed.
		var existing service.TransferResult

		err = tx.QueryRowContext(
			ctx,
			`SELECT
            	id,
            	from_account_id,
            	to_account_id,
            	amount,
            	status
         	FROM transfers
         	WHERE from_account_id = $1
           	AND idempotency_key = $2`,
			input.FromAccountID,
			input.IdempotencyKey,
		).Scan(
			&existing.ID,
			&existing.FromAccountID,
			&existing.ToAccountID,
			&existing.Amount,
			&existing.Status,
		)

		switch {
		case err == nil:
			// The key exists. Verify that the payload matches.
			if existing.FromAccountID != input.FromAccountID ||
				existing.ToAccountID != input.ToAccountID ||
				existing.Amount != input.Amount {

				return service.TransferResult{},
					domain.ErrIdempotencyConflict
			}

			// The existing transfer was already committed.
			// No balance update is necessary.
			if err := tx.Commit(); err != nil {
				return service.TransferResult{},
					fmt.Errorf("commit idempotent replay: %w", err)
			}

			existing.Replayed = true

			return existing, nil

		case errors.Is(err, sql.ErrNoRows):
			// No existing transfer. Continue normal processing.

		default:
			return service.TransferResult{},
				fmt.Errorf("check existing transfer: %w", err)
		}
	}

	// 3. Determine a consistent account locking order.
	firstID := input.FromAccountID
	secondID := input.ToAccountID

	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	// 4. Lock both accounts and read their balances.
	balances := make(map[int64]int64, 2)

	for _, accountID := range []int64{firstID, secondID} {
		var balance int64

		err := tx.QueryRowContext(
			ctx,
			`SELECT balance
			 FROM accounts
			 WHERE id = $1
			 FOR UPDATE`,
			accountID,
		).Scan(&balance)

		if err != nil {
			return service.TransferResult{},
				fmt.Errorf(
					"lock account %d: %w",
					accountID,
					err,
				)
		}

		balances[accountID] = balance
	}

	// 5. Check the sender's balance.
	senderBalance := balances[input.FromAccountID]

	if senderBalance < input.Amount {
		return service.TransferResult{},
			domain.ErrInsufficientFunds
	}

	// 6. Debit the sender.
	_, err = tx.ExecContext(
		ctx,
		`UPDATE accounts
		 SET balance = balance - $1,
		     updated_at = NOW()
		 WHERE id = $2`,
		input.Amount,
		input.FromAccountID,
	)

	if err != nil {
		return service.TransferResult{},
			fmt.Errorf("debit sender: %w", err)
	}

	// 7. Credit the recipient.
	_, err = tx.ExecContext(
		ctx,
		`UPDATE accounts
		 SET balance = balance + $1,
		     updated_at = NOW()
		 WHERE id = $2`,
		input.Amount,
		input.ToAccountID,
	)

	if err != nil {
		return service.TransferResult{},
			fmt.Errorf("credit recipient: %w", err)
	}

	// 8. Record the completed transfer.
	var transferID int64

	idempotencyKey := sql.NullString{
		String: input.IdempotencyKey,
		Valid:  input.IdempotencyKey != "",
	}

	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO transfers (
        	from_account_id,
        	to_account_id,
        	amount,
        	status,
        	idempotency_key
    	)
    	VALUES ($1, $2, $3, 'completed', $4)
    	RETURNING id`,
		input.FromAccountID,
		input.ToAccountID,
		input.Amount,
		idempotencyKey,
	).Scan(&transferID)

	if err != nil {
		return service.TransferResult{},
			fmt.Errorf("record transfer: %w", err)
	}

	// 9. Commit all changes together.
	if err := tx.Commit(); err != nil {
		return service.TransferResult{},
			fmt.Errorf("commit transfer: %w", err)
	}

	return service.TransferResult{
		ID:            transferID,
		FromAccountID: input.FromAccountID,
		ToAccountID:   input.ToAccountID,
		Amount:        input.Amount,
		Status:        "completed",
	}, nil
}
