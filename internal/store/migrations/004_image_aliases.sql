CREATE TABLE IF NOT EXISTS image_aliases (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    alias_name TEXT NOT NULL,
    image_ref TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(team_id, alias_name)
);
