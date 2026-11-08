CREATE TABLE IF NOT EXISTS orgs(
    id UUID PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    private_key TEXT,
    account_no VARCHAR(10) UNIQUE NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    available_balance BIGINT NOT NULL DEFAULT 0,
    locked_balance BIGINT NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orgs_created_at ON orgs(created_at);
CREATE INDEX idx_orgs_updated_at ON orgs(updated_at);
