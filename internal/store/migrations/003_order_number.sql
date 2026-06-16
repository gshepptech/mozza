-- Add order_number column to deployments for human-friendly sequencing.
ALTER TABLE deployments ADD COLUMN order_number INTEGER DEFAULT 0;
