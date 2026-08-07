package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type migrationSchemaSnapshot map[string]migrationSchemaObject

type migrationSchemaObject struct {
	kind        string
	virtual     bool
	columns     []migrationSchemaColumn
	foreignKeys []migrationSchemaForeignKey
	uniques     []migrationSchemaUnique
}

type migrationTableMarker struct {
	tableName   string
	triggerName string
}

type migrationSchemaColumn struct {
	cid        int
	name       string
	columnType string
	notNull    int
	defaultSQL sql.NullString
	primaryKey int
	hidden     int
}

type migrationSchemaForeignKey struct {
	referencedTable string
	onUpdate        string
	onDelete        string
	match           string
	columns         []migrationSchemaForeignKeyColumn
}

type migrationSchemaForeignKeyColumn struct {
	local      string
	referenced sql.NullString
}

type migrationSchemaUnique struct {
	columns []migrationSchemaIndexedColumn
}

type migrationSchemaIndexedColumn struct {
	cid        int
	name       sql.NullString
	descending int
	collation  sql.NullString
}

func TestEmbeddedMigrationChainPreservesSchemaContract(t *testing.T) {
	t.Parallel()

	migrations, err := loadMigrations(migrationFiles)
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if err := lintMigrationContract(context.Background(), openRawMigrationDatabase(t), migrations); err != nil {
		t.Fatalf("embedded migration contract: %v", err)
	}
}

func TestReleasedMigrationsRemainUnchanged(t *testing.T) {
	t.Parallel()

	released := []struct {
		name     string
		checksum string
	}{
		{
			name:     "migrations/0001_baseline.sql",
			checksum: "5f78a707c728208795d6b5247b487b514d92c1e46c5e074dc683b99bd8c90280",
		},
	}
	for _, migration := range released {
		contents, err := fs.ReadFile(migrationFiles, migration.name)
		if err != nil {
			t.Errorf("read released migration %s: %v", migration.name, err)
			continue
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(contents))
		if checksum != migration.checksum {
			t.Errorf("released migration %s checksum = %s, want %s", migration.name, checksum, migration.checksum)
		}
	}
}

func TestMigrationContractAllowsAppendedColumnsAndWholeObjectDrops(t *testing.T) {
	t.Parallel()

	migrations := []migration{
		{
			revision: 1,
			name:     "0001_first.sql",
			sql:      "CREATE TABLE sample (first TEXT); CREATE TABLE source (first TEXT, second TEXT); CREATE VIEW visible AS SELECT first FROM source;",
		},
		{
			revision: 2,
			name:     "0002_append.sql",
			sql:      "ALTER TABLE sample ADD COLUMN second INTEGER; CREATE INDEX idx_sample_first ON sample(first); DROP VIEW visible; CREATE VIEW visible AS SELECT first, second FROM source;",
		},
		{
			revision: 3,
			name:     "0003_drop.sql",
			sql:      "DROP VIEW visible; DROP TABLE sample; DROP TABLE source;",
		},
	}
	if err := lintMigrationContract(context.Background(), openRawMigrationDatabase(t), migrations); err != nil {
		t.Fatalf("lintMigrationContract() error = %v", err)
	}
}

func TestMigrationContractRejectsTableReplacement(t *testing.T) {
	t.Parallel()

	migrations := migrationContractFixture(
		"CREATE TABLE sample (value TEXT);",
		"DROP TABLE sample; CREATE TABLE sample (value TEXT, added TEXT);",
	)
	err := lintMigrationContract(context.Background(), openRawMigrationDatabase(t), migrations)
	if err == nil || !strings.Contains(err.Error(), "replaced table") {
		t.Fatalf("lintMigrationContract() error = %v, want table-replacement violation", err)
	}
}

