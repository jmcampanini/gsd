CREATE TABLE projects (
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

CREATE TABLE tasks (
    id           INTEGER PRIMARY KEY,
    project_id   INTEGER REFERENCES projects(id) ON DELETE RESTRICT,
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

CREATE INDEX idx_tasks_project ON tasks(project_id);

CREATE VIEW inbox AS
SELECT t.*
FROM tasks t
WHERE t.project_id IS NULL AND t.status = 'open';

CREATE VIEW available AS
SELECT t.*, p.title AS project_title
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
WHERE t.status = 'open'
  AND (t.project_id IS NULL OR p.status = 'open')
  AND (t.defer_until IS NULL OR t.defer_until <= date('now', 'localtime'));

CREATE VIEW logbook AS
SELECT 'task' AS kind, t.id, t.title, t.status,
       COALESCE(t.done_at, t.cancelled_at) AS resolved_at,
       p.title AS project_title
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
WHERE t.status IN ('done', 'cancelled')
UNION ALL
SELECT 'project', p.id, p.title, p.status,
       COALESCE(p.done_at, p.cancelled_at),
       NULL
FROM projects p
WHERE p.status IN ('done', 'cancelled');

PRAGMA user_version = 9003;
