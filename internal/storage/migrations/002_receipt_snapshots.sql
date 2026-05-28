-- +goose Up
CREATE TABLE IF NOT EXISTS receipt_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'published', 'failed')),
    news_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    receipt_lines JSONB NOT NULL DEFAULT '[]'::jsonb,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS receipt_snapshots_workspace_created_at
    ON receipt_snapshots (workspace_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS receipt_snapshots_workspace_created_at;
DROP TABLE IF EXISTS receipt_snapshots;
