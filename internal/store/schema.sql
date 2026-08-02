CREATE TABLE areas (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    note        TEXT    NOT NULL DEFAULT '',
    archived_at TEXT,
    position    INTEGER NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE projects (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    area_id      INTEGER REFERENCES areas(id) ON DELETE RESTRICT,
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
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   INTEGER REFERENCES projects(id) ON DELETE RESTRICT,
    area_id      INTEGER REFERENCES areas(id)    ON DELETE RESTRICT,
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
    CHECK (project_id IS NULL OR area_id IS NULL),
    CHECK (done_at IS NULL OR cancelled_at IS NULL),
    CHECK (defer_until IS date(defer_until)),
    CHECK (due_on IS date(due_on))
) STRICT;

CREATE INDEX idx_projects_area ON projects(area_id);
CREATE INDEX idx_tasks_project ON tasks(project_id);
CREATE INDEX idx_tasks_area    ON tasks(area_id);

CREATE VIEW inbox AS
SELECT t.*,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN areas    a ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.project_id IS NULL AND t.area_id IS NULL AND t.status = 'open';

CREATE VIEW available AS
SELECT t.*,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN areas    a ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.status = 'open'
  AND (t.project_id IS NULL OR p.status = 'open')
  AND a.archived_at IS NULL
  AND (t.defer_until IS NULL OR t.defer_until <= date('now', 'localtime'));

CREATE VIEW logbook AS
SELECT 'task' AS kind, t.id, t.title, t.status,
       COALESCE(t.done_at, t.cancelled_at) AS resolved_at,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN areas    a ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.status IN ('done', 'cancelled')
UNION ALL
SELECT 'project', p.id, p.title, p.status,
       COALESCE(p.done_at, p.cancelled_at),
       NULL,
       p.area_id,
       a.title
FROM projects p
LEFT JOIN areas a ON a.id = p.area_id
WHERE p.status IN ('done', 'cancelled');

PRAGMA user_version = 9004;
