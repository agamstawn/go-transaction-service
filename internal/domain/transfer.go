package domain

type TransferInput struct {
	FromAccountID  int64  `json:"from_account_id"`
	ToAccountID    int64  `json:"to_account_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"-"`
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

// ValidateIdempotencyKey validates the transfer idempotency key.
func (t TransferInput) ValidateIdempotencyKey() error {
	key := t.IdempotencyKey

	if len(key) == 0 || len(key) > 128 {
		return ErrInvalidIdempotencyKey
	}

	for _, char := range key {
		valid := (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' ||
			char == '_' ||
			char == '.' ||
			char == ':'

		if !valid {
			return ErrInvalidIdempotencyKey
		}
	}

	return nil
}
