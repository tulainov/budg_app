CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS budget;

CREATE TABLE IF NOT EXISTS budget.categories (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL,
    name         TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('income', 'expense')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (household_id, name)
);

CREATE TABLE IF NOT EXISTS budget.transactions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL,
    user_id      UUID NOT NULL,
    category_id  UUID REFERENCES budget.categories(id),
    scope        TEXT NOT NULL CHECK (scope IN ('personal', 'shared')),
    amount_cents BIGINT NOT NULL,
    currency     TEXT NOT NULL DEFAULT 'EUR',
    description  TEXT,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transactions_household_scope ON budget.transactions(household_id, scope);
CREATE INDEX IF NOT EXISTS idx_transactions_user ON budget.transactions(user_id);
