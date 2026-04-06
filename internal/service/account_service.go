package service

import (
	"context"
	"regexp"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/google/uuid"
)

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

type AccountService interface {
	CreateAccount(ctx context.Context, userID uuid.UUID, currency string) (domain.Account, error)
	GetAccount(ctx context.Context, id uuid.UUID) (domain.Account, error)
	ListUserAccounts(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	GetBalance(ctx context.Context, accountID uuid.UUID) (domain.Money, error)
}

type accountService struct {
	repo repository.AccountRepository
}

func NewAccountService(repo repository.AccountRepository) AccountService {
	return &accountService{repo: repo}
}

func (s *accountService) CreateAccount(ctx context.Context, userID uuid.UUID, currency string) (domain.Account, error) {
	if !currencyRe.MatchString(currency) {
		return domain.Account{}, domain.ErrCurrencyNotSupported
	}
	acc := domain.NewAccount(userID, currency)
	if err := s.repo.Create(ctx, acc); err != nil {
		return domain.Account{}, err
	}
	return acc, nil
}

func (s *accountService) GetAccount(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *accountService) ListUserAccounts(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *accountService) GetBalance(ctx context.Context, accountID uuid.UUID) (domain.Money, error) {
	return s.repo.GetBalance(ctx, accountID)
}
