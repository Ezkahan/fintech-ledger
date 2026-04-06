package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrDuplicateAccount     = errors.New("account already exists for this currency")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrCurrencyNotSupported = errors.New("currency not supported")
)

type Account struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Currency  string
	CreatedAt time.Time
}

func NewAccount(userID uuid.UUID, currency string) Account {
	return Account{
		ID:        uuid.New(),
		UserID:    userID,
		Currency:  currency,
		CreatedAt: time.Now().UTC(),
	}
}
