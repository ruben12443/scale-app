package postgres

import (
	"context"
	"fmt"
	"time"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

type paymentRepo Store

func (r *paymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	s := (*Store)(r)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO payments (id, tenant_id, receipt_id, stripe_payment_intent_id, amount_cents, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.ID, p.TenantID, p.ReceiptID, p.StripePaymentIntentID, p.AmountCents, p.Status, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create payment: %w", err)
	}
	return nil
}

func (r *paymentRepo) Get(ctx context.Context, id string) (*domain.Payment, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, receipt_id, stripe_payment_intent_id, amount_cents, status, created_at, updated_at
		 FROM payments WHERE id = $1`, id)
	return scanPayment(row)
}

func (r *paymentRepo) GetByStripePaymentIntentID(ctx context.Context, intentID string) (*domain.Payment, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, receipt_id, stripe_payment_intent_id, amount_cents, status, created_at, updated_at
		 FROM payments WHERE stripe_payment_intent_id = $1`, intentID)
	return scanPayment(row)
}

func (r *paymentRepo) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error {
	s := (*Store)(r)
	tag, err := s.pool.Exec(ctx,
		`UPDATE payments SET status = $2, updated_at = $3 WHERE id = $1`,
		id, status, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("postgres: update payment status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func scanPayment(row rowScanner) (*domain.Payment, error) {
	var p domain.Payment
	err := row.Scan(&p.ID, &p.TenantID, &p.ReceiptID, &p.StripePaymentIntentID, &p.AmountCents, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, noRowsToNotFound(fmt.Errorf("postgres: get payment: %w", err))
	}
	return &p, nil
}
