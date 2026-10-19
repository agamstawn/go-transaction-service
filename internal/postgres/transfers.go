package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agam-vs/go-transaction-service/internal/domain"
	"github.com/agam-vs/go-transaction-service/internal/service"
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

// ExecuteTransfer moves funds atomically between two accounts.
func (p *TransferProcessor) ExecuteTransfer(
	ctx context.Context,
	input domain.TransferInput,
) (service.TransferResult, error) {

	// 1. Validate the transfer input.
	if err := input.Validate(); err != nil {
		return service.TransferResult{}, err
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

	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO transfers (
			from_account_id,
			to_account_id,
			amount,
			status
		)
		VALUES ($1, $2, $3, 'completed')
		RETURNING id`,
		input.FromAccountID,
		input.ToAccountID,
		input.Amount,
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
