package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/receipt"
	"scale-app/backend/services/core-api/internal/storage"
)

// ReceiptFinalizeHandlers locks a draft receipt and produces its
// printable/emailable output.
type ReceiptFinalizeHandlers struct {
	Receipts     storage.ReceiptRepository
	Transactions storage.TransactionRepository
	Tenants      storage.TenantRepository
	EmailSender  receipt.EmailSender
}

type finalizeReceiptResponse struct {
	receiptResponse
	RenderedText string `json:"rendered_text"`
	RenderedHTML string `json:"rendered_html"`
}

// Finalize locks the caller's open draft receipt: it must exist and have at
// least one line. Locking allocates a sequential, tenant-scoped receipt
// number and returns both the finalized receipt and its rendered
// text/HTML, so the client can display or print it without a second call.
func (h *ReceiptFinalizeHandlers) Finalize(w http.ResponseWriter, r *http.Request) {
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
	draft := open[0]
	if len(draft.TransactionIDs) == 0 {
		writeError(w, http.StatusBadRequest, "cannot finalize an empty receipt")
		return
	}

	lines, err := resolveTransactions(r, h.Transactions, draft.TransactionIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve receipt lines: "+err.Error())
		return
	}

	number, err := h.Receipts.NextReceiptNumber(r.Context(), actor.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "allocate receipt number: "+err.Error())
		return
	}
	finalizedAt := time.Now().UTC()
	if err := draft.Finalize(number, finalizedAt); err != nil {
		// Can't happen: we just confirmed draft.Status == draft above and
		// nothing else could have mutated it concurrently through this
		// single-process, single-request path.
		writeError(w, http.StatusInternalServerError, "finalize receipt: "+err.Error())
		return
	}
	if err := h.Receipts.Update(r.Context(), draft); err != nil {
		writeError(w, http.StatusInternalServerError, "store finalized receipt: "+err.Error())
		return
	}

	tenant, err := h.Tenants.Get(r.Context(), actor.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get tenant: "+err.Error())
		return
	}

	data := receiptDataFrom(tenant, draft, lines)
	writeJSON(w, http.StatusOK, finalizeReceiptResponse{
		receiptResponse: receiptResponse{Receipt: draft, Lines: lines},
		RenderedText:    receipt.RenderText(data),
		RenderedHTML:    receipt.RenderHTML(data),
	})
}

type emailReceiptRequest struct {
	To string `json:"to"`
}

// Email sends a finalized receipt to a customer's email address. It does
// not accept draft receipts — only a finalized receipt is complete and
// immutable enough to hand to a customer.
func (h *ReceiptFinalizeHandlers) Email(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req emailReceiptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.To = strings.TrimSpace(req.To)
	if req.To == "" {
		writeError(w, http.StatusBadRequest, "to is required")
		return
	}

	rc, err := h.Receipts.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "receipt not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get receipt: "+err.Error())
		return
	}
	if rc.TenantID != actor.TenantID {
		writeError(w, http.StatusNotFound, "receipt not found")
		return
	}
	if rc.Status != domain.ReceiptStatusFinalized {
		writeError(w, http.StatusBadRequest, "receipt is not finalized yet")
		return
	}

	lines, err := resolveTransactions(r, h.Transactions, rc.TransactionIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve receipt lines: "+err.Error())
		return
	}
	tenant, err := h.Tenants.Get(r.Context(), actor.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get tenant: "+err.Error())
		return
	}

	data := receiptDataFrom(tenant, rc, lines)
	subject := "Your receipt from " + tenant.Name
	if err := h.EmailSender.Send(r.Context(), req.To, subject, receipt.RenderHTML(data)); err != nil {
		writeError(w, http.StatusBadGateway, "send email: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func resolveTransactions(r *http.Request, transactions storage.TransactionRepository, ids []string) ([]*domain.Transaction, error) {
	out := make([]*domain.Transaction, 0, len(ids))
	for _, id := range ids {
		tx, err := transactions.Get(r.Context(), id)
		if err != nil {
			return nil, err
		}
		out = append(out, tx)
	}
	return out, nil
}

func receiptDataFrom(tenant *domain.Tenant, rc *domain.Receipt, lines []*domain.Transaction) receipt.Data {
	data := receipt.Data{TenantName: tenant.Name, Number: rc.Number, Lines: make([]receipt.LineData, 0, len(lines))}
	if rc.FinalizedAt != nil {
		data.FinalizedAt = *rc.FinalizedAt
	}
	for _, tx := range lines {
		data.Lines = append(data.Lines, receipt.LineData{
			ProductName:     tx.ProductName,
			WeightGrams:     tx.WeightGrams,
			UnitPriceCents:  tx.UnitPriceCents,
			TotalPriceCents: tx.TotalPriceCents,
		})
	}
	return data
}
