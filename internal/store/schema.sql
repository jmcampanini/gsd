CREATE TABLE tasks (
    id           INTEGER PRIMARY KEY,
    title        TEXT    NOT NULL,
    note         TEXT    NOT NULL DEFAULT '',
    done_at      TEXT,
    cancelled_at TEXT,
    status       TEXT    GENERATED ALWAYS AS (
                     CASE WHEN done_at IS NOT NULL THEN 'done'
                          WHEN cancelled_at IS NOT NULL THEN 'cancelled'
                          ELSE 'open' END) VIRTUAL,
    position     INTEGER NOT NULL,
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (done_at IS NULL OR cancelled_at IS NULL)
) STRICT;

CREATE VIEW inbox AS
SELECT *
FROM tasks
WHERE status = 'open';

PRAGMA user_version = 9001;
