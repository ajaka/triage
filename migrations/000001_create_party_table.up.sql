CREATE TYPE party_enum AS ENUM ('user','org');

CREATE TABLE IF NOT EXISTS parties (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    party_type party_enum NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_parties_created_at ON parties(created_at);
