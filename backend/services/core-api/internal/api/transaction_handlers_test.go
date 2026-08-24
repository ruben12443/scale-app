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

func newTestTransactionHandlers(t *testing.T) (*TransactionHandlers, storage.Store) {
	t.Helper()
	store := memory.New()
	return &TransactionHandlers{
		Products:     store.Products(),
		Transactions: store.Transactions(),
		Receipts:     store.Receipts(),
	}, store
}

func TestTransactionHandlersCreateOpensAndAppendsToDraftReceipt(t *testing.T) {
	h, store := newTestTransactionHandlers(t)
	ctx := context.Background()
	_ = store.Products().Create(ctx, &domain.Product{ID: "p1", TenantID: "t1", Name: "Tomatoes", PricePerKgCents: 499})

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	body, _ := json.Marshal(createTransactionRequest{
		ProductID: "p1", ScaleID: "scale-1", WeightGrams: 1250, UnitPriceCents: 499, TotalPriceCents: 624, ScaleStatusCode: "1",
	})
	req := requestAs(actor, http.MethodPost, "/transactions", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var resp createTransactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Transaction.WeightGrams != 1250 || resp.Transaction.TotalPriceCents != 624 {
		t.Fatalf("unexpected transaction: %+v", resp.Transaction)
	}

	receipt, err := store.Receipts().Get(ctx, resp.ReceiptID)
	if err != nil {
		t.Fatalf("get receipt: %v", err)
	}
	if receipt.Status != domain.ReceiptStatusDraft || len(receipt.TransactionIDs) != 1 || receipt.TransactionIDs[0] != resp.Transaction.ID {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}

	// A second transaction for the same user should append to the SAME
	// draft receipt, not open a new one.
	body2, _ := json.Marshal(createTransactionRequest{
		ProductID: "p1", ScaleID: "scale-1", WeightGrams: 500, UnitPriceCents: 499, TotalPriceCents: 250,
	})
	req2 := requestAs(actor, http.MethodPost, "/transactions", body2)
	rec2 := httptest.NewRecorder()
	h.Create(rec2, req2)

	var resp2 createTransactionResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp2.ReceiptID != resp.ReceiptID {
		t.Fatalf("second transaction opened receipt %q, want it reused %q", resp2.ReceiptID, resp.ReceiptID)
	}

	updated, err := store.Receipts().Get(ctx, resp.ReceiptID)
	if err != nil {
		t.Fatalf("get receipt: %v", err)
	}
	if len(updated.TransactionIDs) != 2 {
		t.Fatalf("TransactionIDs = %v, want 2 entries", updated.TransactionIDs)
	}
}

func TestTransactionHandlersCreateRejectsUnknownProduct(t *testing.T) {
	h, _ := newTestTransactionHandlers(t)
	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	body, _ := json.Marshal(createTransactionRequest{ProductID: "missing", ScaleID: "scale-1"})
	req := requestAs(actor, http.MethodPost, "/transactions", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransactionHandlersCreateRejectsCrossTenantProduct(t *testing.T) {
	h, store := newTestTransactionHandlers(t)
	ctx := context.Background()
	_ = store.Products().Create(ctx, &domain.Product{ID: "p1", TenantID: "other-tenant", Name: "Tomatoes"})

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	body, _ := json.Marshal(createTransactionRequest{ProductID: "p1", ScaleID: "scale-1"})
	req := requestAs(actor, http.MethodPost, "/transactions", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransactionHandlersCreateRejectsNegativeValues(t *testing.T) {
	h, store := newTestTransactionHandlers(t)
	_ = store.Products().Create(context.Background(), &domain.Product{ID: "p1", TenantID: "t1", Name: "Tomatoes"})

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	body, _ := json.Marshal(createTransactionRequest{ProductID: "p1", ScaleID: "scale-1", WeightGrams: -1})
	req := requestAs(actor, http.MethodPost, "/transactions", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
