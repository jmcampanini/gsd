package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func withinDeferredTransaction(
	ctx context.Context,
	database *DB,
	noun string,
	apply func(*sql.Conn) error,
) error {
	return withinTransaction(ctx, database.database, noun, "BEGIN", apply)
}

func withinImmediateTransaction(
	ctx context.Context,
	database *DB,
	noun string,
	apply func(*sql.Conn) error,
) error {
	return withinTransaction(ctx, database.database, noun, "BEGIN IMMEDIATE", apply)
}

func runInTransaction[S, T any](
	ctx context.Context,
	within func(context.Context, func(S) error) error,
	apply func(S) (T, error),
) (T, error) {
	var result T
	err := within(ctx, func(store S) error {
		var err error
		result, err = apply(store)
		return err
	})
	if err != nil {
		var zero T
		return zero, err
	}

	return result, nil
}

func withinTransaction(
	ctx context.Context,
	database *sql.DB,
	noun string,
	beginStatement string,
	apply func(*sql.Conn) error,
) error {
	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve %s transaction connection: %w", noun, err)
	}
	defer func() {
		_ = connection.Close()
	}()

	if _, err := connection.ExecContext(ctx, beginStatement); err != nil {
		return fmt.Errorf("begin %s transaction: %w", noun, err)
	}
	transactionOpen := true
	defer func() {
		if transactionOpen {
			_, _ = connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()
	rollback := func(operationErr error) error {
		_, err := connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		if err != nil {
			return errors.Join(operationErr, fmt.Errorf("rollback %s transaction: %w", noun, err))
		}
		transactionOpen = false
		return operationErr
	}

	if err := apply(connection); err != nil {
		return rollback(err)
	}
	if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
		return rollback(fmt.Errorf("commit %s transaction: %w", noun, err))
	}
	transactionOpen = false

	return nil
}
