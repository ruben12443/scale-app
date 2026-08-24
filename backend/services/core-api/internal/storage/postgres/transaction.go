package postgres

import (
	"context"
	"fmt"

	"scale-app/backend/services/core-api/internal/domain"
)

type transactionRepo Store

func (r *transactionRepo) Create(ctx context.Context, t *domain.Transaction) error {
	s := (*Store)(r)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO transactions
		 (id, tenant_id, user_id, product_id, product_name, scale_id, weight_grams, unit_price_cents, total_price_cents, scale_status_code, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		t.ID, t.TenantID, t.UserID, t.ProductID, t.ProductName, t.ScaleID, t.WeightGrams, t.UnitPriceCents, t.TotalPriceCents, t.ScaleStatusCode, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create transaction: %w", err)
	}
	return nil
}

func (r *transactionRepo) Get(ctx context.Context, id string) (*domain.Transaction, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, product_id, product_name, scale_id, weight_grams, unit_price_cents, total_price_cents, scale_status_code, created_at
		 FROM transactions WHERE id = $1`, id)
	return scanTransaction(row)
}

func (r *transactionRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Transaction, error) {
	s := (*Store)(r)
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, product_id, product_name, scale_id, weight_grams, unit_price_cents, total_price_cents, scale_status_code, created_at
		 FROM transactions WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list transactions: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Transaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list transactions: %w", err)
	}
	return out, nil
}

func scanTransaction(row rowScanner) (*domain.Transaction, error) {
	var t domain.Transaction
	err := row.Scan(&t.ID, &t.TenantID, &t.UserID, &t.ProductID, &t.ProductName, &t.ScaleID,
		&t.WeightGrams, &t.UnitPriceCents, &t.TotalPriceCents, &t.ScaleStatusCode, &t.CreatedAt)
	if err != nil {
		return nil, noRowsToNotFound(fmt.Errorf("postgres: get transaction: %w", err))
	}
	return &t, nil
}
