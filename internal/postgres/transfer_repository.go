package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agamstawn/go-transaction-service/internal/service"
)

// TransferRepository handles transfer data retrieval.
type TransferRepository struct {
	db *sql.DB
}

// NewTransferRepository creates a PostgreSQL transfer repository.
func NewTransferRepository(db *sql.DB) *TransferRepository {
	return &TransferRepository{
		db: db,
	}
}

// GetByID retrieves a transfer visible to the authorized account.
func (r *TransferRepository) GetByID(
	ctx context.Context,
	transferID int64,
	authorizedAccountID int64,
) (service.TransferResult, error) {
	var transfer service.TransferResult

	query := `
		SELECT
			id,
			from_account_id,
			to_account_id,
			amount,
			status
		FROM transfers
		WHERE id = $1
		  AND (
			from_account_id = $2
			OR to_account_id = $2
		  )
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		transferID,
		authorizedAccountID,
	).Scan(
		&transfer.ID,
		&transfer.FromAccountID,
		&transfer.ToAccountID,
		&transfer.Amount,
		&transfer.Status,
	)

	if err != nil {
		return service.TransferResult{}, fmt.Errorf(
			"get transfer %d: %w",
			transferID,
			err,
		)
	}

	return transfer, nil
}
