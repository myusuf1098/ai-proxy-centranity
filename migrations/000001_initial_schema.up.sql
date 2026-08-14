-- Migration: 000001_initial_schema.up.sql
-- Description: Baseline schema for ProxyGateway Enterprise

CREATE TABLE IF NOT EXISTS proxy_profiles (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(16) NOT NULL DEFAULT 'DIRECT', -- DIRECT, HTTP, HTTPS, SOCKS5
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INT NOT NULL DEFAULT 0,
    secret_ref VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS providers (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'openai',
    base_url VARCHAR(512) NOT NULL,
    api_key_ref VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INT NOT NULL DEFAULT 100,
    timeout_ms INT NOT NULL DEFAULT 30000,
    proxy_profile_id VARCHAR(64) REFERENCES proxy_profiles(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS models (
    id VARCHAR(128) PRIMARY KEY,
    provider_id VARCHAR(64) REFERENCES providers(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INT NOT NULL DEFAULT 100,
    timeout_ms INT NOT NULL DEFAULT 30000,
    cost_per_1k_input NUMERIC(10, 6) NOT NULL DEFAULT 0.0,
    cost_per_1k_output NUMERIC(10, 6) NOT NULL DEFAULT 0.0,
    context_window INT NOT NULL DEFAULT 8192,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS model_aliases (
    id SERIAL PRIMARY KEY,
    alias VARCHAR(64) NOT NULL UNIQUE,
    target_model_id VARCHAR(128) REFERENCES models(id) ON DELETE CASCADE,
    target_provider_id VARCHAR(64) REFERENCES providers(id) ON DELETE SET NULL,
    priority INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    prefix VARCHAR(16) NOT NULL,
    hash VARCHAR(64) NOT NULL UNIQUE, -- SHA-256 hash of plaintext key
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    allowed_models TEXT[] NOT NULL DEFAULT '{}',
    denied_models TEXT[] NOT NULL DEFAULT '{}',
    allowed_providers TEXT[] NOT NULL DEFAULT '{}',
    denied_providers TEXT[] NOT NULL DEFAULT '{}',
    rpm_limit INT NOT NULL DEFAULT 60,
    rps_limit INT NOT NULL DEFAULT 10,
    tpm_limit INT NOT NULL DEFAULT 100000,
    daily_token_quota BIGINT NOT NULL DEFAULT 1000000,
    monthly_token_quota BIGINT NOT NULL DEFAULT 30000000,
    budget_limit NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_events (
    id SERIAL PRIMARY KEY,
    actor VARCHAR(128) NOT NULL,
    action VARCHAR(64) NOT NULL,
    target VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    correlation_id VARCHAR(64) NOT NULL,
    source_ip VARCHAR(64) NOT NULL DEFAULT '',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage_daily (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL,
    api_key_id VARCHAR(64) REFERENCES api_keys(id) ON DELETE CASCADE,
    model_id VARCHAR(128) NOT NULL,
    provider_id VARCHAR(64) NOT NULL,
    total_requests BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    cost_estimated NUMERIC(12, 6) NOT NULL DEFAULT 0.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(date, api_key_id, model_id, provider_id)
);

CREATE TABLE IF NOT EXISTS config_versions (
    version INT PRIMARY KEY,
    snapshot_json JSONB NOT NULL,
    created_by VARCHAR(128) NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for high throughput performance
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(hash);
CREATE INDEX IF NOT EXISTS idx_models_provider ON models(provider_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_correlation ON audit_events(correlation_id);
CREATE INDEX IF NOT EXISTS idx_usage_daily_key_date ON usage_daily(api_key_id, date);
