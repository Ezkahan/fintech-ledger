CREATE TYPE transaction_status AS ENUM ('pending', 'committed', 'failed');

CREATE TABLE transactions (
    id           UUID               NOT NULL DEFAULT gen_random_uuid(),
    reference_id VARCHAR(255)       NOT NULL,
    status       transaction_status NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_transactions PRIMARY KEY (id),
    CONSTRAINT uq_transactions_reference_id UNIQUE (reference_id)
);

CREATE TYPE entry_type AS ENUM ('debit', 'credit');

CREATE TABLE entries (
    id             UUID        NOT NULL DEFAULT gen_random_uuid(),
    transaction_id UUID        NOT NULL,
    account_id     UUID        NOT NULL,
    amount         NUMERIC(20, 8) NOT NULL,
    type           entry_type  NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_entries PRIMARY KEY (id),
    CONSTRAINT fk_entries_transaction FOREIGN KEY (transaction_id) REFERENCES transactions (id),
    CONSTRAINT fk_entries_account     FOREIGN KEY (account_id)     REFERENCES accounts (id),
    CONSTRAINT ck_entries_amount_positive CHECK (amount > 0)
);

CREATE INDEX idx_entries_transaction_id ON entries (transaction_id);
CREATE INDEX idx_entries_account_id     ON entries (account_id);
