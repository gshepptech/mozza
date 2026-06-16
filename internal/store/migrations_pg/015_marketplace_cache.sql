CREATE TABLE IF NOT EXISTS marketplace_cache (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    category TEXT,
    tags TEXT,
    content_hash TEXT,
    recipe_content TEXT,
    fetched_at TIMESTAMPTZ
);
