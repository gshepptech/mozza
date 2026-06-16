ALTER TABLE deployments ADD COLUMN version INTEGER DEFAULT 0;
ALTER TABLE deployments ADD COLUMN previous_state TEXT DEFAULT '';
