package domain_test

import (
	"testing"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/google/uuid"
)

func validEntries(currency string, amount int64) []domain.Entry {
	money := domain.MustNewMoney(amount, currency)
	return []domain.Entry{
		{AccountID: uuid.New(), Amount: money, Type: domain.EntryDebit},
		{AccountID: uuid.New(), Amount: money, Type: domain.EntryCredit},
	}
}

func TestNewTransaction_Valid(t *testing.T) {
	tx, err := domain.NewTransaction("ref-001", validEntries("USD", 1000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Status != domain.StatusPending {
		t.Fatalf("want pending got %s", tx.Status)
	}
	if len(tx.Entries) != 2 {
		t.Fatalf("want 2 entries got %d", len(tx.Entries))
	}
}

func TestNewTransaction_TooFewEntries(t *testing.T) {
	_, err := domain.NewTransaction("ref-002", []domain.Entry{
		{AccountID: uuid.New(), Amount: domain.MustNewMoney(100, "USD"), Type: domain.EntryDebit},
	})
	if err != domain.ErrEmptyEntries {
		t.Fatalf("want ErrEmptyEntries got %v", err)
	}
}

func TestNewTransaction_ImbalancedEntries(t *testing.T) {
	entries := []domain.Entry{
		{AccountID: uuid.New(), Amount: domain.MustNewMoney(1000, "USD"), Type: domain.EntryDebit},
		{AccountID: uuid.New(), Amount: domain.MustNewMoney(500, "USD"), Type: domain.EntryCredit},
	}
	_, err := domain.NewTransaction("ref-003", entries)
	if err != domain.ErrInvalidEntries {
		t.Fatalf("want ErrInvalidEntries got %v", err)
	}
}

func TestTransaction_Commit(t *testing.T) {
	tx, _ := domain.NewTransaction("ref-004", validEntries("USD", 500))
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if tx.Status != domain.StatusCommitted {
		t.Fatalf("want committed got %s", tx.Status)
	}
	// Double-commit must fail.
	if err := tx.Commit(); err != domain.ErrTransactionNotPending {
		t.Fatalf("want ErrTransactionNotPending got %v", err)
	}
}

func TestTransaction_Fail(t *testing.T) {
	tx, _ := domain.NewTransaction("ref-005", validEntries("USD", 200))
	if err := tx.Fail(); err != nil {
		t.Fatal(err)
	}
	if tx.Status != domain.StatusFailed {
		t.Fatalf("want failed got %s", tx.Status)
	}
}

func TestTransaction_DebitCreditEntries(t *testing.T) {
	tx, _ := domain.NewTransaction("ref-006", validEntries("USD", 300))
	if len(tx.DebitEntries()) != 1 {
		t.Fatal("want 1 debit entry")
	}
	if len(tx.CreditEntries()) != 1 {
		t.Fatal("want 1 credit entry")
	}
}
