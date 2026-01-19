package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// AdvisoryLockTx acquires a transaction-scoped advisory lock.
func AdvisoryLockTx(ctx context.Context, tx pgx.Tx, key1, key2 int32) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1, $2)`, key1, key2)
	return err
}
