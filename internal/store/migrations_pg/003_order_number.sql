-- Add order_number column to deployments for human-friendly sequencing.
-- Postgres requires DO block for idempotent ALTER TABLE ADD COLUMN.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'deployments' AND column_name = 'order_number'
    ) THEN
        ALTER TABLE deployments ADD COLUMN order_number INTEGER DEFAULT 0;
    END IF;
END $$;
