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

func newTestProductHandlers(t *testing.T) (*ProductHandlers, storage.Store) {
	t.Helper()
	store := memory.New()
	return &ProductHandlers{Products: store.Products()}, store
}

func TestProductHandlersCreateAndList(t *testing.T) {
	h, store := newTestProductHandlers(t)
	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}

	body, _ := json.Marshal(productRequest{Name: "Tomatoes", PricingType: domain.PricingPerKg, UnitPriceCents: 499})
	req := requestAs(actor, http.MethodPost, "/products", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var created domain.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.TenantID != "tenant-1" || created.UnitPriceCents != 499 {
		t.Fatalf("unexpected created product: %+v", created)
	}

	_ = store.Products().Create(context.Background(), &domain.Product{ID: "other", TenantID: "tenant-2", Name: "Other tenant's product"})

	listReq := requestAs(actor, http.MethodGet, "/products", nil)
	listRec := httptest.NewRecorder()
	h.List(listRec, listReq)

	var products []domain.Product
	if err := json.Unmarshal(listRec.Body.Bytes(), &products); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(products) != 1 || products[0].ID != created.ID {
		t.Fatalf("got %v, want only the caller's tenant's product", products)
	}
}

func TestProductHandlersCreateValidation(t *testing.T) {
	h, _ := newTestProductHandlers(t)
	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}

	cases := []productRequest{
		{Name: "", PricingType: domain.PricingPerKg, UnitPriceCents: 100},
		{Name: "Valid", PricingType: domain.PricingPerKg, UnitPriceCents: -1},
		{Name: "Valid", PricingType: "", UnitPriceCents: 100},
		{Name: "Valid", PricingType: "per_kilogram", UnitPriceCents: 100},
	}
	for _, tc := range cases {
		body, _ := json.Marshal(tc)
		req := requestAs(actor, http.MethodPost, "/products", body)
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case %+v: status = %d, want %d", tc, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestProductHandlersUpdate(t *testing.T) {
	h, store := newTestProductHandlers(t)
	ctx := context.Background()
	existing := &domain.Product{ID: "p1", TenantID: "tenant-1", Name: "Tomatoes", PricingType: domain.PricingPerKg, UnitPriceCents: 499}
	_ = store.Products().Create(ctx, existing)

	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	body, _ := json.Marshal(productRequest{Name: "Tomatoes (organic)", PricingType: domain.PricingPerKg, UnitPriceCents: 599})
	req := requestAs(actor, http.MethodPut, "/products/p1", body)
	req.SetPathValue("id", "p1")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got, err := store.Products().Get(ctx, "p1")
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if got.Name != "Tomatoes (organic)" || got.UnitPriceCents != 599 {
		t.Fatalf("unexpected stored product: %+v", got)
	}
}

func TestProductHandlersUpdateCrossTenantNotFound(t *testing.T) {
	h, store := newTestProductHandlers(t)
	ctx := context.Background()
	_ = store.Products().Create(ctx, &domain.Product{ID: "p1", TenantID: "tenant-2", Name: "Tomatoes"})

	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	body, _ := json.Marshal(productRequest{Name: "Hijacked", PricingType: domain.PricingPerKg, UnitPriceCents: 1})
	req := requestAs(actor, http.MethodPut, "/products/p1", body)
	req.SetPathValue("id", "p1")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestProductHandlersDelete(t *testing.T) {
	h, store := newTestProductHandlers(t)
	ctx := context.Background()
	_ = store.Products().Create(ctx, &domain.Product{ID: "p1", TenantID: "tenant-1", Name: "Tomatoes"})

	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	req := requestAs(actor, http.MethodDelete, "/products/p1", nil)
	req.SetPathValue("id", "p1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if _, err := store.Products().Get(ctx, "p1"); err == nil {
		t.Fatal("expected product to be deleted")
	}
}

func TestProductHandlersDeleteNotFound(t *testing.T) {
	h, _ := newTestProductHandlers(t)
	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	req := requestAs(actor, http.MethodDelete, "/products/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
