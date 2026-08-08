CREATE TABLE areas (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    note        TEXT    NOT NULL DEFAULT '',
    archived_at TEXT,
    position    INTEGER NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE boards (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    note       TEXT    NOT NULL DEFAULT '',
    position   INTEGER NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE stages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    board_id   INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    title      TEXT    NOT NULL,
    position   INTEGER NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (board_id, title COLLATE NOCASE)
) STRICT;

CREATE INDEX idx_stages_board ON stages(board_id);

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
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    stage_id       INTEGER REFERENCES stages(id) ON DELETE RESTRICT,
    stage_position INTEGER,
    CHECK (done_at IS NULL OR cancelled_at IS NULL),
    CHECK ((stage_id IS NULL) = (stage_position IS NULL))
) STRICT;

CREATE INDEX idx_projects_area  ON projects(area_id);
CREATE INDEX idx_projects_stage ON projects(stage_id);

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
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    defer_stage_id INTEGER REFERENCES stages(id) ON DELETE RESTRICT,
    promotes       INTEGER NOT NULL DEFAULT 0 CHECK (promotes IN (0, 1)),
    CHECK (project_id IS NULL OR area_id IS NULL),
    CHECK (done_at IS NULL OR cancelled_at IS NULL),
    CHECK (defer_until IS date(defer_until)),
    CHECK (due_on IS date(due_on))
) STRICT;

CREATE INDEX idx_tasks_project     ON tasks(project_id);
CREATE INDEX idx_tasks_area        ON tasks(area_id);
CREATE INDEX idx_tasks_defer_stage ON tasks(defer_stage_id);

CREATE TABLE tags (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE task_tags (
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (task_id, tag_id)
) STRICT, WITHOUT ROWID;

CREATE TABLE project_tags (
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id)     ON DELETE CASCADE,
    PRIMARY KEY (project_id, tag_id)
) STRICT, WITHOUT ROWID;

CREATE TABLE area_tags (
    area_id INTEGER NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (area_id, tag_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX idx_task_tags_tag    ON task_tags(tag_id);
CREATE INDEX idx_project_tags_tag ON project_tags(tag_id);
CREATE INDEX idx_area_tags_tag    ON area_tags(tag_id);

CREATE VIEW inbox AS
SELECT t.*,
       ds.title                       AS defer_stage_title,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title,
       (SELECT json_group_array(g.title ORDER BY g.title COLLATE NOCASE)
        FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
        WHERE tt.task_id = t.id)      AS tags
FROM tasks t
LEFT JOIN projects p  ON p.id = t.project_id
LEFT JOIN stages   ds ON ds.id = t.defer_stage_id
LEFT JOIN areas    a  ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.project_id IS NULL AND t.area_id IS NULL AND t.status = 'open';

CREATE VIEW available AS
SELECT t.*,
       ds.title                       AS defer_stage_title,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title,
       (SELECT json_group_array(g.title ORDER BY g.title COLLATE NOCASE)
        FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
        WHERE tt.task_id = t.id)      AS tags
FROM tasks t
LEFT JOIN projects p  ON p.id = t.project_id
LEFT JOIN stages   ds ON ds.id = t.defer_stage_id
LEFT JOIN stages   ps ON ps.id = p.stage_id
LEFT JOIN areas    a  ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.status = 'open'
  AND (t.project_id IS NULL OR p.status = 'open')
  AND a.archived_at IS NULL
  AND (t.defer_until IS NULL OR t.defer_until <= date('now', 'localtime'))
  AND (t.defer_stage_id IS NULL OR
       (ps.board_id = ds.board_id AND ps.position >= ds.position));

CREATE VIEW logbook AS
SELECT 'task' AS kind, t.id, t.title, t.status,
       COALESCE(t.done_at, t.cancelled_at) AS resolved_at,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title,
       (SELECT json_group_array(g.title ORDER BY g.title COLLATE NOCASE)
        FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
        WHERE tt.task_id = t.id)      AS tags
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN areas    a ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.status IN ('done', 'cancelled')
UNION ALL
SELECT 'project', p.id, p.title, p.status,
       COALESCE(p.done_at, p.cancelled_at),
       NULL,
       p.area_id,
       a.title,
       (SELECT json_group_array(g.title ORDER BY g.title COLLATE NOCASE)
        FROM project_tags pt JOIN tags g ON g.id = pt.tag_id
        WHERE pt.project_id = p.id)
FROM projects p
LEFT JOIN areas a ON a.id = p.area_id
WHERE p.status IN ('done', 'cancelled');

