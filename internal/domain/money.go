package domain

import (
	"errors"
	"fmt"
)

type Money struct {
	amount   int64
	currency string
}

var (
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrNegativeAmount   = errors.New("amount must be non-negative")
	ErrZeroAmount       = errors.New("amount must be greater than zero")
)

func NewMoney(amount int64, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, ErrNegativeAmount
	}
	return Money{amount: amount, currency: currency}, nil
}

func MustNewMoney(amount int64, currency string) Money {
	m, err := NewMoney(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

func (m Money) Amount() int64    { return m.amount }
func (m Money) Currency() string { return m.currency }
func (m Money) IsZero() bool     { return m.amount == 0 }

func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	return Money{amount: m.amount + other.amount, currency: m.currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
	if m.amount < other.amount {
		return Money{}, fmt.Errorf("insufficient funds: have %d, need %d", m.amount, other.amount)
	}
	return Money{amount: m.amount - other.amount, currency: m.currency}, nil
}

func (m Money) Equal(other Money) bool {
	return m.currency == other.currency && m.amount == other.amount
}

func (m Money) GreaterThan(other Money) bool {
	return m.currency == other.currency && m.amount > other.amount
}

func (m Money) String() string {
	return fmt.Sprintf("%s %d", m.currency, m.amount)
}
