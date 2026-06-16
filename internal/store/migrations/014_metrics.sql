CREATE TABLE IF NOT EXISTS metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,
    cpu_percent REAL,
    memory_bytes INTEGER,
    network_rx INTEGER,
    network_tx INTEGER,
    request_count INTEGER
);

CREATE INDEX IF NOT EXISTS idx_metrics_app_timestamp ON metrics (app_id, timestamp);
