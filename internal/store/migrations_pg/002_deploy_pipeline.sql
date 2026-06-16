-- Deploy pipeline tables for tracking deploys, resources, and images.

CREATE TABLE IF NOT EXISTS deploys (
    id TEXT PRIMARY KEY,
    app_name TEXT NOT NULL,
    target TEXT NOT NULL,
    k8s_context TEXT,
    namespace TEXT,
    recipe_content TEXT NOT NULL,
    status TEXT NOT NULL,
    triggered_by TEXT NOT NULL DEFAULT 'cli',
    error_message TEXT,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    completed_at TEXT,
    rollback_of TEXT REFERENCES deploys(id)
);

CREATE INDEX IF NOT EXISTS idx_deploys_app_name ON deploys(app_name);
CREATE INDEX IF NOT EXISTS idx_deploys_status ON deploys(status);

CREATE TABLE IF NOT EXISTS deploy_resources (
    id SERIAL PRIMARY KEY,
    deploy_id TEXT NOT NULL REFERENCES deploys(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    status TEXT NOT NULL,
    applied_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_deploy_resources_deploy_id ON deploy_resources(deploy_id);

CREATE TABLE IF NOT EXISTS deploy_images (
    id SERIAL PRIMARY KEY,
    deploy_id TEXT NOT NULL REFERENCES deploys(id) ON DELETE CASCADE,
    slice_name TEXT NOT NULL,
    image TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_deploy_images_deploy_id ON deploy_images(deploy_id);
