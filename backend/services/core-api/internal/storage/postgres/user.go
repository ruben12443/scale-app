package postgres

import (
	"context"
	"fmt"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

type userRepo Store

func (r *userRepo) Create(ctx context.Context, u *domain.User) error {
	s := (*Store)(r)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, zitadel_subject_id, display_name, email, role, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.ID, u.TenantID, u.ZitadelSubjectID, u.DisplayName, u.Email, u.Role, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

func (r *userRepo) Get(ctx context.Context, id string) (*domain.User, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, zitadel_subject_id, display_name, email, role, created_at
		 FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *userRepo) GetByZitadelSubject(ctx context.Context, subject string) (*domain.User, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, zitadel_subject_id, display_name, email, role, created_at
		 FROM users WHERE zitadel_subject_id = $1`, subject)
	return scanUser(row)
}

func (r *userRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.User, error) {
	s := (*Store)(r)
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, zitadel_subject_id, display_name, email, role, created_at
		 FROM users WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list users: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.User, 0)
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.ZitadelSubjectID, &u.DisplayName, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan user: %w", err)
		}
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list users: %w", err)
	}
	return out, nil
}

func (r *userRepo) Delete(ctx context.Context, id string) error {
	s := (*Store)(r)
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query),
// letting scanUser be shared between single-row and multi-row callers.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.TenantID, &u.ZitadelSubjectID, &u.DisplayName, &u.Email, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, noRowsToNotFound(fmt.Errorf("postgres: get user: %w", err))
	}
	return &u, nil
}
