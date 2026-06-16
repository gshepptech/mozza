CREATE TABLE IF NOT EXISTS marketplace_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    category TEXT,
    tags TEXT,
    content_hash TEXT,
    recipe_content TEXT,
    fetched_at TEXT
);
