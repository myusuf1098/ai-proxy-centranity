-- Migration: 000001_initial_schema.down.sql
-- Description: Rollback baseline schema for ProxyGateway Enterprise

DROP TABLE IF EXISTS config_versions CASCADE;
DROP TABLE IF EXISTS usage_daily CASCADE;
DROP TABLE IF EXISTS audit_events CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS model_aliases CASCADE;
DROP TABLE IF EXISTS models CASCADE;
DROP TABLE IF EXISTS providers CASCADE;
DROP TABLE IF EXISTS proxy_profiles CASCADE;
