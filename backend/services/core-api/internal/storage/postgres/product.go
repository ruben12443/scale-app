package postgres

import (
	"context"
	"fmt"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

type productRepo Store

func (r *productRepo) Create(ctx context.Context, p *domain.Product) error {
	s := (*Store)(r)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO products (id, tenant_id, name, pricing_type, unit_price_cents, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.TenantID, p.Name, p.PricingType, p.UnitPriceCents, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create product: %w", err)
	}
	return nil
}

func (r *productRepo) Get(ctx context.Context, id string) (*domain.Product, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, pricing_type, unit_price_cents, created_at FROM products WHERE id = $1`, id)
	var p domain.Product
	if err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.PricingType, &p.UnitPriceCents, &p.CreatedAt); err != nil {
		return nil, noRowsToNotFound(fmt.Errorf("postgres: get product: %w", err))
	}
	return &p, nil
}

func (r *productRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Product, error) {
	s := (*Store)(r)
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, pricing_type, unit_price_cents, created_at
		 FROM products WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list products: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Product, 0)
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.PricingType, &p.UnitPriceCents, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan product: %w", err)
		}
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list products: %w", err)
	}
	return out, nil
}

func (r *productRepo) Update(ctx context.Context, p *domain.Product) error {
	s := (*Store)(r)
	tag, err := s.pool.Exec(ctx,
		`UPDATE products SET name = $2, pricing_type = $3, unit_price_cents = $4 WHERE id = $1`,
		p.ID, p.Name, p.PricingType, p.UnitPriceCents)
	if err != nil {
		return fmt.Errorf("postgres: update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (r *productRepo) Delete(ctx context.Context, id string) error {
	s := (*Store)(r)
	tag, err := s.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}
