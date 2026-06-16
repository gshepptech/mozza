CREATE TABLE IF NOT EXISTS connected_repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_url TEXT NOT NULL,
    webhook_id TEXT,
    oauth_token_encrypted BLOB,
    app_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
