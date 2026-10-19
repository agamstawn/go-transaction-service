package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agam-vs/go-transaction-service/internal/domain"
)

// AccountRepository handles account database operations.
type AccountRepository struct {
	db *sql.DB
}

// NewAccountRepository creates a PostgreSQL account repository.
func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{
		db: db,
	}
}

// GetByID retrieves an account by its ID.
func (r *AccountRepository) GetByID(
	ctx context.Context,
	id int64,
) (domain.Account, error) {
	var account domain.Account

	query := `
		SELECT
			id,
			account_number,
			currency,
			balance
		FROM accounts
		WHERE id = $1
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		id,
	).Scan(
		&account.ID,
		&account.Number,
		&account.Currency,
		&account.Balance,
	)

	if err != nil {
		return domain.Account{}, fmt.Errorf(
			"get account %d: %w",
			id,
			err,
		)
	}

	return account, nil
}
