-- +goose Up
CREATE TABLE IF NOT EXISTS telegram_fax_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    dedupe_key TEXT NOT NULL,
    source TEXT NOT NULL,
    content_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'printed', 'failed')),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    printed_at TIMESTAMPTZ,
    UNIQUE (workspace_id, dedupe_key)
);

CREATE INDEX IF NOT EXISTS telegram_fax_queue_pending
    ON telegram_fax_queue (workspace_id, next_attempt_at, created_at, id)
    WHERE status = 'pending';

-- +goose Down
DROP INDEX IF EXISTS telegram_fax_queue_pending;
DROP TABLE IF EXISTS telegram_fax_queue;
