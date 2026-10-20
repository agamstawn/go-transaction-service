package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"github.com/agam-vs/go-transaction-service/internal/service"
)

const maxHistoryPageSize = 100

// TransferPage represents a paginated transfer history result.
type TransferPage struct {
	Data       []service.TransferResult
	Page       int
	PerPage    int
	Total      int64
	TotalPages int64
}

// ListByAccount returns transfers involving the authorized account.
func (r *TransferRepository) ListByAccount(
	ctx context.Context,
	authorizedAccountID int64,
	page int,
	perPage int,
) (TransferPage, error) {

	// 1. Validate the account scope.
	if authorizedAccountID <= 0 {
		return TransferPage{}, fmt.Errorf(
			"invalid authorized account ID",
		)
	}

	// 2. Validate pagination parameters.
	if page < 1 || perPage < 1 || perPage > maxHistoryPageSize {
		return TransferPage{}, fmt.Errorf(
			"invalid pagination parameters",
		)
	}

	// 3. Calculate OFFSET safely.
	if int64(page-1) > math.MaxInt64/int64(perPage) {
		return TransferPage{}, fmt.Errorf(
			"pagination offset overflow",
		)
	}

	offset := int64(page-1) * int64(perPage)

	// 4. Start a read-only transaction for a consistent snapshot.
	tx, err := r.db.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelRepeatableRead,
			ReadOnly:  true,
		},
	)

	if err != nil {
		return TransferPage{}, fmt.Errorf(
			"begin history query: %w",
			err,
		)
	}

	defer tx.Rollback()

	// 5. Count transfers visible to this account.
	var total int64

	err = tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM transfers
		 WHERE from_account_id = $1
		    OR to_account_id = $1`,
		authorizedAccountID,
	).Scan(&total)

	if err != nil {
		return TransferPage{}, fmt.Errorf(
			"count transfers: %w",
			err,
		)
	}

	// 6. Retrieve the requested page.
	rows, err := tx.QueryContext(
		ctx,
		`SELECT
			id,
			from_account_id,
			to_account_id,
			amount,
			status
		 FROM transfers
		 WHERE from_account_id = $1
		    OR to_account_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		authorizedAccountID,
		perPage,
		offset,
	)

	if err != nil {
		return TransferPage{}, fmt.Errorf(
			"list transfers: %w",
			err,
		)
	}

	// 7. Convert SQL rows into Go structs.
	transfers := make([]service.TransferResult, 0, perPage)

	for rows.Next() {
		var transfer service.TransferResult

		err := rows.Scan(
			&transfer.ID,
			&transfer.FromAccountID,
			&transfer.ToAccountID,
			&transfer.Amount,
			&transfer.Status,
		)

		if err != nil {
			_ = rows.Close()

			return TransferPage{}, fmt.Errorf(
				"scan transfer: %w",
				err,
			)
		}

		transfers = append(transfers, transfer)
	}

	if err := rows.Err(); err != nil {
		_ = rows.Close()

		return TransferPage{}, fmt.Errorf(
			"iterate transfers: %w",
			err,
		)
	}

	if err := rows.Close(); err != nil {
		return TransferPage{}, fmt.Errorf(
			"close transfer rows: %w",
			err,
		)
	}

	// 8. Complete the read-only transaction.
	if err := tx.Commit(); err != nil {
		return TransferPage{}, fmt.Errorf(
			"commit history query: %w",
			err,
		)
	}

	// 9. Calculate pagination metadata.
	totalPages := total / int64(perPage)

	if total%int64(perPage) != 0 {
		totalPages++
	}

	return TransferPage{
		Data:       transfers,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}
