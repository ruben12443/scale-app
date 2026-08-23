package postgres

import (
	"context"
	"fmt"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

type receiptRepo Store

func (r *receiptRepo) Create(ctx context.Context, rc *domain.Receipt) error {
	s := (*Store)(r)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: create receipt: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	if _, err := tx.Exec(ctx,
		`INSERT INTO receipts (id, tenant_id, user_id, status, number, created_at, finalized_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		rc.ID, rc.TenantID, rc.UserID, rc.Status, nullableNumber(rc.Number), rc.CreatedAt, rc.FinalizedAt); err != nil {
		return fmt.Errorf("postgres: create receipt: %w", err)
	}

	if err := insertReceiptLines(ctx, tx, rc.ID, rc.TransactionIDs); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: create receipt: commit: %w", err)
	}
	return nil
}

func (r *receiptRepo) Get(ctx context.Context, id string) (*domain.Receipt, error) {
	s := (*Store)(r)
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, status, number, created_at, finalized_at
		 FROM receipts WHERE id = $1`, id)
	rc, err := scanReceipt(row)
	if err != nil {
		return nil, err
	}

	lines, err := selectReceiptLines(ctx, s.pool, id)
	if err != nil {
		return nil, err
	}
	rc.TransactionIDs = lines
	return rc, nil
}

func (r *receiptRepo) Update(ctx context.Context, rc *domain.Receipt) error {
	s := (*Store)(r)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: update receipt: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	tag, err := tx.Exec(ctx,
		`UPDATE receipts SET status = $2, number = $3, finalized_at = $4 WHERE id = $1`,
		rc.ID, rc.Status, nullableNumber(rc.Number), rc.FinalizedAt)
	if err != nil {
		return fmt.Errorf("postgres: update receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM receipt_lines WHERE receipt_id = $1`, rc.ID); err != nil {
		return fmt.Errorf("postgres: update receipt: clear lines: %w", err)
	}
	if err := insertReceiptLines(ctx, tx, rc.ID, rc.TransactionIDs); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: update receipt: commit: %w", err)
	}
	return nil
}

func (r *receiptRepo) ListOpenByUser(ctx context.Context, userID string) ([]*domain.Receipt, error) {
	s := (*Store)(r)
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, status, number, created_at, finalized_at
		 FROM receipts WHERE user_id = $1 AND status = $2 ORDER BY created_at`,
		userID, domain.ReceiptStatusDraft)
	if err != nil {
		return nil, fmt.Errorf("postgres: list open receipts: %w", err)
	}
	defer rows.Close()

	var out []*domain.Receipt
	for rows.Next() {
		rc, err := scanReceipt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list open receipts: %w", err)
	}

	// N+1 line lookups: acceptable for the current scale of one open draft
	// receipt per active user; revisit with a single joined/aggregated query
	// if this ever shows up in profiling.
	for _, rc := range out {
		lines, err := selectReceiptLines(ctx, s.pool, rc.ID)
		if err != nil {
			return nil, err
		}
		rc.TransactionIDs = lines
	}
	return out, nil
}

func (r *receiptRepo) NextReceiptNumber(ctx context.Context, tenantID string) (int, error) {
	s := (*Store)(r)
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO receipt_number_sequences (tenant_id, next_number) VALUES ($1, 1)
		 ON CONFLICT (tenant_id) DO NOTHING`, tenantID); err != nil {
		return 0, fmt.Errorf("postgres: init receipt number sequence: %w", err)
	}

	var allocated int
	err := s.pool.QueryRow(ctx,
		`UPDATE receipt_number_sequences SET next_number = next_number + 1
		 WHERE tenant_id = $1 RETURNING next_number - 1`, tenantID).Scan(&allocated)
	if err != nil {
		return 0, fmt.Errorf("postgres: allocate receipt number: %w", err)
	}
	return allocated, nil
}

func insertReceiptLines(ctx context.Context, tx pgxTx, receiptID string, transactionIDs []string) error {
	for i, txID := range transactionIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO receipt_lines (receipt_id, transaction_id, position) VALUES ($1, $2, $3)`,
			receiptID, txID, i); err != nil {
			return fmt.Errorf("postgres: insert receipt line: %w", err)
		}
	}
	return nil
}

func selectReceiptLines(ctx context.Context, q pgxQuerier, receiptID string) ([]string, error) {
	rows, err := q.Query(ctx,
		`SELECT transaction_id FROM receipt_lines WHERE receipt_id = $1 ORDER BY position`, receiptID)
	if err != nil {
		return nil, fmt.Errorf("postgres: select receipt lines: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var txID string
		if err := rows.Scan(&txID); err != nil {
			return nil, fmt.Errorf("postgres: scan receipt line: %w", err)
		}
		out = append(out, txID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: select receipt lines: %w", err)
	}
	return out, nil
}

func scanReceipt(row rowScanner) (*domain.Receipt, error) {
	var rc domain.Receipt
	var number *int
	err := row.Scan(&rc.ID, &rc.TenantID, &rc.UserID, &rc.Status, &number, &rc.CreatedAt, &rc.FinalizedAt)
	if err != nil {
		return nil, noRowsToNotFound(fmt.Errorf("postgres: get receipt: %w", err))
	}
	if number != nil {
		rc.Number = *number
	}
	return &rc, nil
}

// nullableNumber maps a zero receipt number (not yet finalized) to SQL NULL.
func nullableNumber(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
