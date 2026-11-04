// Package principal implements the Postgres-backed Principal repository:
// a person with their permission group's grants and their workspaces.
package principal

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

// Repo -.
type Repo struct {
	*postgres.Postgres
}

// New returns a Principal repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.PrincipalRepo {
	return newTraced(&Repo{pg})
}

// ByID -.
func (r *Repo) ByID(ctx context.Context, userID string) (entity.Principal, error) {
	p, _, err := r.one(ctx, sq.Eq{"u.id::text": userID})

	return p, err
}

// ByEmail -.
func (r *Repo) ByEmail(ctx context.Context, email string) (entity.Principal, string, error) {
	return r.one(ctx, sq.Eq{"lower(u.email)": email})
}

// one is a person, their group and their workspaces; no workspace listed is every one (NULL).
func (r *Repo) one(ctx context.Context, where sq.Eq) (entity.Principal, string, error) {
	var (
		p    entity.Principal
		hash string
	)

	sql, args, err := r.Builder.
		Select("u.id::text", "u.name", "u.email", "u.role", "u.password_hash",
			"COALESCE(g.id, '')", "COALESCE(g.name, '')", "COALESCE(g.grants, '{}')", "COALESCE(g.doctor, false)",
			"array_agg(uw.workspace_id ORDER BY uw.workspace_id) FILTER (WHERE uw.workspace_id IS NOT NULL)").
		From("users u").
		LeftJoin("permission_groups g ON g.id = u.group_id").
		LeftJoin("user_workspaces uw ON uw.user_id = u.id").
		Where(where).
		GroupBy("u.id", "g.id").
		ToSql()
	if err != nil {
		return entity.Principal{}, "", fmt.Errorf("PrincipalRepo - one - r.Builder: %w", err)
	}

	err = r.Pool.QueryRow(ctx, sql, args...).
		Scan(&p.UserID, &p.Name, &p.Email, &p.Role, &hash, &p.Group, &p.GroupName, &p.Grants, &p.Doctor, &p.Workspaces)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Principal{}, "", entity.ErrUserNotFound
		}

		return entity.Principal{}, "", fmt.Errorf("PrincipalRepo - one - r.Pool.QueryRow: %w", err)
	}

	return p, hash, nil
}
