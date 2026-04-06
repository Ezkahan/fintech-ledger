package repository_test

import (
	"context"
	"testing"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/ezkahan/fintech-ledger/internal/testhelper"
	"github.com/google/uuid"
)

func TestAccountRepo_CreateAndGetByID(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	repo := repository.NewAccountRepository(pool)

	userID := uuid.New()
	acc := domain.NewAccount(userID, "USD")

	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != acc.ID || got.Currency != "USD" || got.UserID != userID {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestAccountRepo_DuplicateAccount(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	repo := repository.NewAccountRepository(pool)

	userID := uuid.New()
	acc := domain.NewAccount(userID, "EUR")

	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatal(err)
	}
	acc2 := domain.NewAccount(userID, "EUR")
	err := repo.Create(context.Background(), acc2)
	if err != domain.ErrDuplicateAccount {
		t.Fatalf("want ErrDuplicateAccount got %v", err)
	}
}

func TestAccountRepo_GetByID_NotFound(t *testing.T) {
	pool := testhelper.NewPool(t)
	repo := repository.NewAccountRepository(pool)

	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != domain.ErrAccountNotFound {
		t.Fatalf("want ErrAccountNotFound got %v", err)
	}
}

func TestAccountRepo_ListByUserID(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	repo := repository.NewAccountRepository(pool)

	userID := uuid.New()
	for _, cur := range []string{"USD", "EUR", "GBP"} {
		if err := repo.Create(context.Background(), domain.NewAccount(userID, cur)); err != nil {
			t.Fatal(err)
		}
	}

	accounts, err := repo.ListByUserID(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 3 {
		t.Fatalf("want 3 got %d", len(accounts))
	}
}

func TestAccountRepo_GetBalance_Empty(t *testing.T) {
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	repo := repository.NewAccountRepository(pool)

	acc := domain.NewAccount(uuid.New(), "USD")
	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatal(err)
	}

	money, err := repo.GetBalance(context.Background(), acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !money.IsZero() {
		t.Fatalf("want zero balance got %d", money.Amount())
	}
}
