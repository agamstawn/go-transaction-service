package service

import (
	"context"

	"github.com/agam-vs/go-transaction-service/internal/domain"
)

// TransferResult represents a completed transfer.
type TransferResult struct {
	ID            int64  `json:"id"`
	FromAccountID int64  `json:"from_account_id"`
	ToAccountID   int64  `json:"to_account_id"`
	Amount        int64  `json:"amount"`
	Status        string `json:"status"`
	Replayed      bool   `json:"-"`
}

// TransferProcessor defines how a transfer is processed.
type TransferProcessor interface {
	ExecuteTransfer(
		ctx context.Context,
		input domain.TransferInput,
	) (TransferResult, error)
}

// TransferService handles transfer business operations.
type TransferService struct {
	processor TransferProcessor
}

// NewTransferService creates a new transfer service.
func NewTransferService(
	processor TransferProcessor,
) *TransferService {
	return &TransferService{
		processor: processor,
	}
}

// Transfer validates input and delegates transaction processing.
func (s *TransferService) Transfer(
	ctx context.Context,
	input domain.TransferInput,
) (TransferResult, error) {
	// Stop processing if the request is already cancelled.
	if err := ctx.Err(); err != nil {
		return TransferResult{}, err
	}

	// Validate transfer business rules.
	if err := input.Validate(); err != nil {
		return TransferResult{}, err
	}

	// Delegate transaction processing to the processor.
	return s.processor.ExecuteTransfer(ctx, input)
}
