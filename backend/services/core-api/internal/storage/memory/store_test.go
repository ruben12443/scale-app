package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

func TestTenantCreateGetList(t *testing.T) {
	s := New()
	ctx := context.Background()

	want := &domain.Tenant{ID: "t1", Name: "Farmers Market Stand", CreatedAt: time.Now()}
	if err := s.Tenants().Create(ctx, want); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := s.Tenants().Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Name != want.Name {
		t.Fatalf("Name = %q, want %q", got.Name, want.Name)
	}

	list, err := s.Tenants().List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d tenants, want 1", len(list))
	}
}

func TestTenantGetNotFound(t *testing.T) {
	s := New()
	if _, err := s.Tenants().Get(context.Background(), "missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get returned %v, want %v", err, storage.ErrNotFound)
	}
}

func TestUserGetByRauthySubjectAndDelete(t *testing.T) {
	s := New()
	ctx := context.Background()

	u := &domain.User{ID: "u1", TenantID: "t1", RauthySubjectID: "rauthy-sub-1", Role: domain.RoleVendor}
	if err := s.Users().Create(ctx, u); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := s.Users().GetByRauthySubject(ctx, "rauthy-sub-1")
	if err != nil {
		t.Fatalf("GetByRauthySubject returned error: %v", err)
	}
	if got.ID != "u1" {
		t.Fatalf("ID = %q, want %q", got.ID, "u1")
	}

	if err := s.Users().Delete(ctx, "u1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := s.Users().Get(ctx, "u1"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get after delete returned %v, want %v", err, storage.ErrNotFound)
	}
}

func TestUserListByTenantScoping(t *testing.T) {
	s := New()
	ctx := context.Background()
	_ = s.Users().Create(ctx, &domain.User{ID: "u1", TenantID: "t1"})
	_ = s.Users().Create(ctx, &domain.User{ID: "u2", TenantID: "t1"})
	_ = s.Users().Create(ctx, &domain.User{ID: "u3", TenantID: "t2"})

	got, err := s.Users().ListByTenant(ctx, "t1")
	if err != nil {
		t.Fatalf("ListByTenant returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d users for t1, want 2", len(got))
	}
}

func TestProductCreateUpdateDelete(t *testing.T) {
	s := New()
	ctx := context.Background()

	p := &domain.Product{ID: "p1", TenantID: "t1", Name: "Tomatoes", PricingType: domain.PricingPerKg, UnitPriceCents: 499}
	if err := s.Products().Create(ctx, p); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	p.UnitPriceCents = 549
	if err := s.Products().Update(ctx, p); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	got, err := s.Products().Get(ctx, "p1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.UnitPriceCents != 549 {
		t.Fatalf("UnitPriceCents = %d, want 549", got.UnitPriceCents)
	}

	if err := s.Products().Delete(ctx, "p1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := s.Products().Get(ctx, "p1"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get after delete returned %v, want %v", err, storage.ErrNotFound)
	}
}

func TestProductUpdateNotFound(t *testing.T) {
	s := New()
	err := s.Products().Update(context.Background(), &domain.Product{ID: "missing"})
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Update returned %v, want %v", err, storage.ErrNotFound)
	}
}

func TestTransactionCreateGetListByTenant(t *testing.T) {
	s := New()
	ctx := context.Background()

	tx1 := &domain.Transaction{ID: "tx1", TenantID: "t1", CreatedAt: time.Now()}
	tx2 := &domain.Transaction{ID: "tx2", TenantID: "t1", CreatedAt: time.Now().Add(time.Second)}
	tx3 := &domain.Transaction{ID: "tx3", TenantID: "t2", CreatedAt: time.Now()}
	for _, tx := range []*domain.Transaction{tx1, tx2, tx3} {
		if err := s.Transactions().Create(ctx, tx); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
	}

	got, err := s.Transactions().ListByTenant(ctx, "t1")
	if err != nil {
		t.Fatalf("ListByTenant returned error: %v", err)
	}
	if len(got) != 2 || got[0].ID != "tx1" || got[1].ID != "tx2" {
		t.Fatalf("got %v, want [tx1 tx2] in order", got)
	}
}

