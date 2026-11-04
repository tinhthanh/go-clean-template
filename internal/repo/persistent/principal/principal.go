// Package principal implements the Postgres-backed Principal repository:
// a person with their permission group's grants and their workspaces.
package principal

import (
	"context"
	"errors"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

// A person, their group and their workspaces; no workspace listed is every one (NULL).
const _select = `
SELECT u.id, u.name, u.email, u.role, u.password_hash,
       COALESCE(g.id, ''), COALESCE(g.name, ''), COALESCE(g.grants, '{}'), COALESCE(g.doctor, false),
       array_agg(uw.workspace_id ORDER BY uw.workspace_id) FILTER (WHERE uw.workspace_id IS NOT NULL)
FROM users u
LEFT JOIN permission_groups g ON g.id = u.group_id
LEFT JOIN user_workspaces uw ON uw.user_id = u.id
WHERE %s = $1
GROUP BY u.id, g.id`

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
	p, _, err := r.one(ctx, "u.id::text", userID)

	return p, err
}

// ByEmail -.
func (r *Repo) ByEmail(ctx context.Context, email string) (entity.Principal, string, error) {
	return r.one(ctx, "lower(u.email)", email)
}

func (r *Repo) one(ctx context.Context, column, value string) (entity.Principal, string, error) {
	var (
		p    entity.Principal
		hash string
	)

	err := r.Pool.QueryRow(ctx, fmt.Sprintf(_select, column), value).
		Scan(&p.UserID, &p.Name, &p.Email, &p.Role, &hash, &p.Group, &p.GroupName, &p.Grants, &p.Doctor, &p.Workspaces)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Principal{}, "", entity.ErrUserNotFound
		}

		return entity.Principal{}, "", fmt.Errorf("PrincipalRepo - one - r.Pool.QueryRow: %w", err)
	}

	return p, hash, nil
}
