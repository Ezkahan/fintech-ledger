package domain_test

import (
	"testing"

	"github.com/ezkahan/fintech-ledger/internal/domain"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		cur     string
		wantErr error
	}{
		{"valid", 1000, "USD", nil},
		{"zero ok", 0, "USD", nil},
		{"negative", -1, "USD", domain.ErrNegativeAmount},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewMoney(tc.amount, tc.cur)
			if err != tc.wantErr {
				t.Fatalf("want %v got %v", tc.wantErr, err)
			}
		})
	}
}

func TestMoneyAdd(t *testing.T) {
	a := domain.MustNewMoney(500, "USD")
	b := domain.MustNewMoney(300, "USD")

	got, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount() != 800 {
		t.Fatalf("want 800 got %d", got.Amount())
	}
}

func TestMoneyAdd_CurrencyMismatch(t *testing.T) {
	a := domain.MustNewMoney(500, "USD")
	b := domain.MustNewMoney(300, "EUR")
	_, err := a.Add(b)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMoneySub(t *testing.T) {
	a := domain.MustNewMoney(1000, "USD")
	b := domain.MustNewMoney(400, "USD")

	got, err := a.Sub(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount() != 600 {
		t.Fatalf("want 600 got %d", got.Amount())
	}
}

func TestMoneySub_Underflow(t *testing.T) {
	a := domain.MustNewMoney(100, "USD")
	b := domain.MustNewMoney(200, "USD")
	_, err := a.Sub(b)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMoneyEqual(t *testing.T) {
	a := domain.MustNewMoney(500, "USD")
	b := domain.MustNewMoney(500, "USD")
	c := domain.MustNewMoney(500, "EUR")

	if !a.Equal(b) {
		t.Fatal("should be equal")
	}
	if a.Equal(c) {
		t.Fatal("different currencies should not be equal")
	}
}

func TestMoneyGreaterThan(t *testing.T) {
	a := domain.MustNewMoney(1000, "USD")
	b := domain.MustNewMoney(500, "USD")

	if !a.GreaterThan(b) {
		t.Fatal("1000 should be greater than 500")
	}
	if b.GreaterThan(a) {
		t.Fatal("500 should not be greater than 1000")
	}
}
