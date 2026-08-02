package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func withinImmediateTransaction(
	ctx context.Context,
	database *DB,
	noun string,
	apply func(*sql.Conn) error,
) error {
	connection, err := database.database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve %s transaction connection: %w", noun, err)
	}
	defer func() {
		_ = connection.Close()
	}()

	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin %s transaction: %w", noun, err)
	}
	transactionOpen := true
	defer func() {
		if transactionOpen {
			_, _ = connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	if err := apply(connection); err != nil {
		if _, rollbackErr := connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback %s transaction: %w", noun, rollbackErr))
		}
		transactionOpen = false
		return err
	}
	if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
		if _, rollbackErr := connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("commit %s transaction: %w", noun, err),
				fmt.Errorf("rollback %s transaction: %w", noun, rollbackErr),
			)
		}
		transactionOpen = false
		return fmt.Errorf("commit %s transaction: %w", noun, err)
	}
	transactionOpen = false

	return nil
}
