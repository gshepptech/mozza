CREATE TABLE IF NOT EXISTS builds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_url TEXT NOT NULL,
    commit_sha TEXT,
    branch TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    logs TEXT,
    duration_ms INTEGER,
    image_tag TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_builds_repo_branch ON builds (repo_url, branch);
