package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/payment"
	"scale-app/backend/services/core-api/internal/receipt"
	"scale-app/backend/services/core-api/internal/storage"
	"scale-app/backend/services/core-api/internal/storage/memory"
)

// fakeVerifier lets server tests exercise the full auth.Middleware chain
// without a live Rauthy instance.
type fakeVerifier struct{ subject string }

func (f fakeVerifier) Verify(ctx context.Context, rawToken string) (string, error) {
	if rawToken != "valid-token" {
		return "", context.DeadlineExceeded // any non-nil error
	}
	return f.subject, nil
}

func newTestServer(store storage.Store, verifier auth.TokenVerifier) *Server {
	return NewServer(ServerConfig{
		Store:            store,
		Verifier:         verifier,
		Admin:            auth.NewFakeAdminClient(),
		EmailSender:      &receipt.FakeEmailSender{},
		PaymentProcessor: payment.NewFakeProcessor(),
		Currency:         "chf",
	})
}

func TestServerRejectsUnauthenticated(t *testing.T) {
	store := memory.New()
	srv := newTestServer(store, fakeVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServerRejectsNonAdminOnAdminRoute(t *testing.T) {
	store := memory.New()
	_ = store.Users().Create(context.Background(), &domain.User{ID: "u1", TenantID: "t1", RauthySubjectID: "sub-vendor", Role: domain.RoleVendor})
	srv := newTestServer(store, fakeVerifier{subject: "sub-vendor"})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestServerAllowsAdminEndToEnd(t *testing.T) {
	store := memory.New()
	_ = store.Users().Create(context.Background(), &domain.User{ID: "admin-1", TenantID: "t1", RauthySubjectID: "sub-admin", Role: domain.RoleAdmin})
	srv := newTestServer(store, fakeVerifier{subject: "sub-admin"})

	body := []byte(`{"email":"vendor@example.com","display_name":"Jane Vendor"}`)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestServerStripeWebhookIsNotBehindAuth(t *testing.T) {
	store := memory.New()
	srv := newTestServer(store, fakeVerifier{})

	// No Authorization header at all - the webhook route must still be
	// reachable (it will separately reject a bad signature, but that's a
	// 400, not the 401 an authenticated route would give).
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("status = %d, want the webhook route to be reachable without auth (expected a signature-related 400, not 401)", rec.Code)
	}
}
