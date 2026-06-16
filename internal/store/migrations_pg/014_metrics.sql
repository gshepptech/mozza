CREATE TABLE IF NOT EXISTS metrics (
    id SERIAL PRIMARY KEY,
    app_id INTEGER NOT NULL,
    timestamp BIGINT NOT NULL,
    cpu_percent DOUBLE PRECISION,
    memory_bytes BIGINT,
    network_rx BIGINT,
    network_tx BIGINT,
    request_count INTEGER
);

CREATE INDEX IF NOT EXISTS idx_metrics_app_timestamp ON metrics (app_id, timestamp);
