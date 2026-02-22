-- migrations/001_initial.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Stores normalized data from each source.
-- All collectors write here; all handlers read from here.
-- Upsert on (source, record_key) so re-syncs are idempotent.
CREATE TABLE source_records (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source      TEXT NOT NULL,
    record_key  TEXT NOT NULL,
    data        JSONB NOT NULL,
    raw_data    JSONB,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,
    UNIQUE (source, record_key)
);

CREATE INDEX idx_source_records_source_key ON source_records(source, record_key);
CREATE INDEX idx_source_records_data       ON source_records USING GIN(data);
CREATE INDEX idx_source_records_fetched_at ON source_records(fetched_at DESC);

-- Tracks each paid API query for billing and analytics.
CREATE TABLE query_log (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    endpoint        TEXT NOT NULL,
    payment_hash    TEXT,
    amount_usdc     NUMERIC(18, 6),
    wallet_address  TEXT,
    duration_ms     INT,
    cache_hit       BOOLEAN DEFAULT FALSE,
    status_code     INT,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_query_log_created_at ON query_log(created_at DESC);
CREATE INDEX idx_query_log_wallet     ON query_log(wallet_address);

-- Tracks the state and schedule of background data collectors.
CREATE TABLE collector_runs (
    source        TEXT PRIMARY KEY,
    last_run_at   TIMESTAMPTZ,
    last_success  TIMESTAMPTZ,
    next_run_at   TIMESTAMPTZ,
    status        TEXT DEFAULT 'pending',   -- 'pending', 'running', 'ok', 'error'
    error_msg     TEXT,
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);
