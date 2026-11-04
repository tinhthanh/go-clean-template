// Package patient implements the Postgres-backed Patient repository. A
// page's total and summary are worked out over every match, not the page.
package patient

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

const (
	_columns = "id::text, code, workspace, name, COALESCE(gender, ''), birth, phone, cccd, email, address, occupation, note, tags, created_at, updated_at"
	// The clinic's day, whatever the server's zone.
	_clinicDay = "(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date = ?::date"
	// A masked number's digits match only its last three, as the mask shows them.
	_shownDigits = 3
)

// Repo -.
type Repo struct {
	*postgres.Postgres
}

// New returns a Patient repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.PatientRepo {
	return newTraced(&Repo{pg})
}

// List -.
func (r *Repo) List(ctx context.Context, q *entity.PatientQuery) (entity.PatientPage, error) {
	where := filter(q)

	var page entity.PatientPage

	sql, args, err := r.Builder.
		Select("count(*)", "count(*) FILTER (WHERE gender = 'male')", "count(*) FILTER (WHERE gender = 'female')").
		From("patients").Where(where).ToSql()
	if err != nil {
		return page, fmt.Errorf("PatientRepo - List - summary: %w", err)
	}

	if err = r.Pool.QueryRow(ctx, sql, args...).Scan(&page.Total, &page.Male, &page.Female); err != nil {
		return page, fmt.Errorf("PatientRepo - List - summary - QueryRow: %w", err)
	}

	sql, args, err = r.Builder.Select(_columns).From("patients").Where(where).
		OrderBy(order(q)).Limit(q.Limit).Offset(q.Offset).ToSql()
	if err != nil {
		return page, fmt.Errorf("PatientRepo - List - rows: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return page, fmt.Errorf("PatientRepo - List - Query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		p, scanErr := scan(rows)
		if scanErr != nil {
			return page, scanErr
		}

		page.Items = append(page.Items, p)
	}

	if err = rows.Err(); err != nil {
		return page, fmt.Errorf("PatientRepo - List - rows.Err: %w", err)
	}

	return page, nil
}

// filter is the query's WHERE: the branches it covers, a day, a tag and a search.
func filter(q *entity.PatientQuery) sq.And {
	where := sq.And{}

	if q.MadeAt != nil {
		where = append(where, sq.Eq{"workspace": q.MadeAt})
	}

	if q.Tag != "" {
		where = append(where, sq.Expr("? = ANY(tags)", q.Tag))
	}

	if q.Created != "" {
		where = append(where, sq.Expr(_clinicDay, q.Created))
	}

	if q.Q != "" {
		where = append(where, search(q))
	}

	return where
}

func search(q *entity.PatientQuery) sq.Or {
	or := sq.Or{sq.ILike{"name": "%" + q.Q + "%"}, sq.ILike{"code": "%" + q.Q + "%"}}

	digits := onlyDigits(q.Q)
	if digits == "" {
		return or
	}

	for _, col := range []string{"phone", "cccd"} {
		number := "regexp_replace(" + col + ", '\\D', '', 'g')"

		switch {
		case !q.PhoneDigitsOnly:
			or = append(or, sq.Expr(number+" LIKE ?", "%"+digits+"%"))
		case len(digits) <= _shownDigits:
			or = append(or, sq.Expr(number+" LIKE ?", "%"+digits))
		}
	}

	return or
}

func onlyDigits(s string) string {
	out := make([]rune, 0, len(s))

	for _, r := range s {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}

	return string(out)
}

func order(q *entity.PatientQuery) string {
	dir := " ASC"
	if q.Desc {
		dir = " DESC"
	}

	switch q.Sort {
	case "code", "name":
		return q.Sort + dir + ", id"
	default:
		return "created_at DESC, id"
	}
}

// Get -.
func (r *Repo) Get(ctx context.Context, id string) (entity.Patient, error) {
	sql, args, err := r.Builder.Select(_columns).From("patients").Where(sq.Eq{"id::text": id}).ToSql()
	if err != nil {
		return entity.Patient{}, fmt.Errorf("PatientRepo - Get - r.Builder: %w", err)
	}

	p, err := scan(r.Pool.QueryRow(ctx, sql, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Patient{}, entity.ErrNotFound
	}

	return p, err
}

// Store issues the code from the counter: KH-000001 on, never reused.
func (r *Repo) Store(ctx context.Context, p *entity.Patient) error {
	err := r.Pool.QueryRow(ctx, `
INSERT INTO patients (code, workspace, name, gender, birth, phone, cccd, email, address, occupation, note, tags)
VALUES ('KH-' || lpad(nextval('patient_code_seq')::text, 6, '0'), $1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id::text, code, created_at, updated_at`,
		p.Workspace, p.Name, p.Gender, p.Birth, p.Phone, p.CCCD, p.Email, p.Address, p.Occupation, p.Note, tagsOf(p),
	).Scan(&p.ID, &p.Code, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("PatientRepo - Store - QueryRow: %w", err)
	}

	return nil
}

// Update -.
func (r *Repo) Update(ctx context.Context, p *entity.Patient) error {
	err := r.Pool.QueryRow(ctx, `
UPDATE patients SET name = $2, gender = NULLIF($3, ''), birth = $4, phone = $5, cccd = $6, email = $7,
       address = $8, occupation = $9, note = $10, updated_at = now()
WHERE id::text = $1
RETURNING updated_at`,
		p.ID, p.Name, p.Gender, p.Birth, p.Phone, p.CCCD, p.Email, p.Address, p.Occupation, p.Note,
	).Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("PatientRepo - Update - QueryRow: %w", err)
	}

	return nil
}

func tagsOf(p *entity.Patient) []string {
	if p.Tags == nil {
		return []string{}
	}

	return p.Tags
}

func scan(row pgx.Row) (entity.Patient, error) {
	var p entity.Patient

	err := row.Scan(&p.ID, &p.Code, &p.Workspace, &p.Name, &p.Gender, &p.Birth, &p.Phone, &p.CCCD, &p.Email,
		&p.Address, &p.Occupation, &p.Note, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, pgx.ErrNoRows
		}

		return p, fmt.Errorf("PatientRepo - scan: %w", err)
	}

	return p, nil
}
