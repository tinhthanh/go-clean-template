// Package audit implements the Postgres-backed log: lines are only added
// (the table's triggers refuse an UPDATE, DELETE or TRUNCATE).
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const _insert = `
INSERT INTO audit_log (actor_id, actor_name, branch, action, resource, target, rows, changes)
VALUES (NULLIF($1, '')::uuid, $2, NULLIF($3, ''), $4, $5, $6, NULLIF($7, 0), $8)`

// The same actor's same line on the same target, within the window.
const _recent = `
SELECT EXISTS (SELECT 1 FROM audit_log
WHERE actor_id = NULLIF($1, '')::uuid AND action = $2 AND resource = $3 AND target = $4 AND at > now() - $5::interval)`

// Repo -.
type Repo struct {
	*postgres.Postgres
}

// New returns an Audit repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.AuditRepo {
	return newTraced(&Repo{pg})
}

// Write -.
func (r *Repo) Write(ctx context.Context, line *entity.AuditLine) error {
	return write(ctx, r.Pool, line)
}

// WriteOnce holds a lock on the line's key for the transaction, so two
// looks at once still make one line.
func (r *Repo) WriteOnce(ctx context.Context, line *entity.AuditLine, within time.Duration) (bool, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("AuditRepo - WriteOnce - r.Pool.Begin: %w", err)
	}

	defer rollback(ctx, tx)

	key := line.ActorID + "|" + line.Action + "|" + line.Resource + "|" + line.Target
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", key); err != nil {
		return false, fmt.Errorf("AuditRepo - WriteOnce - lock: %w", err)
	}

	var seen bool

	err = tx.QueryRow(ctx, _recent, line.ActorID, line.Action, line.Resource, line.Target, within.String()).Scan(&seen)
	if err != nil {
		return false, fmt.Errorf("AuditRepo - WriteOnce - recent: %w", err)
	}

	if seen {
		return false, nil
	}

	if writeErr := write(ctx, tx, line); writeErr != nil {
		return false, writeErr
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("AuditRepo - WriteOnce - tx.Commit: %w", err)
	}

	return true, nil
}

// rollback ends a transaction not committed; after a Commit it does nothing.
func rollback(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return // the connection is gone, and with it the transaction
	}
}

// execer is the pool or a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func write(ctx context.Context, db execer, line *entity.AuditLine) error {
	var changes []byte

	if len(line.Changes) > 0 {
		var err error
		if changes, err = json.Marshal(line.Changes); err != nil {
			return fmt.Errorf("AuditRepo - write - json.Marshal: %w", err)
		}
	}

	_, err := db.Exec(ctx, _insert, line.ActorID, line.ActorName, line.Branch, line.Action, line.Resource, line.Target, line.Rows, changes)
	if err != nil {
		return fmt.Errorf("AuditRepo - write - db.Exec: %w", err)
	}

	return nil
}
