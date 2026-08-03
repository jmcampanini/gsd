package store

import (
	"database/sql"
	"fmt"
)

type rowScanner interface {
	Scan(...any) error
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
