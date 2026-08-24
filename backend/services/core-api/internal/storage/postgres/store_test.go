package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

// These are integration tests against a real Postgres instance. They only
// run when DATABASE_URL is set (e.g. in an environment with the
// docker-compose Postgres service up), and skip cleanly otherwise so
// `go test ./...` doesn't require a live database.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping postgres integration test")
	}

	ctx := context.Background()
	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(s.Close)

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	// Start every test from a clean slate: these tests use fixed,
	// test-name-derived IDs, so leftover rows from a previous run against
	// the same database would collide on primary keys.
	_, err = s.pool.Exec(ctx, `TRUNCATE TABLE
		payments, receipt_lines, receipt_number_sequences, receipts, transactions, products, users, tenants
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	return s
}

func TestPostgresTenantRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: "t-" + t.Name(), Name: "Test Tenant", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Tenants().Create(ctx, tenant); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := s.Tenants().Get(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Name != tenant.Name {
		t.Fatalf("Name = %q, want %q", got.Name, tenant.Name)
	}
}

func TestPostgresTenantGetNotFound(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.Tenants().Get(context.Background(), "does-not-exist"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get returned %v, want %v", err, storage.ErrNotFound)
	}
}

func TestPostgresReceiptLineLifecycle(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: "t-" + t.Name(), Name: "Tenant", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Tenants().Create(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &domain.User{ID: "u-" + t.Name(), TenantID: tenant.ID, ZitadelSubjectID: "sub-" + t.Name(), Role: domain.RoleVendor, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	product := &domain.Product{ID: "p-" + t.Name(), TenantID: tenant.ID, Name: "Tomatoes", PricingType: domain.PricingPerKg, UnitPriceCents: 499, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Products().Create(ctx, product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	tx1 := &domain.Transaction{ID: "tx1-" + t.Name(), TenantID: tenant.ID, UserID: user.ID, ProductID: product.ID, ScaleID: "scale-1", WeightGrams: 1250, UnitPriceCents: 499, TotalPriceCents: 624, ScaleStatusCode: "1", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	tx2 := &domain.Transaction{ID: "tx2-" + t.Name(), TenantID: tenant.ID, UserID: user.ID, ProductID: product.ID, ScaleID: "scale-1", WeightGrams: 500, UnitPriceCents: 499, TotalPriceCents: 250, ScaleStatusCode: "1", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	for _, tx := range []*domain.Transaction{tx1, tx2} {
		if err := s.Transactions().Create(ctx, tx); err != nil {
			t.Fatalf("create transaction: %v", err)
		}
	}

	receipt := &domain.Receipt{ID: "r-" + t.Name(), TenantID: tenant.ID, UserID: user.ID, Status: domain.ReceiptStatusDraft, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	_ = receipt.AddLine(tx1.ID)
	if err := s.Receipts().Create(ctx, receipt); err != nil {
		t.Fatalf("create receipt: %v", err)
	}

	got, err := s.Receipts().Get(ctx, receipt.ID)
	if err != nil {
		t.Fatalf("get receipt: %v", err)
	}
	if len(got.TransactionIDs) != 1 || got.TransactionIDs[0] != tx1.ID {
		t.Fatalf("TransactionIDs = %v, want [%s]", got.TransactionIDs, tx1.ID)
	}

	_ = got.AddLine(tx2.ID)
	if err := s.Receipts().Update(ctx, got); err != nil {
		t.Fatalf("update receipt: %v", err)
	}

	number, err := s.Receipts().NextReceiptNumber(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}
	if err := got.Finalize(number, time.Now().UTC().Truncate(time.Microsecond)); err != nil {
		t.Fatalf("Finalize returned error: %v", err)
	}
	if err := s.Receipts().Update(ctx, got); err != nil {
		t.Fatalf("update finalized receipt: %v", err)
	}

	final, err := s.Receipts().Get(ctx, receipt.ID)
	if err != nil {
		t.Fatalf("get finalized receipt: %v", err)
	}
	if final.Status != domain.ReceiptStatusFinalized {
		t.Fatalf("Status = %q, want %q", final.Status, domain.ReceiptStatusFinalized)
	}
	if len(final.TransactionIDs) != 2 {
		t.Fatalf("TransactionIDs = %v, want 2 entries", final.TransactionIDs)
	}
	if final.Number != number {
		t.Fatalf("Number = %d, want %d", final.Number, number)
	}
}

func TestPostgresPerPieceTransactionRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: "t-" + t.Name(), Name: "Tenant", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Tenants().Create(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &domain.User{ID: "u-" + t.Name(), TenantID: tenant.ID, ZitadelSubjectID: "sub-" + t.Name(), Role: domain.RoleVendor, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	product := &domain.Product{ID: "p-" + t.Name(), TenantID: tenant.ID, Name: "Eggs (dozen)", PricingType: domain.PricingPerPiece, UnitPriceCents: 550, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Products().Create(ctx, product); err != nil {
		t.Fatalf("create product: %v", err)
	}

	tx := &domain.Transaction{
		ID: "tx-" + t.Name(), TenantID: tenant.ID, UserID: user.ID, ProductID: product.ID, ProductName: product.Name,
		PricingType: domain.PricingPerPiece, Quantity: 3, UnitPriceCents: 550, TotalPriceCents: 1650,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := s.Transactions().Create(ctx, tx); err != nil {
		t.Fatalf("create transaction: %v", err)
	}

	got, err := s.Transactions().Get(ctx, tx.ID)
	if err != nil {
		t.Fatalf("get transaction: %v", err)
	}
	if got.PricingType != domain.PricingPerPiece || got.Quantity != 3 || got.WeightGrams != 0 || got.ScaleID != "" {
		t.Fatalf("unexpected transaction: %+v", got)
	}
}

func TestPostgresReceiptReopenAndSentRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: "t-" + t.Name(), Name: "Tenant", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Tenants().Create(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &domain.User{ID: "u-" + t.Name(), TenantID: tenant.ID, ZitadelSubjectID: "sub-" + t.Name(), Role: domain.RoleVendor, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	receipt := &domain.Receipt{ID: "r-" + t.Name(), TenantID: tenant.ID, UserID: user.ID, Status: domain.ReceiptStatusDraft, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	_ = receipt.AddLine("tx-1")
	if err := s.Receipts().Create(ctx, receipt); err != nil {
		t.Fatalf("create receipt: %v", err)
	}

	number, err := s.Receipts().NextReceiptNumber(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}
	if err := receipt.Finalize(number, time.Now().UTC().Truncate(time.Microsecond)); err != nil {
		t.Fatalf("Finalize returned error: %v", err)
	}
	if err := s.Receipts().Update(ctx, receipt); err != nil {
		t.Fatalf("update finalized receipt: %v", err)
	}

	if err := receipt.Reopen(); err != nil {
		t.Fatalf("Reopen returned error: %v", err)
	}
	if err := s.Receipts().Update(ctx, receipt); err != nil {
		t.Fatalf("update reopened receipt: %v", err)
	}
	reopened, err := s.Receipts().Get(ctx, receipt.ID)
	if err != nil {
		t.Fatalf("get reopened receipt: %v", err)
	}
	if reopened.Status != domain.ReceiptStatusDraft || reopened.Number != 0 || reopened.FinalizedAt != nil {
		t.Fatalf("unexpected reopened receipt: %+v", reopened)
	}

	number2, err := s.Receipts().NextReceiptNumber(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}
	sentAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := reopened.Finalize(number2, sentAt); err != nil {
		t.Fatalf("re-finalize returned error: %v", err)
	}
	if err := reopened.MarkSent(sentAt, "customer@example.com"); err != nil {
		t.Fatalf("MarkSent returned error: %v", err)
	}
	if err := s.Receipts().Update(ctx, reopened); err != nil {
		t.Fatalf("update sent receipt: %v", err)
	}

	sent, err := s.Receipts().Get(ctx, receipt.ID)
	if err != nil {
		t.Fatalf("get sent receipt: %v", err)
	}
	if sent.Status != domain.ReceiptStatusSent {
		t.Fatalf("Status = %q, want %q", sent.Status, domain.ReceiptStatusSent)
	}
	if sent.SentTo != "customer@example.com" {
		t.Fatalf("SentTo = %q, want %q", sent.SentTo, "customer@example.com")
	}
	if sent.SentAt == nil || !sent.SentAt.Equal(sentAt) {
		t.Fatalf("SentAt = %v, want %v", sent.SentAt, sentAt)
	}
}

func TestPostgresNextReceiptNumberIsSequential(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: "t-" + t.Name(), Name: "Tenant", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Tenants().Create(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	n1, err := s.Receipts().NextReceiptNumber(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}
	n2, err := s.Receipts().NextReceiptNumber(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}
	if n2 != n1+1 {
		t.Fatalf("sequence = %d, %d, want consecutive", n1, n2)
	}
}

func TestPostgresPaymentRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tenant := &domain.Tenant{ID: "t-" + t.Name(), Name: "Tenant", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Tenants().Create(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &domain.User{ID: "u-" + t.Name(), TenantID: tenant.ID, ZitadelSubjectID: "sub-" + t.Name(), Role: domain.RoleVendor, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	receipt := &domain.Receipt{ID: "r-" + t.Name(), TenantID: tenant.ID, UserID: user.ID, Status: domain.ReceiptStatusFinalized, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Receipts().Create(ctx, receipt); err != nil {
		t.Fatalf("create receipt: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	p := &domain.Payment{
		ID: "pay-" + t.Name(), TenantID: tenant.ID, ReceiptID: receipt.ID,
		StripePaymentIntentID: "pi_" + t.Name(), AmountCents: 624,
		Status: domain.PaymentStatusPending, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Payments().Create(ctx, p); err != nil {
		t.Fatalf("create payment: %v", err)
	}

	got, err := s.Payments().Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if got.AmountCents != 624 || got.Status != domain.PaymentStatusPending {
		t.Fatalf("unexpected payment: %+v", got)
	}

	byIntent, err := s.Payments().GetByStripePaymentIntentID(ctx, p.StripePaymentIntentID)
	if err != nil {
		t.Fatalf("get payment by intent id: %v", err)
	}
	if byIntent.ID != p.ID {
		t.Fatalf("ID = %q, want %q", byIntent.ID, p.ID)
	}

	if err := s.Payments().UpdateStatus(ctx, p.ID, domain.PaymentStatusSucceeded); err != nil {
		t.Fatalf("update payment status: %v", err)
	}
	updated, err := s.Payments().Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if updated.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("Status = %q, want %q", updated.Status, domain.PaymentStatusSucceeded)
	}
	if !updated.UpdatedAt.After(now) {
		t.Fatalf("UpdatedAt = %v, want it to have advanced past %v", updated.UpdatedAt, now)
	}
}
