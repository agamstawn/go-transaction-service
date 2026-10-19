package domain

import (
	"errors"
	"testing"
)

func TestAccountValidate(t *testing.T) {
	tests := []struct {
		name     string
		id       int64
		number   string
		currency string
		balance  int64
		wantErr  error
	}{
		{
			name:     "valid account",
			id:       1,
			number:   "10000001",
			currency: "IDR",
			balance:  500_000,
		},
		{
			name:     "zero balance is allowed",
			id:       1,
			number:   "10000001",
			currency: "IDR",
			balance:  0,
		},
		{
			name:     "invalid account ID",
			id:       0,
			number:   "10000001",
			currency: "IDR",
			balance:  500_000,
			wantErr:  ErrInvalidAccount,
		},
		{
			name:     "empty account number",
			id:       1,
			number:   "",
			currency: "IDR",
			balance:  500_000,
			wantErr:  ErrInvalidAccountNumber,
		},
		{
			name:     "whitespace account number",
			id:       1,
			number:   "   ",
			currency: "IDR",
			balance:  500_000,
			wantErr:  ErrInvalidAccountNumber,
		},
		{
			name:     "unsupported currency",
			id:       1,
			number:   "10000001",
			currency: "USD",
			balance:  500_000,
			wantErr:  ErrInvalidCurrency,
		},
		{
			name:     "negative balance",
			id:       1,
			number:   "10000001",
			currency: "IDR",
			balance:  -10_000,
			wantErr:  ErrNegativeBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := Account{
				ID:       tt.id,
				Number:   tt.number,
				Currency: tt.currency,
				Balance:  tt.balance,
			}

			err := account.Validate()

			if !errors.Is(err, tt.wantErr) {
				t.Errorf(
					"Validate() error = %v, want %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
