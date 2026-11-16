CREATE TABLE IF NOT EXISTS preview_deploys (
    id SERIAL PRIMARY KEY,
    app_id INTEGER NOT NULL,
    branch TEXT NOT NULL,
    domain TEXT,
    deploy_id INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_activity TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_preview_deploys_app_branch ON preview_deploys (app_id, branch);
