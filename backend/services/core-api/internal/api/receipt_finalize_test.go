package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/receipt"
	"scale-app/backend/services/core-api/internal/storage"
	"scale-app/backend/services/core-api/internal/storage/memory"
)

func newTestFinalizeHandlers(t *testing.T) (*ReceiptFinalizeHandlers, storage.Store, *receipt.FakeEmailSender) {
	t.Helper()
	store := memory.New()
	_ = store.Tenants().Create(context.Background(), &domain.Tenant{ID: "t1", Name: "Farmers Market Stand"})
	sender := &receipt.FakeEmailSender{}
	return &ReceiptFinalizeHandlers{
		Receipts:     store.Receipts(),
		Transactions: store.Transactions(),
		Tenants:      store.Tenants(),
		EmailSender:  sender,
	}, store, sender
}

func seedDraftReceiptWithOneLine(t *testing.T, store storage.Store, tenantID, userID string) *domain.Receipt {
	t.Helper()
	ctx := context.Background()
	tx := &domain.Transaction{ID: "tx1", TenantID: tenantID, UserID: userID, ProductName: "Tomatoes", WeightGrams: 1250, UnitPriceCents: 499, TotalPriceCents: 624}
	if err := store.Transactions().Create(ctx, tx); err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	rc := &domain.Receipt{ID: "r1", TenantID: tenantID, UserID: userID, Status: domain.ReceiptStatusDraft}
	_ = rc.AddLine("tx1")
	if err := store.Receipts().Create(ctx, rc); err != nil {
		t.Fatalf("create receipt: %v", err)
	}
	return rc
}

func TestFinalizeSuccess(t *testing.T) {
	h, store, _ := newTestFinalizeHandlers(t)
	seedDraftReceiptWithOneLine(t, store, "t1", "vendor-1")

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/current/finalize", nil)
	rec := httptest.NewRecorder()
	h.Finalize(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp finalizeReceiptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != domain.ReceiptStatusFinalized || resp.Number != 1 {
		t.Fatalf("unexpected finalized receipt: status=%q number=%d", resp.Status, resp.Number)
	}
	if resp.RenderedText == "" || resp.RenderedHTML == "" {
		t.Fatal("expected non-empty rendered text and HTML")
	}

	stored, err := store.Receipts().Get(context.Background(), "r1")
	if err != nil {
		t.Fatalf("get receipt: %v", err)
	}
	if stored.Status != domain.ReceiptStatusFinalized {
		t.Fatalf("stored receipt status = %q, want finalized", stored.Status)
	}
}

func TestFinalizeRejectsEmptyReceipt(t *testing.T) {
	h, store, _ := newTestFinalizeHandlers(t)
	rc := &domain.Receipt{ID: "r1", TenantID: "t1", UserID: "vendor-1", Status: domain.ReceiptStatusDraft}
	_ = store.Receipts().Create(context.Background(), rc)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/current/finalize", nil)
	rec := httptest.NewRecorder()
	h.Finalize(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestFinalizeRejectsMissingDraft(t *testing.T) {
	h, _, _ := newTestFinalizeHandlers(t)
	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/current/finalize", nil)
	rec := httptest.NewRecorder()
	h.Finalize(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestFinalizeNumbersAreSequentialPerTenant(t *testing.T) {
	h, store, _ := newTestFinalizeHandlers(t)
	seedDraftReceiptWithOneLine(t, store, "t1", "vendor-1")

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/current/finalize", nil)
	rec := httptest.NewRecorder()
	h.Finalize(rec, req)
	var first finalizeReceiptResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &first)

	// Start a second sale for the same vendor and finalize it too.
	ctx := context.Background()
	tx2 := &domain.Transaction{ID: "tx2", TenantID: "t1", UserID: "vendor-1", ProductName: "Potatoes"}
	_ = store.Transactions().Create(ctx, tx2)
	rc2 := &domain.Receipt{ID: "r2", TenantID: "t1", UserID: "vendor-1", Status: domain.ReceiptStatusDraft}
	_ = rc2.AddLine("tx2")
	_ = store.Receipts().Create(ctx, rc2)

	req2 := requestAs(actor, http.MethodPost, "/receipts/current/finalize", nil)
	rec2 := httptest.NewRecorder()
	h.Finalize(rec2, req2)
	var second finalizeReceiptResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &second)

	if first.Number != 1 || second.Number != 2 {
		t.Fatalf("receipt numbers = %d, %d, want 1, 2", first.Number, second.Number)
	}
}

func TestEmailSendsFinalizedReceipt(t *testing.T) {
	h, store, sender := newTestFinalizeHandlers(t)
	rc := seedDraftReceiptWithOneLine(t, store, "t1", "vendor-1")
	ctx := context.Background()
	_ = rc.Finalize(1, time.Now())
	_ = store.Receipts().Update(ctx, rc)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	body := []byte(`{"to":"customer@example.com"}`)
	req := requestAs(actor, http.MethodPost, "/receipts/r1/email", body)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()
	h.Email(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if len(sender.Sent) != 1 || sender.Sent[0].To != "customer@example.com" {
		t.Fatalf("Sent = %+v, want one email to customer@example.com", sender.Sent)
	}
}

func TestEmailRejectsDraftReceipt(t *testing.T) {
	h, store, _ := newTestFinalizeHandlers(t)
	seedDraftReceiptWithOneLine(t, store, "t1", "vendor-1")

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	body := []byte(`{"to":"customer@example.com"}`)
	req := requestAs(actor, http.MethodPost, "/receipts/r1/email", body)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()
	h.Email(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestEmailRejectsCrossTenantReceipt(t *testing.T) {
	h, store, _ := newTestFinalizeHandlers(t)
	rc := seedDraftReceiptWithOneLine(t, store, "other-tenant", "vendor-2")
	ctx := context.Background()
	_ = rc.Finalize(1, time.Now())
	_ = store.Receipts().Update(ctx, rc)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	body := []byte(`{"to":"customer@example.com"}`)
	req := requestAs(actor, http.MethodPost, "/receipts/r1/email", body)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()
	h.Email(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestEmailRejectsMissingTo(t *testing.T) {
	h, store, _ := newTestFinalizeHandlers(t)
	rc := seedDraftReceiptWithOneLine(t, store, "t1", "vendor-1")
	ctx := context.Background()
	_ = rc.Finalize(1, time.Now())
	_ = store.Receipts().Update(ctx, rc)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/r1/email", []byte(`{"to":""}`))
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()
	h.Email(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
