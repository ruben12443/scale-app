package postgres

import (
	"context"
	"fmt"

	"scale-app/backend/services/core-api/internal/domain"
)

type tenantRepo Store

func (r *tenantRepo) Create(ctx context.Context, t *domain.Tenant) error {
	s := (*Store)(r)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO tenants (id, name, created_at) VALUES ($1, $2, $3)`,
		t.ID, t.Name, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create tenant: %w", err)
	}
	return nil
}

func (r *tenantRepo) Get(ctx context.Context, id string) (*domain.Tenant, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, created_at FROM tenants WHERE id = $1`, id)
	var t domain.Tenant
	if err := row.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
		return nil, noRowsToNotFound(fmt.Errorf("postgres: get tenant: %w", err))
	}
	return &t, nil
}

func (r *tenantRepo) List(ctx context.Context) ([]*domain.Tenant, error) {
	s := (*Store)(r)
	rows, err := s.pool.Query(ctx, `SELECT id, name, created_at FROM tenants ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("postgres: list tenants: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Tenant, 0)
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan tenant: %w", err)
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list tenants: %w", err)
	}
	return out, nil
}
