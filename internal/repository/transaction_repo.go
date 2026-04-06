package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository interface {
	Create(ctx context.Context, tx domain.Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error)
	GetByReferenceID(ctx context.Context, refID string) (domain.Transaction, error)
}

type pgTransactionRepo struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) TransactionRepository {
	return &pgTransactionRepo{db: db}
}

func (r *pgTransactionRepo) Create(ctx context.Context, tx domain.Transaction) error {
	dbTx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer dbTx.Rollback(ctx)

	_, err = dbTx.Exec(ctx,
		`INSERT INTO transactions (id, reference_id, status, created_at) VALUES ($1, $2, $3, $4)`,
		tx.ID, tx.ReferenceID, string(tx.Status), tx.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateReference
		}
		return err
	}

	for _, e := range tx.Entries {
		_, err = dbTx.Exec(ctx,
			`INSERT INTO entries (id, transaction_id, account_id, amount, type, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			e.ID, e.TransactionID, e.AccountID, e.Amount.Amount(), string(e.Type), e.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	payload, err := json.Marshal(map[string]any{
		"transaction_id": tx.ID,
		"reference_id":   tx.ReferenceID,
		"status":         string(tx.Status),
	})
	if err != nil {
		return err
	}

	_, err = dbTx.Exec(ctx,
		`INSERT INTO outbox (id, aggregate_id, event_type, payload, created_at) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), tx.ID, "transaction.committed", payload, time.Now().UTC(),
	)
	if err != nil {
		return err
	}

	return dbTx.Commit(ctx)
}

func (r *pgTransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error) {
	return r.fetchTransaction(ctx, `WHERE t.id = $1`, id)
}

func (r *pgTransactionRepo) GetByReferenceID(ctx context.Context, refID string) (domain.Transaction, error) {
	return r.fetchTransaction(ctx, `WHERE t.reference_id = $1`, refID)
}

func (r *pgTransactionRepo) fetchTransaction(ctx context.Context, where string, arg any) (domain.Transaction, error) {
	var tx domain.Transaction
	err := r.db.QueryRow(ctx,
		`SELECT id, reference_id, status, created_at FROM transactions t `+where,
		arg,
	).Scan(&tx.ID, &tx.ReferenceID, &tx.Status, &tx.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transaction{}, domain.ErrTransactionNotFound
		}
		return domain.Transaction{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.transaction_id, e.account_id, e.amount, e.type, e.created_at, a.currency
		FROM entries e
		JOIN accounts a ON a.id = e.account_id
		WHERE e.transaction_id = $1
	`, tx.ID)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var e domain.Entry
		var n pgtype.Numeric
		var entryType string
		var currency string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &n, &entryType, &e.CreatedAt, &currency); err != nil {
			return domain.Transaction{}, err
		}
		amount := numericToInt64(n)
		money, err := domain.NewMoney(amount, currency)
		if err != nil {
			return domain.Transaction{}, err
		}
		e.Amount = money
		e.Type = domain.EntryType(entryType)
		tx.Entries = append(tx.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return domain.Transaction{}, err
	}

	return tx, nil
}
