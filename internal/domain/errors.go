package domain

import "errors"

var (
	ErrInvalidAccount = errors.New(
		"account IDs must be positive",
	)

	ErrSameAccount = errors.New(
		"source and destination accounts must be different",
	)

	ErrInvalidAmount = errors.New(
		"transfer amount must be positive",
	)

	ErrInvalidAccountNumber = errors.New(
		"account number is required",
	)

	ErrInvalidCurrency = errors.New(
		"unsupported account currency",
	)

	ErrNegativeBalance = errors.New(
		"account balance cannot be negative",
	)

	ErrInsufficientFunds = errors.New(
		"insufficient funds",
	)
)
