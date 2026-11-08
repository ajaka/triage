DROP TRIGGER IF EXISTS trigger_users_updated_at ON users;
DROP TRIGGER IF EXISTS trigger_orgs_updated_at ON orgs;
DROP TRIGGER IF EXISTS trigger_conns_updated_at ON conns;
DROP TRIGGER IF EXISTS trigger_webhooks_updated_at ON webhooks;
DROP FUNCTION IF EXISTS update_updated_at();
