package api

import (
	"net/http"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

// ReceiptHandlers manages the caller's current draft receipt: viewing it
// with its line items resolved, and removing a line to correct a mistake.
// Finalizing a receipt is handled separately (see receipt_finalize.go)
// alongside generating its printable/emailable output.
type ReceiptHandlers struct {
	Receipts     storage.ReceiptRepository
	Transactions storage.TransactionRepository
}

// receiptResponse presents a receipt with its transaction lines resolved,
// since a client has no other way to look up transactions by ID.
type receiptResponse struct {
	*domain.Receipt
	Lines []*domain.Transaction `json:"lines"`
}

func (h *ReceiptHandlers) resolve(r *http.Request, receipt *domain.Receipt) (receiptResponse, error) {
	lines := make([]*domain.Transaction, 0, len(receipt.TransactionIDs))
	for _, id := range receipt.TransactionIDs {
		tx, err := h.Transactions.Get(r.Context(), id)
		if err != nil {
			return receiptResponse{}, err
		}
		lines = append(lines, tx)
	}
	return receiptResponse{Receipt: receipt, Lines: lines}, nil
}

// GetCurrent returns the caller's open draft receipt, creating one if none
// exists yet.
func (h *ReceiptHandlers) GetCurrent(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	receipt, err := getOrCreateDraftReceipt(r, h.Receipts, actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get draft receipt: "+err.Error())
		return
	}
	resp, err := h.resolve(r, receipt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve receipt lines: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// RemoveLine drops one transaction from the caller's open draft receipt,
// without deleting the underlying transaction record — it stays as an
// audit trail of what the scale actually measured, just no longer part of
// this receipt.
func (h *ReceiptHandlers) RemoveLine(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	open, err := h.Receipts.ListOpenByUser(r.Context(), actor.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list draft receipts: "+err.Error())
		return
	}
	if len(open) == 0 {
		writeError(w, http.StatusNotFound, "no draft receipt")
		return
	}
	receipt := open[0]

	removed, err := receipt.RemoveLine(r.PathValue("transactionId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "remove line: "+err.Error())
		return
	}
	if !removed {
		writeError(w, http.StatusNotFound, "transaction is not a line on the current receipt")
		return
	}

	if err := h.Receipts.Update(r.Context(), receipt); err != nil {
		writeError(w, http.StatusInternalServerError, "update draft receipt: "+err.Error())
		return
	}

	resp, err := h.resolve(r, receipt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve receipt lines: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
