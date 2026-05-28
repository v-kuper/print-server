-- +goose Up
ALTER TABLE receipt_snapshots
    ADD COLUMN IF NOT EXISTS paper_chars INTEGER NOT NULL DEFAULT 32;

-- +goose Down
ALTER TABLE receipt_snapshots
    DROP COLUMN IF EXISTS paper_chars;
