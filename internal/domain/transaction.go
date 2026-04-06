package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusCommitted TransactionStatus = "committed"
	StatusFailed    TransactionStatus = "failed"
)

type EntryType string

const (
	EntryDebit  EntryType = "debit"
	EntryCredit EntryType = "credit"
)

var (
	ErrTransactionNotFound   = errors.New("transaction not found")
	ErrDuplicateReference    = errors.New("transaction with this reference_id already exists")
	ErrInvalidEntries        = errors.New("entries do not balance: sum(debits) must equal sum(credits)")
	ErrTransactionNotPending = errors.New("transaction is not in pending state")
	ErrEmptyEntries          = errors.New("transaction must have at least two entries")
)

type Transaction struct {
	ID          uuid.UUID
	ReferenceID string
	Status      TransactionStatus
	CreatedAt   time.Time
	Entries     []Entry
}

type Entry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	Amount        Money
	Type          EntryType
	CreatedAt     time.Time
}

func NewTransaction(referenceID string, entries []Entry) (Transaction, error) {
	if len(entries) < 2 {
		return Transaction{}, ErrEmptyEntries
	}

	txID := uuid.New()
	now := time.Now().UTC()

	for i := range entries {
		entries[i].ID = uuid.New()
		entries[i].TransactionID = txID
		entries[i].CreatedAt = now
	}

	t := Transaction{
		ID:          txID,
		ReferenceID: referenceID,
		Status:      StatusPending,
		CreatedAt:   now,
		Entries:     entries,
	}

	if err := t.validate(); err != nil {
		return Transaction{}, err
	}

	return t, nil
}

func (t Transaction) validate() error {
	if len(t.Entries) == 0 {
		return ErrEmptyEntries
	}

	var debits, credits int64
	for _, e := range t.Entries {
		switch e.Type {
		case EntryDebit:
			debits += e.Amount.Amount()
		case EntryCredit:
			credits += e.Amount.Amount()
		}
	}

	if debits != credits {
		return ErrInvalidEntries
	}

	return nil
}

func (t *Transaction) Commit() error {
	if t.Status != StatusPending {
		return ErrTransactionNotPending
	}
	t.Status = StatusCommitted
	return nil
}

func (t *Transaction) Fail() error {
	if t.Status != StatusPending {
		return ErrTransactionNotPending
	}
	t.Status = StatusFailed
	return nil
}

func (t Transaction) DebitEntries() []Entry {
	return t.entriesByType(EntryDebit)
}

func (t Transaction) CreditEntries() []Entry {
	return t.entriesByType(EntryCredit)
}

func (t Transaction) entriesByType(et EntryType) []Entry {
	out := make([]Entry, 0, len(t.Entries))
	for _, e := range t.Entries {
		if e.Type == et {
			out = append(out, e)
		}
	}
	return out
}
