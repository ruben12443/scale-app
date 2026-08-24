package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
	"scale-app/backend/services/core-api/internal/storage/memory"
)

func newTestReceiptHandlers(t *testing.T) (*ReceiptHandlers, storage.Store) {
	t.Helper()
	store := memory.New()
	return &ReceiptHandlers{Receipts: store.Receipts(), Transactions: store.Transactions()}, store
}

func TestReceiptHandlersGetCurrentCreatesEmptyDraft(t *testing.T) {
	h, _ := newTestReceiptHandlers(t)
	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}

	req := requestAs(actor, http.MethodGet, "/receipts/current", nil)
	rec := httptest.NewRecorder()
	h.GetCurrent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp receiptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != domain.ReceiptStatusDraft || len(resp.Lines) != 0 {
		t.Fatalf("unexpected receipt: %+v", resp)
	}
}

func TestReceiptHandlersGetCurrentResolvesLines(t *testing.T) {
	h, store := newTestReceiptHandlers(t)
	ctx := context.Background()

	tx := &domain.Transaction{ID: "tx1", TenantID: "t1", UserID: "vendor-1", WeightGrams: 1000, TotalPriceCents: 500}
	_ = store.Transactions().Create(ctx, tx)
	receipt := &domain.Receipt{ID: "r1", TenantID: "t1", UserID: "vendor-1", Status: domain.ReceiptStatusDraft}
	_ = receipt.AddLine("tx1")
	_ = store.Receipts().Create(ctx, receipt)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodGet, "/receipts/current", nil)
	rec := httptest.NewRecorder()
	h.GetCurrent(rec, req)

	var resp receiptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Lines) != 1 || resp.Lines[0].ID != "tx1" || resp.Lines[0].TotalPriceCents != 500 {
		t.Fatalf("unexpected resolved lines: %+v", resp.Lines)
	}
}

func TestReceiptHandlersRemoveLine(t *testing.T) {
	h, store := newTestReceiptHandlers(t)
	ctx := context.Background()

	tx1 := &domain.Transaction{ID: "tx1", TenantID: "t1", UserID: "vendor-1"}
	tx2 := &domain.Transaction{ID: "tx2", TenantID: "t1", UserID: "vendor-1"}
	_ = store.Transactions().Create(ctx, tx1)
	_ = store.Transactions().Create(ctx, tx2)
	receipt := &domain.Receipt{ID: "r1", TenantID: "t1", UserID: "vendor-1", Status: domain.ReceiptStatusDraft}
	_ = receipt.AddLine("tx1")
	_ = receipt.AddLine("tx2")
	_ = store.Receipts().Create(ctx, receipt)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodDelete, "/receipts/current/lines/tx1", nil)
	req.SetPathValue("transactionId", "tx1")
	rec := httptest.NewRecorder()
	h.RemoveLine(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp receiptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Lines) != 1 || resp.Lines[0].ID != "tx2" {
		t.Fatalf("unexpected remaining lines: %+v", resp.Lines)
	}

	// The removed transaction itself must still exist (audit trail).
	if _, err := store.Transactions().Get(ctx, "tx1"); err != nil {
		t.Fatalf("expected transaction tx1 to still exist after removal, got: %v", err)
	}
}

func TestReceiptHandlersRemoveLineNotOnReceipt(t *testing.T) {
	h, store := newTestReceiptHandlers(t)
	ctx := context.Background()
	receipt := &domain.Receipt{ID: "r1", TenantID: "t1", UserID: "vendor-1", Status: domain.ReceiptStatusDraft}
	_ = store.Receipts().Create(ctx, receipt)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodDelete, "/receipts/current/lines/does-not-exist", nil)
	req.SetPathValue("transactionId", "does-not-exist")
	rec := httptest.NewRecorder()
	h.RemoveLine(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestReceiptHandlersRemoveLineNoDraftReceipt(t *testing.T) {
	h, _ := newTestReceiptHandlers(t)
	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodDelete, "/receipts/current/lines/tx1", nil)
	req.SetPathValue("transactionId", "tx1")
	rec := httptest.NewRecorder()
	h.RemoveLine(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
