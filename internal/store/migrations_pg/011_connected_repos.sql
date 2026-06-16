CREATE TABLE IF NOT EXISTS connected_repos (
    id SERIAL PRIMARY KEY,
    repo_url TEXT NOT NULL,
    webhook_id TEXT,
    oauth_token_encrypted BYTEA,
    app_id INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
