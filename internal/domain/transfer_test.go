package domain

import (
	"errors"
	"strings"
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

func TestTransferInputValidateIdempotencyKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{
			name: "valid key",
			key:  "transfer-001",
		},
		{
			name: "valid UUID",
			key:  "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "maximum length",
			key:  strings.Repeat("a", 128),
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name:    "whitespace only",
			key:     "   ",
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name:    "leading whitespace",
			key:     " transfer-001",
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name:    "trailing whitespace",
			key:     "transfer-001 ",
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name:    "exceeds maximum length",
			key:     strings.Repeat("a", 129),
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name: "valid separators",
			key:  "retry_01:phase.2",
		},
		{
			name:    "embedded whitespace",
			key:     "transfer 001",
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name:    "non ASCII character",
			key:     "transfer-é",
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name:    "control character",
			key:     "transfer-\n001",
			wantErr: ErrInvalidIdempotencyKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := TransferInput{
				IdempotencyKey: tt.key,
			}

			err := input.ValidateIdempotencyKey()

			if !errors.Is(err, tt.wantErr) {
				t.Errorf(
					"ValidateIdempotencyKey() error = %v, want %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
