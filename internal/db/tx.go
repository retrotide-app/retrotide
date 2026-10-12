package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type (
	DBTX interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
		PrepareContext(context.Context, string) (*sql.Stmt, error)
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}
)

func Joined(tx []*sql.Tx) *sql.Tx {
	if len(tx) > 0 && tx[0] != nil {
		return tx[0]
	}

	return nil
}

func TxOrDB(database *sql.DB, tx []*sql.Tx) DBTX {
	if joined := Joined(tx); joined != nil {
		return joined
	}

	return database
}

func withTx(ctx context.Context, database *sql.DB, opts *sql.TxOptions, fn func(*sql.Tx) error) (err error) {
	tx, err := database.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback tx: %w", rollbackErr))
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func WithTx(ctx context.Context, database *sql.DB, tx []*sql.Tx, fn func(*sql.Tx) error) error {
	if joined := Joined(tx); joined != nil {
		return fn(joined)
	}

	return withTx(ctx, database, nil, fn)
}

func WithReadTx(ctx context.Context, database *sql.DB, tx []*sql.Tx, fn func(*sql.Tx) error) error {
	if Joined(tx) != nil {
		return ErrInboundTx
	}

	return withTx(ctx, database, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, fn)
}
