package api

import (
	"errors"
	"net/http"
	"time"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/idgen"
	"scale-app/backend/services/core-api/internal/storage"
)

// TransactionHandlers records scale-approved weigh events and, in the same
// action, locks them into the caller's current draft receipt — matching the
// app's "verify on screen, then tap to lock in" flow: there's no separate
// step to add a completed weigh to the receipt.
type TransactionHandlers struct {
	Products     storage.ProductRepository
	Transactions storage.TransactionRepository
	Receipts     storage.ReceiptRepository
}

type createTransactionRequest struct {
	ProductID string `json:"product_id"`

	// ScaleID, WeightGrams, and ScaleStatusCode apply only to a per-kg
	// product (weighed on a scale); Quantity applies only to a per-piece
	// product (counted, never touching a scale). Which fields are required
	// is determined by the referenced product's own PricingType, not by
	// the caller.
	ScaleID         string `json:"scale_id"`
	WeightGrams     int    `json:"weight_grams"`
	Quantity        int    `json:"quantity"`
	UnitPriceCents  int    `json:"unit_price_cents"`
	TotalPriceCents int    `json:"total_price_cents"`
	ScaleStatusCode string `json:"scale_status_code"`
}

type createTransactionResponse struct {
	Transaction *domain.Transaction `json:"transaction"`
	ReceiptID   string              `json:"receipt_id"`
}

// Create stores a transaction and appends it to the caller's open draft
// receipt, creating one if none exists yet.
func (h *TransactionHandlers) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req createTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.ProductID == "" {
		writeError(w, http.StatusBadRequest, "product_id is required")
		return
	}
	if req.WeightGrams < 0 || req.Quantity < 0 || req.UnitPriceCents < 0 || req.TotalPriceCents < 0 {
		writeError(w, http.StatusBadRequest, "weight_grams, quantity, unit_price_cents, and total_price_cents must not be negative")
		return
	}

	product, err := h.Products.Get(r.Context(), req.ProductID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "unknown product_id")
			return
		}
		writeError(w, http.StatusInternalServerError, "get product: "+err.Error())
		return
	}
	if product.TenantID != actor.TenantID {
		writeError(w, http.StatusBadRequest, "unknown product_id")
		return
	}

	switch product.PricingType {
	case domain.PricingPerKg:
		if req.ScaleID == "" {
			writeError(w, http.StatusBadRequest, "scale_id is required for a per-kg product")
			return
		}
		if req.WeightGrams <= 0 {
			writeError(w, http.StatusBadRequest, "weight_grams must be greater than zero for a per-kg product")
			return
		}
		if req.Quantity != 0 {
			writeError(w, http.StatusBadRequest, "quantity must not be set for a per-kg product")
			return
		}
	case domain.PricingPerPiece:
		if req.Quantity <= 0 {
			writeError(w, http.StatusBadRequest, "quantity must be greater than zero for a per-piece product")
			return
		}
		if req.WeightGrams != 0 || req.ScaleID != "" || req.ScaleStatusCode != "" {
			writeError(w, http.StatusBadRequest, "weight_grams, scale_id, and scale_status_code must not be set for a per-piece product")
			return
		}
	default:
		writeError(w, http.StatusInternalServerError, "product has an unknown pricing type")
		return
	}

	tx := &domain.Transaction{
		ID:              idgen.New(),
		TenantID:        actor.TenantID,
		UserID:          actor.ID,
		ProductID:       req.ProductID,
		ProductName:     product.Name,
		PricingType:     product.PricingType,
		ScaleID:         req.ScaleID,
		WeightGrams:     req.WeightGrams,
		Quantity:        req.Quantity,
		UnitPriceCents:  req.UnitPriceCents,
		TotalPriceCents: req.TotalPriceCents,
		ScaleStatusCode: req.ScaleStatusCode,
		CreatedAt:       time.Now().UTC(),
	}
	if err := h.Transactions.Create(r.Context(), tx); err != nil {
		writeError(w, http.StatusInternalServerError, "store transaction: "+err.Error())
		return
	}

	receipt, err := getOrCreateDraftReceipt(r, h.Receipts, actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get draft receipt: "+err.Error())
		return
	}
	// AddLine only fails if the receipt isn't a draft, which can't happen
	// here since getOrCreateDraftReceipt only ever returns draft receipts.
	_ = receipt.AddLine(tx.ID)
	if err := h.Receipts.Update(r.Context(), receipt); err != nil {
		writeError(w, http.StatusInternalServerError, "update draft receipt: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, createTransactionResponse{Transaction: tx, ReceiptID: receipt.ID})
}

// getOrCreateDraftReceipt returns the caller's single open draft receipt,
// creating one if none exists.
func getOrCreateDraftReceipt(r *http.Request, receipts storage.ReceiptRepository, actor *domain.User) (*domain.Receipt, error) {
	open, err := receipts.ListOpenByUser(r.Context(), actor.ID)
	if err != nil {
		return nil, err
	}
	if len(open) > 0 {
		return open[0], nil
	}

	receipt := &domain.Receipt{
		ID:        idgen.New(),
		TenantID:  actor.TenantID,
		UserID:    actor.ID,
		Status:    domain.ReceiptStatusDraft,
		CreatedAt: time.Now().UTC(),
	}
	if err := receipts.Create(r.Context(), receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}
