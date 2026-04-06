package repository

import (
	"context"
	"errors"
	"math/big"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountRepository interface {
	Create(ctx context.Context, acc domain.Account) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
	GetByUserIDAndCurrency(ctx context.Context, userID uuid.UUID, currency string) (domain.Account, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	GetBalance(ctx context.Context, accountID uuid.UUID) (domain.Money, error)
}

type pgAccountRepo struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) AccountRepository {
	return &pgAccountRepo{db: db}
}

func (r *pgAccountRepo) Create(ctx context.Context, acc domain.Account) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO accounts (id, user_id, currency, created_at) VALUES ($1, $2, $3, $4)`,
		acc.ID, acc.UserID, acc.Currency, acc.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateAccount
		}
		return err
	}
	return nil
}

func (r *pgAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	var acc domain.Account
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, currency, created_at FROM accounts WHERE id = $1`,
		id,
	).Scan(&acc.ID, &acc.UserID, &acc.Currency, &acc.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrAccountNotFound
		}
		return domain.Account{}, err
	}
	return acc, nil
}

func (r *pgAccountRepo) GetByUserIDAndCurrency(ctx context.Context, userID uuid.UUID, currency string) (domain.Account, error) {
	var acc domain.Account
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, currency, created_at FROM accounts WHERE user_id = $1 AND currency = $2`,
		userID, currency,
	).Scan(&acc.ID, &acc.UserID, &acc.Currency, &acc.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrAccountNotFound
		}
		return domain.Account{}, err
	}
	return acc, nil
}

func (r *pgAccountRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, currency, created_at FROM accounts WHERE user_id = $1 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var acc domain.Account
		if err := rows.Scan(&acc.ID, &acc.UserID, &acc.Currency, &acc.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

func (r *pgAccountRepo) GetBalance(ctx context.Context, accountID uuid.UUID) (domain.Money, error) {
	var currency string
	var n pgtype.Numeric
	err := r.db.QueryRow(ctx, `
		SELECT
			a.currency,
			COALESCE(SUM(CASE WHEN e.type = 'credit' THEN e.amount ELSE -e.amount END), 0) AS balance
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		LEFT JOIN transactions t ON t.id = e.transaction_id AND t.status = 'committed'
		WHERE a.id = $1
		GROUP BY a.currency
	`, accountID).Scan(&currency, &n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Money{}, domain.ErrAccountNotFound
		}
		return domain.Money{}, err
	}

	amount := numericToInt64(n)
	return domain.NewMoney(amount, currency)
}

func numericToInt64(n pgtype.Numeric) int64 {
	if !n.Valid || n.NaN || n.InfinityModifier != pgtype.Finite {
		return 0
	}
	if n.Int == nil {
		return 0
	}
	val := new(big.Int).Set(n.Int)
	if n.Exp > 0 {
		mul := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Exp)), nil)
		val.Mul(val, mul)
	} else if n.Exp < 0 {
		div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil)
		val.Div(val, div)
	}
	return val.Int64()
}
