CREATE TABLE IF NOT EXISTS webhooks(
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    org_id UUID REFERENCES orgs(id) ON DELETE CASCADE,
    secret_header_key TEXT,
    secret_header_value TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhooks_created_at ON webhooks(created_at);
CREATE INDEX idx_webhooks_updated_at ON webhooks(updated_at);
