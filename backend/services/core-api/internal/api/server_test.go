package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/receipt"
	"scale-app/backend/services/core-api/internal/storage/memory"
)

// fakeVerifier lets server tests exercise the full auth.Middleware chain
// without a live Zitadel instance.
type fakeVerifier struct{ subject string }

func (f fakeVerifier) Verify(ctx context.Context, rawToken string) (string, error) {
	if rawToken != "valid-token" {
		return "", context.DeadlineExceeded // any non-nil error
	}
	return f.subject, nil
}

func TestServerRejectsUnauthenticated(t *testing.T) {
	store := memory.New()
	srv := NewServer(store, fakeVerifier{}, auth.NewFakeAdminClient(), &receipt.FakeEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServerRejectsNonAdminOnAdminRoute(t *testing.T) {
	store := memory.New()
	_ = store.Users().Create(context.Background(), &domain.User{ID: "u1", TenantID: "t1", ZitadelSubjectID: "sub-vendor", Role: domain.RoleVendor})
	srv := NewServer(store, fakeVerifier{subject: "sub-vendor"}, auth.NewFakeAdminClient(), &receipt.FakeEmailSender{})

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
	_ = store.Users().Create(context.Background(), &domain.User{ID: "admin-1", TenantID: "t1", ZitadelSubjectID: "sub-admin", Role: domain.RoleAdmin})
	srv := NewServer(store, fakeVerifier{subject: "sub-admin"}, auth.NewFakeAdminClient(), &receipt.FakeEmailSender{})

	body := []byte(`{"email":"vendor@example.com","display_name":"Jane Vendor"}`)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}
