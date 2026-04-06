package service_test

import (
	"context"
	"testing"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/ezkahan/fintech-ledger/internal/service"
	"github.com/ezkahan/fintech-ledger/internal/testhelper"
	"github.com/google/uuid"
)

// creditAccount directly inserts a committed credit entry to fund an account.
func creditAccount(t *testing.T, accRepo repository.AccountRepository, txRepo repository.TransactionRepository, accountID uuid.UUID, amount int64, currency string) {
	t.Helper()
	// Use a bank/float account as the source.
	bankAcc := domain.NewAccount(uuid.New(), currency)
	if err := accRepo.Create(context.Background(), bankAcc); err != nil {
		t.Fatal(err)
	}
	money := domain.MustNewMoney(amount, currency)
	entries := []domain.Entry{
		{AccountID: bankAcc.ID, Amount: money, Type: domain.EntryDebit},
		{AccountID: accountID, Amount: money, Type: domain.EntryCredit},
	}
	tx, err := domain.NewTransaction(uuid.NewString(), entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := txRepo.Create(context.Background(), tx); err != nil {
		t.Fatal(err)
	}
}

func TestTransferService_Transfer_Success(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)
	transferSvc := service.NewTransferService(accRepo, txRepo)

	userA := uuid.New()
	userB := uuid.New()
	accA := domain.NewAccount(userA, "USD")
	accB := domain.NewAccount(userB, "USD")
	if err := accRepo.Create(context.Background(), accA); err != nil {
		t.Fatal(err)
	}
	if err := accRepo.Create(context.Background(), accB); err != nil {
		t.Fatal(err)
	}

	// Fund account A with 10 000 cents ($100).
	creditAccount(t, accRepo, txRepo, accA.ID, 10_000, "USD")

	tx, err := transferSvc.Transfer(context.Background(), service.TransferRequest{
		ReferenceID:   "pay-001",
		FromAccountID: accA.ID,
		ToAccountID:   accB.ID,
		Amount:        3_000,
		Currency:      "USD",
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if tx.Status != domain.StatusCommitted {
		t.Fatalf("want committed got %s", tx.Status)
	}

	// Verify balances.
	balA, _ := accRepo.GetBalance(context.Background(), accA.ID)
	balB, _ := accRepo.GetBalance(context.Background(), accB.ID)

	if balA.Amount() != 7_000 {
		t.Fatalf("A: want 7000 got %d", balA.Amount())
	}
	if balB.Amount() != 3_000 {
		t.Fatalf("B: want 3000 got %d", balB.Amount())
	}
}

func TestTransferService_Transfer_InsufficientFunds(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)
	transferSvc := service.NewTransferService(accRepo, txRepo)

	accA := domain.NewAccount(uuid.New(), "USD")
	accB := domain.NewAccount(uuid.New(), "USD")
	accRepo.Create(context.Background(), accA) //nolint:errcheck
	accRepo.Create(context.Background(), accB) //nolint:errcheck

	// No funding — balance is zero.
	_, err := transferSvc.Transfer(context.Background(), service.TransferRequest{
		ReferenceID:   "pay-insuf",
		FromAccountID: accA.ID,
		ToAccountID:   accB.ID,
		Amount:        1,
		Currency:      "USD",
	})
	if err != domain.ErrInsufficientFunds {
		t.Fatalf("want ErrInsufficientFunds got %v", err)
	}
}

func TestTransferService_Transfer_CurrencyMismatch(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)
	transferSvc := service.NewTransferService(accRepo, txRepo)

	accA := domain.NewAccount(uuid.New(), "USD")
	accB := domain.NewAccount(uuid.New(), "EUR")
	accRepo.Create(context.Background(), accA) //nolint:errcheck
	accRepo.Create(context.Background(), accB) //nolint:errcheck

	creditAccount(t, accRepo, txRepo, accA.ID, 5_000, "USD")

	_, err := transferSvc.Transfer(context.Background(), service.TransferRequest{
		ReferenceID:   "pay-cur",
		FromAccountID: accA.ID,
		ToAccountID:   accB.ID,
		Amount:        1_000,
		Currency:      "USD",
	})
	if err == nil {
		t.Fatal("expected currency mismatch error")
	}
}

func TestTransferService_Idempotency(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)
	transferSvc := service.NewTransferService(accRepo, txRepo)

	accA := domain.NewAccount(uuid.New(), "USD")
	accB := domain.NewAccount(uuid.New(), "USD")
	accRepo.Create(context.Background(), accA) //nolint:errcheck
	accRepo.Create(context.Background(), accB) //nolint:errcheck
	creditAccount(t, accRepo, txRepo, accA.ID, 20_000, "USD")

	req := service.TransferRequest{
		ReferenceID:   "pay-idem",
		FromAccountID: accA.ID,
		ToAccountID:   accB.ID,
		Amount:        1_000,
		Currency:      "USD",
	}
	if _, err := transferSvc.Transfer(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_, err := transferSvc.Transfer(context.Background(), req)
	if err != domain.ErrDuplicateReference {
		t.Fatalf("want ErrDuplicateReference got %v", err)
	}
}
