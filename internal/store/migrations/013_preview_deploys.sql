CREATE TABLE IF NOT EXISTS preview_deploys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id INTEGER NOT NULL,
    branch TEXT NOT NULL,
    domain TEXT,
    deploy_id INTEGER,
    created_at TEXT NOT NULL,
    last_activity TEXT
);

CREATE INDEX IF NOT EXISTS idx_preview_deploys_app_branch ON preview_deploys (app_id, branch);
