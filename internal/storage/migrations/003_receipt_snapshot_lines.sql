-- +goose Up
ALTER TABLE receipt_snapshots
    ADD COLUMN IF NOT EXISTS receipt_lines JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE receipt_snapshots
    DROP COLUMN IF EXISTS receipt_lines;
