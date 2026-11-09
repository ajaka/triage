CREATE TABLE IF NOT EXISTS currencies(
    id SERIAL PRIMARY KEY,
    currency TEXT NOT NULL UNIQUE,
    code VARCHAR(3) NOT NULL UNIQUE,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    last_known_rate_to_dollars NUMERIC(15,6) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


CREATE INDEX idx_currencies_created_at ON currencies(created_at);
CREATE INDEX idx_currencies_updated_at ON currencies(updated_at);
