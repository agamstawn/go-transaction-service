package domain

import (
	"errors"
	"testing"
)

func TestTransferInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   TransferInput
		wantErr error
	}{
		{
			name: "valid transfer",
			input: TransferInput{
				FromAccountID: 1,
				ToAccountID:   2,
				Amount:        125000,
			},
			wantErr: nil,
		},
		{
			name: "invalid source account",
			input: TransferInput{
				FromAccountID: 0,
				ToAccountID:   2,
				Amount:        125000,
			},
			wantErr: ErrInvalidAccount,
		},
		{
			name: "invalid destination account",
			input: TransferInput{
				FromAccountID: 1,
				ToAccountID:   0,
				Amount:        125000,
			},
			wantErr: ErrInvalidAccount,
		},
		{
			name: "same source and destination",
			input: TransferInput{
				FromAccountID: 1,
				ToAccountID:   1,
				Amount:        125000,
			},
			wantErr: ErrSameAccount,
		},
		{
			name: "zero amount",
			input: TransferInput{
				FromAccountID: 1,
				ToAccountID:   2,
				Amount:        0,
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "negative amount",
			input: TransferInput{
				FromAccountID: 1,
				ToAccountID:   2,
				Amount:        -10000,
			},
			wantErr: ErrInvalidAmount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

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
