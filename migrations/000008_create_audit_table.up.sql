CREATE TYPE action_enum AS ENUM ('user_user','org_user');
CREATE TYPE action_type_enum AS ENUM ('D','C','R');

CREATE TABLE IF NOT EXISTS audits(
    id SERIAL PRIMARY KEY,
    reference_id UUID NOT NULL,
    action action_enum NOT NULL,
    action_type action_type_enum NOT NULL,
    party1 UUID NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    party2 UUID NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    party1_currency VARCHAR(3) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE ON DELETE RESTRICT,
    party2_currency VARCHAR(3) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE ON DELETE RESTRICT,
    exchange_rate NUMERIC(15,6) NOT NULL,
    amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audits_created_at ON audits(created_at);