func TestMigrationContractRejectsChangesToSurvivingObjects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		migrations []migration
	}{
		{
			name: "column retype",
			migrations: migrationContractFixture(
				"CREATE TABLE sample (value TEXT);",
				"ALTER TABLE sample RENAME TO old_sample; CREATE TABLE sample (value INTEGER); DROP TABLE old_sample;",
			),
		},
		{
			name: "column loss",
			migrations: migrationContractFixture(
				"CREATE TABLE sample (first TEXT, second TEXT);",
				"ALTER TABLE sample RENAME TO old_sample; CREATE TABLE sample (first TEXT); DROP TABLE old_sample;",
			),
		},
		{
			name: "view column loss",
			migrations: migrationContractFixture(
				"CREATE TABLE source (first TEXT, second TEXT); CREATE VIEW sample AS SELECT first, second FROM source;",
				"DROP VIEW sample; CREATE VIEW sample AS SELECT first FROM source;",
			),
		},
		{
			name: "nullability change",
			migrations: migrationContractFixture(
				"CREATE TABLE sample (value TEXT);",
				"ALTER TABLE sample RENAME TO old_sample; CREATE TABLE sample (value TEXT NOT NULL); DROP TABLE old_sample;",
			),
		},
		{
			name: "foreign key retarget",
			migrations: migrationContractFixture(
				"CREATE TABLE first (id INTEGER PRIMARY KEY); CREATE TABLE second (id INTEGER PRIMARY KEY); CREATE TABLE sample (parent_id INTEGER REFERENCES first(id));",
				"ALTER TABLE sample RENAME TO old_sample; CREATE TABLE sample (parent_id INTEGER REFERENCES second(id)); DROP TABLE old_sample;",
			),
		},
		{
			name: "unique constraint loss",
			migrations: migrationContractFixture(
				"CREATE TABLE sample (value TEXT UNIQUE);",
				"ALTER TABLE sample RENAME TO old_sample; CREATE TABLE sample (value TEXT); DROP TABLE old_sample;",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := lintMigrationContract(
				context.Background(),
				openRawMigrationDatabase(t),
				test.migrations,
			)
			if err == nil {
				t.Fatal("lintMigrationContract() error = nil, want contract violation")
			}
		})
	}
}

func migrationContractFixture(first, second string) []migration {
	return []migration{
		{revision: 1, name: "0001_first.sql", sql: first},
		{revision: 2, name: "0002_second.sql", sql: second},
	}
}

func lintMigrationContract(
	ctx context.Context,
	database *sql.DB,
	migrations []migration,
) error {
	previous, err := readMigrationSchema(ctx, database)
	if err != nil {
		return err
	}
	for index := range migrations {
		markers, err := markMigrationTables(ctx, database, previous)
		if err != nil {
			return fmt.Errorf("mark tables before migration revision %d: %w", index+1, err)
		}
		if err := migrate(ctx, database, migrations[:index+1]); err != nil {
			return fmt.Errorf("apply migrations through revision %d: %w", index+1, err)
		}
		current, err := readMigrationSchema(ctx, database)
		if err != nil {
			return err
		}
		if err := verifyMigrationTableMarkers(ctx, database, markers, current); err != nil {
			return fmt.Errorf("migration revision %d: %w", index+1, err)
		}
		if err := compareMigrationSchemas(previous, current); err != nil {
			return fmt.Errorf("migration revision %d: %w", index+1, err)
		}
		if err := removeMigrationTableMarkers(ctx, database, markers); err != nil {
			return fmt.Errorf("remove markers after migration revision %d: %w", index+1, err)
		}
		previous = current
	}

	return nil
}

func markMigrationTables(
	ctx context.Context,
	database *sql.DB,
	schema migrationSchemaSnapshot,
) ([]migrationTableMarker, error) {
	names := make([]string, 0, len(schema))
	for name, object := range schema {
		if object.kind == "table" && !object.virtual {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	markers := make([]migrationTableMarker, 0, len(names))
	for index, name := range names {
		marker := migrationTableMarker{
			tableName:   name,
			triggerName: fmt.Sprintf("__gsd_migration_contract_%d", index),
		}
		if _, err := database.ExecContext(
			ctx,
			fmt.Sprintf(
				"CREATE TRIGGER %s AFTER INSERT ON %s BEGIN SELECT 1; END",
				quoteMigrationIdentifier(marker.triggerName),
				quoteMigrationIdentifier(marker.tableName),
			),
		); err != nil {
			return nil, fmt.Errorf("mark table %s: %w", name, err)
		}
		markers = append(markers, marker)
	}

	return markers, nil
}

func verifyMigrationTableMarkers(
	ctx context.Context,
	database *sql.DB,
	markers []migrationTableMarker,
	after migrationSchemaSnapshot,
) error {
	for _, marker := range markers {
		current, exists := after[marker.tableName]
		if !exists || current.kind != "table" {
			continue
		}
		var count int
		if err := database.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM sqlite_schema WHERE type = 'trigger' AND name = ? AND tbl_name = ?",
			marker.triggerName,
			marker.tableName,
		).Scan(&count); err != nil {
			return fmt.Errorf("inspect marker for table %s: %w", marker.tableName, err)
		}
		if count != 1 {
			return fmt.Errorf("schema object %s replaced table", marker.tableName)
		}
	}

	return nil
}

func removeMigrationTableMarkers(
	ctx context.Context,
	database *sql.DB,
	markers []migrationTableMarker,
) error {
	for _, marker := range markers {
		if _, err := database.ExecContext(
			ctx,
			"DROP TRIGGER IF EXISTS "+quoteMigrationIdentifier(marker.triggerName),
		); err != nil {
			return fmt.Errorf("remove marker for table %s: %w", marker.tableName, err)
		}
	}
	return nil
}

func quoteMigrationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func readMigrationSchema(ctx context.Context, database *sql.DB) (migrationSchemaSnapshot, error) {
	rows, err := database.QueryContext(ctx, `
SELECT type, name, sql
FROM sqlite_schema
WHERE type IN ('table', 'view')
  AND name NOT LIKE 'sqlite\_%' ESCAPE '\'
ORDER BY type, name
`)
	if err != nil {
		return nil, fmt.Errorf("list schema objects: %w", err)
	}
	type identity struct {
		kind    string
		name    string
		virtual bool
	}
	identities := make([]identity, 0)
	for rows.Next() {
		var current identity
		var definition string
		if err := rows.Scan(&current.kind, &current.name, &definition); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan schema object: %w", err)
		}
		current.virtual = strings.HasPrefix(strings.ToUpper(strings.TrimSpace(definition)), "CREATE VIRTUAL TABLE")
		identities = append(identities, current)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate schema objects: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close schema objects: %w", err)
	}

	snapshot := make(migrationSchemaSnapshot, len(identities))
	for _, current := range identities {
		object, err := readMigrationSchemaObject(ctx, database, current.kind, current.name, current.virtual)
		if err != nil {
			return nil, err
		}
		snapshot[current.name] = object
	}

	return snapshot, nil
}

