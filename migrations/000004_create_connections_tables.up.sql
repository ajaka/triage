CREATE TYPE conn_status AS ENUM ('pending','accepted','rejected','expired');

CREATE TABLE IF NOT EXISTS conns(
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    status conn_status DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_conns_created_at ON conns(created_at);
CREATE INDEX idx_conns_updated_at ON conns(updated_at);
