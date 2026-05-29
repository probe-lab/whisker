ALTER TABLE storage_checks ADD COLUMN IF NOT EXISTS failure LowCardinality(String) DEFAULT ''
