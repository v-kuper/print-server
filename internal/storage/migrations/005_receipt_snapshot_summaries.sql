-- +goose Up
CREATE TABLE IF NOT EXISTS receipt_snapshot_summaries (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    snapshot_id UUID NOT NULL REFERENCES receipt_snapshots(id) ON DELETE CASCADE,
    line_index INTEGER NOT NULL CHECK (line_index >= 0),
    url TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL,
    bullets JSONB NOT NULL DEFAULT '[]'::jsonb,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, snapshot_id, line_index, url)
);

CREATE INDEX IF NOT EXISTS receipt_snapshot_summaries_snapshot_idx
    ON receipt_snapshot_summaries (workspace_id, snapshot_id, line_index);

-- +goose Down
DROP INDEX IF EXISTS receipt_snapshot_summaries_snapshot_idx;
DROP TABLE IF EXISTS receipt_snapshot_summaries;
