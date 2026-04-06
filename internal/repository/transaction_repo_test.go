package repository_test

import (
	"context"
	"testing"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/ezkahan/fintech-ledger/internal/testhelper"
	"github.com/google/uuid"
)

func seedAccounts(t *testing.T, repo repository.AccountRepository, currencies ...string) []domain.Account {
	t.Helper()
	var accounts []domain.Account
	for _, cur := range currencies {
		acc := domain.NewAccount(uuid.New(), cur) // unique user per account to avoid duplicate-currency constraint
		if err := repo.Create(context.Background(), acc); err != nil {
			t.Fatal(err)
		}
		accounts = append(accounts, acc)
	}
	return accounts
}

func buildTx(t *testing.T, refID string, from, to uuid.UUID, amount int64, cur string) domain.Transaction {
	t.Helper()
	money := domain.MustNewMoney(amount, cur)
	entries := []domain.Entry{
		{AccountID: from, Amount: money, Type: domain.EntryDebit},
		{AccountID: to, Amount: money, Type: domain.EntryCredit},
	}
	tx, err := domain.NewTransaction(refID, entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestTransactionRepo_CreateAndGetByID(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)

	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)

	accounts := seedAccounts(t, accRepo, "USD", "USD")
	tx := buildTx(t, "ref-tx-001", accounts[0].ID, accounts[1].ID, 500, "USD")

	if err := txRepo.Create(context.Background(), tx); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := txRepo.GetByID(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != tx.ID {
		t.Fatal("ID mismatch")
	}
	if got.Status != domain.StatusCommitted {
		t.Fatalf("want committed got %s", got.Status)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("want 2 entries got %d", len(got.Entries))
	}
}

func TestTransactionRepo_GetByReferenceID(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)

	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)

	accounts := seedAccounts(t, accRepo, "EUR", "EUR")
	tx := buildTx(t, "ref-tx-002", accounts[0].ID, accounts[1].ID, 200, "EUR")

	if err := txRepo.Create(context.Background(), tx); err != nil {
		t.Fatal(err)
	}

	got, err := txRepo.GetByReferenceID(context.Background(), "ref-tx-002")
	if err != nil {
		t.Fatalf("GetByReferenceID: %v", err)
	}
	if got.ReferenceID != "ref-tx-002" {
		t.Fatalf("wrong reference: %s", got.ReferenceID)
	}
}

func TestTransactionRepo_DuplicateReference(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)

	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)

	accounts := seedAccounts(t, accRepo, "USD", "USD")
	tx1 := buildTx(t, "ref-dup", accounts[0].ID, accounts[1].ID, 100, "USD")
	tx2 := buildTx(t, "ref-dup", accounts[0].ID, accounts[1].ID, 100, "USD")

	if err := txRepo.Create(context.Background(), tx1); err != nil {
		t.Fatal(err)
	}
	err := txRepo.Create(context.Background(), tx2)
	if err != domain.ErrDuplicateReference {
		t.Fatalf("want ErrDuplicateReference got %v", err)
	}
}

func TestTransactionRepo_NotFound(t *testing.T) {
	pool := testhelper.NewPool(t)
	txRepo := repository.NewTransactionRepository(pool)

	_, err := txRepo.GetByID(context.Background(), uuid.New())
	if err != domain.ErrTransactionNotFound {
		t.Fatalf("want ErrTransactionNotFound got %v", err)
	}
}

func TestTransactionRepo_OutboxEventWritten(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)

	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)

	accounts := seedAccounts(t, accRepo, "USD", "USD")
	tx := buildTx(t, "ref-outbox-01", accounts[0].ID, accounts[1].ID, 750, "USD")

	if err := txRepo.Create(context.Background(), tx); err != nil {
		t.Fatal(err)
	}

	events, err := outboxRepo.GetUnprocessed(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 outbox event got %d", len(events))
	}
	if events[0].EventType != "transaction.committed" {
		t.Fatalf("unexpected event type: %s", events[0].EventType)
	}
}
