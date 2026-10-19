//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")

	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}

	if u.Path != "/transactions_test" {
		t.Fatal("integration tests require transactions_test database")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
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

	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	var databaseName string

	err = db.QueryRowContext(
		ctx,
		"SELECT current_database()",
	).Scan(&databaseName)

	if err != nil {
		t.Fatal(err)
	}

	if databaseName != "transactions_test" {
		t.Fatalf("unexpected database: %s", databaseName)
	}

	return db
}

func TestAccountRepositoryGetByID(t *testing.T) {
	db := openTestDB(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	accountNumber := fmt.Sprintf(
		"T%d",
		time.Now().UnixNano(),
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
		500_000,
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

		_, err := db.ExecContext(
			cleanupCtx,
			"DELETE FROM accounts WHERE id = $1",
			id,
		)

		if err != nil {
			t.Errorf("cleanup account: %v", err)
		}
	})

	repository := NewAccountRepository(db)

	account, err := repository.GetByID(ctx, id)

	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if account.ID != id {
		t.Errorf("ID = %d, want %d", account.ID, id)
	}

	if account.Number != accountNumber {
		t.Errorf(
			"Number = %s, want %s",
			account.Number,
			accountNumber,
		)
	}

	if account.Currency != "IDR" {
		t.Errorf(
			"Currency = %s, want IDR",
			account.Currency,
		)
	}

	if account.Balance != 500_000 {
		t.Errorf(
			"Balance = %d, want 500000",
			account.Balance,
		)
	}
}

func TestAccountRepositoryGetByIDNotFound(t *testing.T) {
	db := openTestDB(t)

	repository := NewAccountRepository(db)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	_, err := repository.GetByID(ctx, -1)

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf(
			"GetByID() error = %v, want sql.ErrNoRows",
			err,
		)
	}
}