func readMigrationSchemaObject(
	ctx context.Context,
	database *sql.DB,
	kind string,
	name string,
	virtual bool,
) (migrationSchemaObject, error) {
	columns, err := readMigrationSchemaColumns(ctx, database, name)
	if err != nil {
		return migrationSchemaObject{}, err
	}
	object := migrationSchemaObject{kind: kind, virtual: virtual, columns: columns}
	if kind == "view" {
		return object, nil
	}
	object.foreignKeys, err = readMigrationSchemaForeignKeys(ctx, database, name)
	if err != nil {
		return migrationSchemaObject{}, err
	}
	object.uniques, err = readMigrationSchemaUniques(ctx, database, name)
	if err != nil {
		return migrationSchemaObject{}, err
	}

	return object, nil
}

func readMigrationSchemaColumns(
	ctx context.Context,
	database *sql.DB,
	objectName string,
) ([]migrationSchemaColumn, error) {
	rows, err := database.QueryContext(ctx, `
SELECT cid, name, type, "notnull", dflt_value, pk, hidden
FROM pragma_table_xinfo(?)
ORDER BY cid
`, objectName)
	if err != nil {
		return nil, fmt.Errorf("read %s columns: %w", objectName, err)
	}
	defer func() { _ = rows.Close() }()

	columns := make([]migrationSchemaColumn, 0)
	for rows.Next() {
		var column migrationSchemaColumn
		if err := rows.Scan(
			&column.cid,
			&column.name,
			&column.columnType,
			&column.notNull,
			&column.defaultSQL,
			&column.primaryKey,
			&column.hidden,
		); err != nil {
			return nil, fmt.Errorf("scan %s column: %w", objectName, err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s columns: %w", objectName, err)
	}

	return columns, nil
}

func readMigrationSchemaForeignKeys(
	ctx context.Context,
	database *sql.DB,
	tableName string,
) ([]migrationSchemaForeignKey, error) {
	rows, err := database.QueryContext(ctx, `
SELECT id, seq, "table", "from", "to", on_update, on_delete, match
FROM pragma_foreign_key_list(?)
ORDER BY id, seq
`, tableName)
	if err != nil {
		return nil, fmt.Errorf("read %s foreign keys: %w", tableName, err)
	}
	defer func() { _ = rows.Close() }()

	foreignKeys := make([]migrationSchemaForeignKey, 0)
	lastID := -1
	for rows.Next() {
		var id, sequence int
		var referencedTable, local, onUpdate, onDelete, match string
		var referenced sql.NullString
		if err := rows.Scan(
			&id,
			&sequence,
			&referencedTable,
			&local,
			&referenced,
			&onUpdate,
			&onDelete,
			&match,
		); err != nil {
			return nil, fmt.Errorf("scan %s foreign key: %w", tableName, err)
		}
		if id != lastID {
			foreignKeys = append(foreignKeys, migrationSchemaForeignKey{
				referencedTable: referencedTable,
				onUpdate:        onUpdate,
				onDelete:        onDelete,
				match:           match,
			})
			lastID = id
		}
		current := &foreignKeys[len(foreignKeys)-1]
		if sequence != len(current.columns) {
			return nil, fmt.Errorf("%s foreign key %d has sequence %d", tableName, id, sequence)
		}
		current.columns = append(current.columns, migrationSchemaForeignKeyColumn{
			local:      local,
			referenced: referenced,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s foreign keys: %w", tableName, err)
	}
	sort.Slice(foreignKeys, func(left, right int) bool {
		return fmt.Sprint(foreignKeys[left]) < fmt.Sprint(foreignKeys[right])
	})

	return foreignKeys, nil
}

func readMigrationSchemaUniques(
	ctx context.Context,
	database *sql.DB,
	tableName string,
) ([]migrationSchemaUnique, error) {
	rows, err := database.QueryContext(ctx, `
SELECT name
FROM pragma_index_list(?)
WHERE "unique" = 1 AND origin = 'u'
ORDER BY seq
`, tableName)
	if err != nil {
		return nil, fmt.Errorf("read %s unique constraints: %w", tableName, err)
	}
	indexNames := make([]string, 0)
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan %s unique constraint: %w", tableName, err)
		}
		indexNames = append(indexNames, indexName)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate %s unique constraints: %w", tableName, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close %s unique constraints: %w", tableName, err)
	}

	uniques := make([]migrationSchemaUnique, 0, len(indexNames))
	for _, indexName := range indexNames {
		unique, err := readMigrationSchemaUnique(ctx, database, indexName)
		if err != nil {
			return nil, err
		}
		uniques = append(uniques, unique)
	}
	sort.Slice(uniques, func(left, right int) bool {
		return fmt.Sprint(uniques[left]) < fmt.Sprint(uniques[right])
	})

	return uniques, nil
}

func readMigrationSchemaUnique(
	ctx context.Context,
	database *sql.DB,
	indexName string,
) (migrationSchemaUnique, error) {
	rows, err := database.QueryContext(ctx, `
SELECT cid, name, "desc", coll
FROM pragma_index_xinfo(?)
WHERE key = 1
ORDER BY seqno
`, indexName)
	if err != nil {
		return migrationSchemaUnique{}, fmt.Errorf("read unique constraint %s: %w", indexName, err)
	}
	defer func() { _ = rows.Close() }()

	var unique migrationSchemaUnique
	for rows.Next() {
		var column migrationSchemaIndexedColumn
		if err := rows.Scan(
			&column.cid,
			&column.name,
			&column.descending,
			&column.collation,
		); err != nil {
			return migrationSchemaUnique{}, fmt.Errorf("scan unique constraint %s: %w", indexName, err)
		}
		unique.columns = append(unique.columns, column)
	}
	if err := rows.Err(); err != nil {
		return migrationSchemaUnique{}, fmt.Errorf("iterate unique constraint %s: %w", indexName, err)
	}

	return unique, nil
}

func compareMigrationSchemas(before, after migrationSchemaSnapshot) error {
	names := make([]string, 0, len(before))
	for name := range before {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		previous := before[name]
		current, survives := after[name]
		if !survives {
			continue
		}
		if current.kind != previous.kind {
			return fmt.Errorf("schema object %s changed from %s to %s", name, previous.kind, current.kind)
		}
		if len(current.columns) < len(previous.columns) {
			return fmt.Errorf("schema object %s lost columns", name)
		}
		if !reflect.DeepEqual(current.columns[:len(previous.columns)], previous.columns) {
			return fmt.Errorf("schema object %s changed existing columns", name)
		}

		if previous.kind == "table" {
			oldColumns := make(map[string]struct{}, len(previous.columns))
			for _, column := range previous.columns {
				oldColumns[column.name] = struct{}{}
			}
			currentForeignKeys := migrationForeignKeysTouching(current.foreignKeys, oldColumns)
			if !reflect.DeepEqual(currentForeignKeys, previous.foreignKeys) {
				return fmt.Errorf("schema object %s changed existing foreign keys", name)
			}
			currentUniques := migrationUniquesTouching(current.uniques, oldColumns)
			if !reflect.DeepEqual(currentUniques, previous.uniques) {
				return fmt.Errorf("schema object %s changed existing unique constraints", name)
			}
		}
	}

	return nil
}

func migrationForeignKeysTouching(
	foreignKeys []migrationSchemaForeignKey,
	columns map[string]struct{},
) []migrationSchemaForeignKey {
	filtered := make([]migrationSchemaForeignKey, 0, len(foreignKeys))
	for _, foreignKey := range foreignKeys {
		for _, column := range foreignKey.columns {
			if _, exists := columns[column.local]; exists {
				filtered = append(filtered, foreignKey)
				break
			}
		}
	}
	return filtered
}

func migrationUniquesTouching(
	uniques []migrationSchemaUnique,
	columns map[string]struct{},
) []migrationSchemaUnique {
	filtered := make([]migrationSchemaUnique, 0, len(uniques))
	for _, unique := range uniques {
		for _, column := range unique.columns {
			if column.name.Valid {
				if _, exists := columns[column.name.String]; exists {
					filtered = append(filtered, unique)
					break
				}
			}
		}
	}
	return filtered
}
