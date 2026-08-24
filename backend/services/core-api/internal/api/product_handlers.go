package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/idgen"
	"scale-app/backend/services/core-api/internal/storage"
)

// ProductHandlers manages a tenant's product/price catalog. Listing is
// available to any authenticated user in the tenant (vendors need it to sell
// against); creating, updating, and deleting are admin-only, wired that way
// in Server.
type ProductHandlers struct {
	Products storage.ProductRepository
}

type productRequest struct {
	Name           string             `json:"name"`
	PricingType    domain.PricingType `json:"pricing_type"`
	UnitPriceCents int                `json:"unit_price_cents"`
}

// List returns every product in the caller's tenant.
func (h *ProductHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	products, err := h.Products.ListByTenant(r.Context(), actor.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list products: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, products)
}

// Create adds a product to the caller's tenant.
func (h *ProductHandlers) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	req, ok := decodeProductRequest(w, r)
	if !ok {
		return
	}

	product := &domain.Product{
		ID:             idgen.New(),
		TenantID:       actor.TenantID,
		Name:           req.Name,
		PricingType:    req.PricingType,
		UnitPriceCents: req.UnitPriceCents,
		CreatedAt:      time.Now().UTC(),
	}
	if err := h.Products.Create(r.Context(), product); err != nil {
		writeError(w, http.StatusInternalServerError, "store product: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

// Update changes a product's name/price. A target from a different tenant is
// reported as not found.
func (h *ProductHandlers) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	existing, ok := h.getOwnedProduct(w, r, actor.TenantID)
	if !ok {
		return
	}

	req, ok := decodeProductRequest(w, r)
	if !ok {
		return
	}

	existing.Name = req.Name
	existing.PricingType = req.PricingType
	existing.UnitPriceCents = req.UnitPriceCents
	if err := h.Products.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "update product: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

// Delete removes a product from the caller's tenant. A target from a
// different tenant is reported as not found.
func (h *ProductHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	if _, ok := h.getOwnedProduct(w, r, actor.TenantID); !ok {
		return
	}

	if err := h.Products.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, "delete product: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandlers) getOwnedProduct(w http.ResponseWriter, r *http.Request, tenantID string) (*domain.Product, bool) {
	product, err := h.Products.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product not found")
			return nil, false
		}
		writeError(w, http.StatusInternalServerError, "get product: "+err.Error())
		return nil, false
	}
	if product.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "product not found")
		return nil, false
	}
	return product, true
}

func decodeProductRequest(w http.ResponseWriter, r *http.Request) (productRequest, bool) {
	var req productRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return productRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return productRequest{}, false
	}
	if req.PricingType != domain.PricingPerKg && req.PricingType != domain.PricingPerPiece {
		writeError(w, http.StatusBadRequest, `pricing_type must be "per_kg" or "per_piece"`)
		return productRequest{}, false
	}
	if req.UnitPriceCents < 0 {
		writeError(w, http.StatusBadRequest, "unit_price_cents must not be negative")
		return productRequest{}, false
	}
	return req, true
}
