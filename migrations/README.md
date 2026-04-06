accounts:
    id (UUID)
    user_id
    currency
    created_at

transactions:
    id (UUID)
    reference_id (idempotency key)
    status
    created_at

entries:
    id
    transaction_id
    account_id
    amount (DECIMAL)
    type (debit/credit)
    created_at


Rule: Sum(debits) == Sum(credits) ALWAYS!