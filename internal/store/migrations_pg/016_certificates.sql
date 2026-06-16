CREATE TABLE IF NOT EXISTS certificates (
    id SERIAL PRIMARY KEY,
    domain TEXT NOT NULL UNIQUE,
    issued_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    provider TEXT,
    cert_path TEXT,
    key_path TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_certificates_domain ON certificates (domain);
