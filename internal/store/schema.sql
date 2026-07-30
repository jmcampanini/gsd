CREATE TABLE tasks (
    id           INTEGER PRIMARY KEY,
    title        TEXT    NOT NULL,
    note         TEXT    NOT NULL DEFAULT '',
    defer_until  TEXT,
    due_on       TEXT,
    done_at      TEXT,
    cancelled_at TEXT,
    status       TEXT    GENERATED ALWAYS AS (
                     CASE WHEN done_at IS NOT NULL THEN 'done'
                          WHEN cancelled_at IS NOT NULL THEN 'cancelled'
                          ELSE 'open' END) VIRTUAL,
    position     INTEGER NOT NULL,
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (done_at IS NULL OR cancelled_at IS NULL),
    CHECK (defer_until IS date(defer_until)),
    CHECK (due_on IS date(due_on))
) STRICT;

CREATE VIEW inbox AS
SELECT *
FROM tasks
WHERE status = 'open';

CREATE VIEW available AS
SELECT *
FROM tasks
WHERE status = 'open'
  AND (defer_until IS NULL OR defer_until <= date('now', 'localtime'));

PRAGMA user_version = 9002;
