CREATE TABLE IF NOT EXISTS cluster_snapshots (
    id SERIAL PRIMARY KEY,
    data TEXT NOT NULL,
    created_at TEXT NOT NULL
);
