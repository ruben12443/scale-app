package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	stripego "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/idgen"
	"scale-app/backend/services/core-api/internal/payment"
	"scale-app/backend/services/core-api/internal/storage"
)

// PaymentHandlers integrates the mobile app as a Stripe Terminal
// (phone-as-terminal / Tap to Pay) with a finalized receipt's total.
type PaymentHandlers struct {
	Processor     payment.Processor
	Payments      storage.PaymentRepository
	Receipts      storage.ReceiptRepository
	Transactions  storage.TransactionRepository
	WebhookSecret string
	Currency      string
}

// ConnectionToken returns a short-lived secret the mobile app's Stripe
// Terminal SDK uses to connect to a reader.
func (h *PaymentHandlers) ConnectionToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	secret, err := h.Processor.CreateConnectionToken(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "create connection token: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret})
}

type createPaymentResponse struct {
	PaymentID       string `json:"payment_id"`
	PaymentIntentID string `json:"payment_intent_id"`
	ClientSecret    string `json:"client_secret"`
	AmountCents     int    `json:"amount_cents"`
}

// CreatePayment starts a card-present charge for a finalized receipt's
// total. Only a finalized receipt has a stable, complete total to charge.
func (h *PaymentHandlers) CreatePayment(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
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
	total := 0
	for _, tx := range lines {
		total += tx.TotalPriceCents
	}
	if total <= 0 {
		writeError(w, http.StatusBadRequest, "receipt total must be greater than zero")
		return
	}

	intentID, clientSecret, err := h.Processor.CreatePaymentIntent(r.Context(), int64(total), h.Currency, map[string]string{
		"receipt_id": rc.ID,
		"tenant_id":  rc.TenantID,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "create payment intent: "+err.Error())
		return
	}

	now := time.Now().UTC()
	p := &domain.Payment{
		ID:                    idgen.New(),
		TenantID:              rc.TenantID,
		ReceiptID:             rc.ID,
		StripePaymentIntentID: intentID,
		AmountCents:           total,
		Status:                domain.PaymentStatusPending,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := h.Payments.Create(r.Context(), p); err != nil {
		writeError(w, http.StatusInternalServerError, "store payment: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, createPaymentResponse{
		PaymentID:       p.ID,
		PaymentIntentID: intentID,
		ClientSecret:    clientSecret,
		AmountCents:     total,
	})
}

// StripeWebhook receives payment_intent lifecycle events from Stripe and
// updates the matching local Payment's status. It is not behind the auth
// middleware: Stripe authenticates via a signed payload, not a bearer
// token.
func (h *PaymentHandlers) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	// IgnoreAPIVersionMismatch: Stripe sends events at whatever API version
	// the dashboard's webhook endpoint is configured for, which is
	// independent of (and can drift from) whichever stripe-go version this
	// service is pinned to. We only read stable fields (event type, the
	// payment intent's ID), so a version mismatch isn't a real risk here.
	event, err := webhook.ConstructEventWithOptions(payload, r.Header.Get("Stripe-Signature"), h.WebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid signature: "+err.Error())
		return
	}

	var newStatus domain.PaymentStatus
	switch event.Type {
	case "payment_intent.succeeded":
		newStatus = domain.PaymentStatusSucceeded
	case "payment_intent.payment_failed":
		newStatus = domain.PaymentStatusFailed
	default:
		w.WriteHeader(http.StatusOK)
		return
	}

	var intent stripego.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &intent); err != nil {
		writeError(w, http.StatusBadRequest, "decode payment intent: "+err.Error())
		return
	}

	p, err := h.Payments.GetByStripePaymentIntentID(r.Context(), intent.ID)
	if err != nil {
		// Unknown to us (e.g. a Stripe test event, or a charge not created
		// through this API): acknowledge anyway so Stripe doesn't retry.
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.Payments.UpdateStatus(r.Context(), p.ID, newStatus); err != nil {
		writeError(w, http.StatusInternalServerError, "update payment status: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
