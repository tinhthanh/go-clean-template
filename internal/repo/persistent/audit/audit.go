// Package audit implements the Postgres-backed log: lines are only added
// (the table's triggers refuse an UPDATE, DELETE or TRUNCATE).
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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
	return r.write(ctx, r.Pool, line)
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

	// The same actor's same line on the same target, within the window.
	sql, args, err := r.Builder.Select("count(*) > 0").From("audit_log").Where(sq.And{
		sq.Expr("actor_id = NULLIF(?, '')::uuid", line.ActorID),
		sq.Eq{"action": line.Action, "resource": line.Resource, "target": line.Target},
		sq.Expr("at > now() - ?::interval", within.String()),
	}).ToSql()
	if err != nil {
		return false, fmt.Errorf("AuditRepo - WriteOnce - r.Builder: %w", err)
	}

	var seen bool
	if err = tx.QueryRow(ctx, sql, args...).Scan(&seen); err != nil {
		return false, fmt.Errorf("AuditRepo - WriteOnce - recent: %w", err)
	}

	if seen {
		return false, nil
	}

	if writeErr := r.write(ctx, tx, line); writeErr != nil {
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

func (r *Repo) write(ctx context.Context, db execer, line *entity.AuditLine) error {
	var changes []byte

	if len(line.Changes) > 0 {
		var err error
		if changes, err = json.Marshal(line.Changes); err != nil {
			return fmt.Errorf("AuditRepo - write - json.Marshal: %w", err)
		}
	}

	sql, args, err := r.Builder.Insert("audit_log").
		Columns("actor_id", "actor_name", "branch", "action", "resource", "target", "rows", "changes").
		Values(sq.Expr("NULLIF(?, '')::uuid", line.ActorID), line.ActorName, sq.Expr("NULLIF(?, '')", line.Branch),
			line.Action, line.Resource, line.Target, sq.Expr("NULLIF(?, 0)", line.Rows), changes).
		ToSql()
	if err != nil {
		return fmt.Errorf("AuditRepo - write - r.Builder: %w", err)
	}

	if _, err = db.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("AuditRepo - write - db.Exec: %w", err)
	}

	return nil
}
