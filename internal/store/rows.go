package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type rowScanner interface {
	Scan(...any) error
}

// The entity column constants (taskColumns, projectColumns, areaColumns) must
// stay bare column names joined by exactly ", " for this rewrite to hold.
func qualifiedColumns(alias, columns string) string {
	return alias + "." + strings.ReplaceAll(columns, ", ", ", "+alias+".")
}

func deleteRows(
	ctx context.Context,
	executor interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	},
	expected int64,
	statement string,
	arguments ...any,
) error {
	result, err := executor.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted row count: %w", err)
	}
	if deleted != expected {
		return fmt.Errorf("deleted %d rows, want %d", deleted, expected)
	}

	return nil
}

func collectRows[T any](
	rows *sql.Rows,
	scan func(rowScanner) (T, error),
	scanAction, iterateAction string,
) ([]T, error) {
	defer func() {
		_ = rows.Close()
	}()

	values := make([]T, 0)
	for rows.Next() {
		value, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", scanAction, err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", iterateAction, err)
	}

	return values, nil
}
