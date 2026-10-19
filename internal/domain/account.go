package domain

import "strings"

type Account struct {
	ID       int64  `json:"id"`
	Number   string `json:"number"`
	Currency string `json:"currency"`
	Balance  int64  `json:"balance"`
}

func (a Account) Validate() error {
	if a.ID <= 0 {
		return ErrInvalidAccount
	}

	if strings.TrimSpace(a.Number) == "" {
		return ErrInvalidAccountNumber
	}

	if a.Currency != "IDR" {
		return ErrInvalidCurrency
	}

	if a.Balance < 0 {
		return ErrNegativeBalance
	}

	return nil
}
