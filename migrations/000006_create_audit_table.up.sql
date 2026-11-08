CREATE TYPE action_enum AS ENUM ('user_user','org_user');
CREATE TYPE action_type_enum AS ENUM ('D','C','R');

CREATE TABLE IF NOT EXISTS audits(
    id SERIAL PRIMARY KEY,
    reference_id UUID NOT NULL,
    action action_enum NOT NULL,
    action_type action_type_enum NOT NULL,
    party1 UUID REFERENCES parties(id) ON DELETE SET NULL,
    party2 UUID REFERENCES parties(id) ON DELETE SET NULL,
    party1_currency VARCHAR(3) NOT NULL,
    party2_currency VARCHAR(3) NOT NULL,
    exchange_rate DECIMAL NOT NULL,
    amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audits_created_at ON audits(created_at);
