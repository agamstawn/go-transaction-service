package domain

type TransferInput struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}

func (t TransferInput) Validate() error {
	if t.FromAccountID <= 0 || t.ToAccountID <= 0 {
		return ErrInvalidAccount
	}

	if t.FromAccountID == t.ToAccountID {
		return ErrSameAccount
	}

	if t.Amount <= 0 {
		return ErrInvalidAmount
	}

	return nil
}