func TestReceiptRoundTripAndMutationIsolation(t *testing.T) {
	s := New()
	ctx := context.Background()

	r := &domain.Receipt{ID: "r1", TenantID: "t1", UserID: "u1", Status: domain.ReceiptStatusDraft}
	_ = r.AddLine("tx1")
	if err := s.Receipts().Create(ctx, r); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// Mutating the caller's copy after Create must not affect the stored copy.
	_ = r.AddLine("tx2")

	got, err := s.Receipts().Get(ctx, "r1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(got.TransactionIDs) != 1 || got.TransactionIDs[0] != "tx1" {
		t.Fatalf("stored TransactionIDs = %v, want [tx1] (mutation of caller's copy leaked into the store)", got.TransactionIDs)
	}

	_ = got.AddLine("tx2")
	if err := s.Receipts().Update(ctx, got); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	got2, err := s.Receipts().Get(ctx, "r1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(got2.TransactionIDs) != 2 {
		t.Fatalf("after Update, TransactionIDs = %v, want 2 entries", got2.TransactionIDs)
	}
}

func TestReceiptUpdateNotFound(t *testing.T) {
	s := New()
	err := s.Receipts().Update(context.Background(), &domain.Receipt{ID: "missing"})
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Update returned %v, want %v", err, storage.ErrNotFound)
	}
}

func TestReceiptListOpenByUserOnlyReturnsDrafts(t *testing.T) {
	s := New()
	ctx := context.Background()

	draft := &domain.Receipt{ID: "r1", UserID: "u1", Status: domain.ReceiptStatusDraft}
	finalized := &domain.Receipt{ID: "r2", UserID: "u1", Status: domain.ReceiptStatusFinalized}
	otherUser := &domain.Receipt{ID: "r3", UserID: "u2", Status: domain.ReceiptStatusDraft}
	for _, r := range []*domain.Receipt{draft, finalized, otherUser} {
		if err := s.Receipts().Create(ctx, r); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
	}

	got, err := s.Receipts().ListOpenByUser(ctx, "u1")
	if err != nil {
		t.Fatalf("ListOpenByUser returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("got %v, want only [r1]", got)
	}
}

func TestNextReceiptNumberIsSequentialPerTenant(t *testing.T) {
	s := New()
	ctx := context.Background()

	n1, err := s.Receipts().NextReceiptNumber(ctx, "t1")
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}
	n2, err := s.Receipts().NextReceiptNumber(ctx, "t1")
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}
	n3, err := s.Receipts().NextReceiptNumber(ctx, "t2")
	if err != nil {
		t.Fatalf("NextReceiptNumber returned error: %v", err)
	}

	if n1 != 1 || n2 != 2 {
		t.Fatalf("tenant t1 sequence = %d, %d, want 1, 2", n1, n2)
	}
	if n3 != 1 {
		t.Fatalf("tenant t2 first number = %d, want 1 (sequences are per-tenant)", n3)
	}
}

func TestPaymentCreateGetAndUpdateStatus(t *testing.T) {
	s := New()
	ctx := context.Background()

	p := &domain.Payment{ID: "pay1", TenantID: "t1", ReceiptID: "r1", StripePaymentIntentID: "pi_123", AmountCents: 624, Status: domain.PaymentStatusPending}
	if err := s.Payments().Create(ctx, p); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := s.Payments().Get(ctx, "pay1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.StripePaymentIntentID != "pi_123" {
		t.Fatalf("StripePaymentIntentID = %q, want %q", got.StripePaymentIntentID, "pi_123")
	}

	byIntent, err := s.Payments().GetByStripePaymentIntentID(ctx, "pi_123")
	if err != nil {
		t.Fatalf("GetByStripePaymentIntentID returned error: %v", err)
	}
	if byIntent.ID != "pay1" {
		t.Fatalf("ID = %q, want %q", byIntent.ID, "pay1")
	}

	if err := s.Payments().UpdateStatus(ctx, "pay1", domain.PaymentStatusSucceeded); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	updated, err := s.Payments().Get(ctx, "pay1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if updated.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("Status = %q, want %q", updated.Status, domain.PaymentStatusSucceeded)
	}
	if updated.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set after UpdateStatus")
	}
}

func TestPaymentGetNotFound(t *testing.T) {
	s := New()
	ctx := context.Background()
	if _, err := s.Payments().Get(ctx, "missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get returned %v, want %v", err, storage.ErrNotFound)
	}
	if _, err := s.Payments().GetByStripePaymentIntentID(ctx, "missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetByStripePaymentIntentID returned %v, want %v", err, storage.ErrNotFound)
	}
	if err := s.Payments().UpdateStatus(ctx, "missing", domain.PaymentStatusFailed); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("UpdateStatus returned %v, want %v", err, storage.ErrNotFound)
	}
}
