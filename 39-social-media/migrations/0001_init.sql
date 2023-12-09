-- 0001_init.sql — initial schema for the social-media tool.
-- Applied automatically on startup (kept in sync with store.Schema).

CREATE TABLE IF NOT EXISTS posts (
    id           TEXT PRIMARY KEY,
    author       TEXT NOT NULL,
    body         TEXT NOT NULL,
    visibility   TEXT NOT NULL,                 -- public | connections | private
    target       TEXT NOT NULL,                 -- mock | linkedin | x
    status       TEXT NOT NULL,                 -- draft|scheduled|queued|publishing|published|failed|canceled
    scheduled_at TEXT,                          -- RFC3339 UTC; NULL = publish immediately
    published_at TEXT,                          -- RFC3339 UTC; NULL until published
    external_id  TEXT NOT NULL DEFAULT '',      -- network post id (idempotency)
    error        TEXT NOT NULL DEFAULT '',      -- last failure reason
    attempts     INTEGER NOT NULL DEFAULT 0,    -- publish attempt count
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- Covering index for the scheduler's due-query.
CREATE INDEX IF NOT EXISTS idx_posts_due ON posts(status, scheduled_at);

-- Dedupe / lookup by network id.
CREATE INDEX IF NOT EXISTS idx_posts_target_external ON posts(target, external_id);
