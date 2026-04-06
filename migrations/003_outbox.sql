CREATE TABLE outbox (
    id           UUID        NOT NULL DEFAULT gen_random_uuid(),
    aggregate_id UUID        NOT NULL,
    event_type   VARCHAR(100) NOT NULL,
    payload      JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,

    CONSTRAINT pk_outbox PRIMARY KEY (id)
);

CREATE INDEX idx_outbox_unprocessed ON outbox (created_at)
    WHERE processed_at IS NULL;
