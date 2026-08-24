package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82/webhook"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/payment"
	"scale-app/backend/services/core-api/internal/storage"
	"scale-app/backend/services/core-api/internal/storage/memory"
)

const testWebhookSecret = "whsec_test_secret"

func newTestPaymentHandlers(t *testing.T) (*PaymentHandlers, storage.Store, *payment.FakeProcessor) {
	t.Helper()
	store := memory.New()
	proc := payment.NewFakeProcessor()
	return &PaymentHandlers{
		Processor:     proc,
		Payments:      store.Payments(),
		Receipts:      store.Receipts(),
		Transactions:  store.Transactions(),
		WebhookSecret: testWebhookSecret,
		Currency:      "chf",
	}, store, proc
}

func TestConnectionToken(t *testing.T) {
	h, _, _ := newTestPaymentHandlers(t)
	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/payments/connection-token", nil)
	rec := httptest.NewRecorder()
	h.ConnectionToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["secret"] == "" {
		t.Fatal("expected a non-empty secret")
	}
}

func finalizedReceiptWithTotal(t *testing.T, store storage.Store, tenantID, userID string, totalCents int) *domain.Receipt {
	t.Helper()
	ctx := context.Background()
	tx := &domain.Transaction{ID: "tx1", TenantID: tenantID, UserID: userID, TotalPriceCents: totalCents}
	if err := store.Transactions().Create(ctx, tx); err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	rc := &domain.Receipt{ID: "r1", TenantID: tenantID, UserID: userID, Status: domain.ReceiptStatusDraft}
	_ = rc.AddLine("tx1")
	_ = rc.Finalize(1, time.Now().UTC())
	if err := store.Receipts().Create(ctx, rc); err != nil {
		t.Fatalf("create receipt: %v", err)
	}
	return rc
}

func TestCreatePaymentSuccess(t *testing.T) {
	h, store, proc := newTestPaymentHandlers(t)
	finalizedReceiptWithTotal(t, store, "t1", "vendor-1", 624)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/r1/payment", nil)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()
	h.CreatePayment(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var resp createPaymentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AmountCents != 624 || resp.PaymentIntentID == "" || resp.ClientSecret == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if _, ok := proc.Intents[resp.PaymentIntentID]; !ok {
		t.Fatalf("expected processor to have recorded intent %q", resp.PaymentIntentID)
	}

	stored, err := store.Payments().Get(context.Background(), resp.PaymentID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if stored.Status != domain.PaymentStatusPending || stored.StripePaymentIntentID != resp.PaymentIntentID {
		t.Fatalf("unexpected stored payment: %+v", stored)
	}
}

func TestCreatePaymentRejectsDraftReceipt(t *testing.T) {
	h, store, _ := newTestPaymentHandlers(t)
	ctx := context.Background()
	_ = store.Receipts().Create(ctx, &domain.Receipt{ID: "r1", TenantID: "t1", UserID: "vendor-1", Status: domain.ReceiptStatusDraft})

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/r1/payment", nil)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()
	h.CreatePayment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreatePaymentRejectsCrossTenantReceipt(t *testing.T) {
	h, store, _ := newTestPaymentHandlers(t)
	finalizedReceiptWithTotal(t, store, "other-tenant", "vendor-2", 624)

	actor := &domain.User{ID: "vendor-1", TenantID: "t1", Role: domain.RoleVendor}
	req := requestAs(actor, http.MethodPost, "/receipts/r1/payment", nil)
	req.SetPathValue("id", "r1")
	rec := httptest.NewRecorder()
	h.CreatePayment(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// signedWebhookRequest builds a genuinely Stripe-signed request body using
// the SDK's own test-signing helper, so the handler's signature
// verification is exercised for real, not mocked.
func signedWebhookRequest(t *testing.T, eventType, paymentIntentID string) *http.Request {
	t.Helper()
	payload := []byte(`{
		"id": "evt_test_1",
		"type": "` + eventType + `",
		"data": {"object": {"id": "` + paymentIntentID + `", "object": "payment_intent", "status": "succeeded"}}
	}`)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    testWebhookSecret,
		Timestamp: time.Now(),
	})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	return req
}

func TestStripeWebhookUpdatesPaymentStatusOnSuccess(t *testing.T) {
	h, store, _ := newTestPaymentHandlers(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p := &domain.Payment{ID: "pay1", TenantID: "t1", ReceiptID: "r1", StripePaymentIntentID: "pi_test_123", Status: domain.PaymentStatusPending, CreatedAt: now, UpdatedAt: now}
	_ = store.Payments().Create(ctx, p)

	req := signedWebhookRequest(t, "payment_intent.succeeded", "pi_test_123")
	rec := httptest.NewRecorder()
	h.StripeWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	updated, err := store.Payments().Get(ctx, "pay1")
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if updated.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("Status = %q, want %q", updated.Status, domain.PaymentStatusSucceeded)
	}
}

func TestStripeWebhookUpdatesPaymentStatusOnFailure(t *testing.T) {
	h, store, _ := newTestPaymentHandlers(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p := &domain.Payment{ID: "pay1", TenantID: "t1", ReceiptID: "r1", StripePaymentIntentID: "pi_test_123", Status: domain.PaymentStatusPending, CreatedAt: now, UpdatedAt: now}
	_ = store.Payments().Create(ctx, p)

	req := signedWebhookRequest(t, "payment_intent.payment_failed", "pi_test_123")
	rec := httptest.NewRecorder()
	h.StripeWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	updated, err := store.Payments().Get(ctx, "pay1")
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if updated.Status != domain.PaymentStatusFailed {
		t.Fatalf("Status = %q, want %q", updated.Status, domain.PaymentStatusFailed)
	}
}

func TestStripeWebhookRejectsBadSignature(t *testing.T) {
	h, _, _ := newTestPaymentHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader([]byte(`{"type":"payment_intent.succeeded"}`)))
	req.Header.Set("Stripe-Signature", "t=1,v1=deadbeef")
	rec := httptest.NewRecorder()
	h.StripeWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStripeWebhookAcknowledgesUnknownIntent(t *testing.T) {
	h, _, _ := newTestPaymentHandlers(t)
	req := signedWebhookRequest(t, "payment_intent.succeeded", "pi_unknown")
	rec := httptest.NewRecorder()
	h.StripeWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (an unknown intent should still be acknowledged)", rec.Code, http.StatusOK)
	}
}

func TestStripeWebhookIgnoresUnhandledEventType(t *testing.T) {
	h, _, _ := newTestPaymentHandlers(t)
	req := signedWebhookRequest(t, "charge.refunded", "pi_test_123")
	rec := httptest.NewRecorder()
	h.StripeWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
