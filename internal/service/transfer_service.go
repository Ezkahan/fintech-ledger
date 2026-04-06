package service

import (
	"context"
	"fmt"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/google/uuid"
)

type TransferRequest struct {
	ReferenceID   string
	FromAccountID uuid.UUID
	ToAccountID   uuid.UUID
	Amount        int64
	Currency      string
}

type TransferService interface {
	Transfer(ctx context.Context, req TransferRequest) (domain.Transaction, error)
	GetTransaction(ctx context.Context, id uuid.UUID) (domain.Transaction, error)
	GetTransactionByReference(ctx context.Context, refID string) (domain.Transaction, error)
}

type transferService struct {
	accountRepo repository.AccountRepository
	txRepo      repository.TransactionRepository
}

func NewTransferService(accountRepo repository.AccountRepository, txRepo repository.TransactionRepository) TransferService {
	return &transferService{accountRepo: accountRepo, txRepo: txRepo}
}

func (s *transferService) Transfer(ctx context.Context, req TransferRequest) (domain.Transaction, error) {
	fromAcc, err := s.accountRepo.GetByID(ctx, req.FromAccountID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("from account: %w", err)
	}
	toAcc, err := s.accountRepo.GetByID(ctx, req.ToAccountID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("to account: %w", err)
	}

	if fromAcc.Currency != req.Currency {
		return domain.Transaction{}, fmt.Errorf("%w: from account currency %s does not match %s",
			domain.ErrCurrencyNotSupported, fromAcc.Currency, req.Currency)
	}
	if toAcc.Currency != req.Currency {
		return domain.Transaction{}, fmt.Errorf("%w: to account currency %s does not match %s",
			domain.ErrCurrencyNotSupported, toAcc.Currency, req.Currency)
	}

	amount, err := domain.NewMoney(req.Amount, req.Currency)
	if err != nil {
		return domain.Transaction{}, err
	}

	balance, err := s.accountRepo.GetBalance(ctx, req.FromAccountID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if !balance.GreaterThan(amount) && !balance.Equal(amount) {
		return domain.Transaction{}, domain.ErrInsufficientFunds
	}

	entries := []domain.Entry{
		{AccountID: req.FromAccountID, Amount: amount, Type: domain.EntryDebit},
		{AccountID: req.ToAccountID, Amount: amount, Type: domain.EntryCredit},
	}

	tx, err := domain.NewTransaction(req.ReferenceID, entries)
	if err != nil {
		return domain.Transaction{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Transaction{}, err
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return domain.Transaction{}, err
	}

	return tx, nil
}

func (s *transferService) GetTransaction(ctx context.Context, id uuid.UUID) (domain.Transaction, error) {
	return s.txRepo.GetByID(ctx, id)
}

func (s *transferService) GetTransactionByReference(ctx context.Context, refID string) (domain.Transaction, error) {
	return s.txRepo.GetByReferenceID(ctx, refID)
}
