package service

import (
	"context"
	"errors"
	"testing"

	"github.com/agamstawn/go-transaction-service/internal/domain"
)

type fakeTransferProcessor struct {
	calls           int
	receivedContext context.Context
	receivedInput   domain.TransferInput
	result          TransferResult
	err             error
}

func (f *fakeTransferProcessor) ExecuteTransfer(
	ctx context.Context,
	input domain.TransferInput,
) (TransferResult, error) {
	f.calls++
	f.receivedContext = ctx
	f.receivedInput = input

	return f.result, f.err
}

func TestTransferServiceSuccess(t *testing.T) {
	input := domain.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		Amount:        125_000,
	}

	expected := TransferResult{
		ID:            100,
		FromAccountID: 1,
		ToAccountID:   2,
		Amount:        125_000,
		Status:        "completed",
	}

	processor := &fakeTransferProcessor{
		result: expected,
	}

	service := NewTransferService(processor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := service.Transfer(ctx, input)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("result = %+v, want %+v", result, expected)
	}

	if processor.calls != 1 {
		t.Errorf("processor called %d times, want 1", processor.calls)
	}

	if processor.receivedInput != input {
		t.Error("processor received unexpected input")
	}

	if processor.receivedContext != ctx {
		t.Error("processor did not receive the request context")
	}
}

func TestTransferServiceRejectsInvalidInput(t *testing.T) {
	processor := &fakeTransferProcessor{}
	service := NewTransferService(processor)

	input := domain.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		Amount:        -10_000,
	}

	_, err := service.Transfer(context.Background(), input)

	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("error = %v, want ErrInvalidAmount", err)
	}

	if processor.calls != 0 {
		t.Errorf("processor called %d times, want 0", processor.calls)
	}
}

func TestTransferServicePropagatesProcessorError(t *testing.T) {
	expectedErr := errors.New("transaction processing failed")

	processor := &fakeTransferProcessor{
		err: expectedErr,
	}

	service := NewTransferService(processor)

	input := domain.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		Amount:        125_000,
	}

	_, err := service.Transfer(context.Background(), input)

	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}

	if processor.calls != 1 {
		t.Errorf("processor called %d times, want 1", processor.calls)
	}
}

func TestTransferServiceRejectsCancelledContext(t *testing.T) {
	processor := &fakeTransferProcessor{}
	service := NewTransferService(processor)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input := domain.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		Amount:        125_000,
	}

	_, err := service.Transfer(ctx, input)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}

	if processor.calls != 0 {
		t.Errorf("processor called %d times, want 0", processor.calls)
	}
}
