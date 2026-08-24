package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
	"scale-app/backend/services/core-api/internal/storage/memory"
)

func newTestUserHandlers(t *testing.T) (*UserHandlers, storage.Store, *auth.FakeAdminClient) {
	t.Helper()
	store := memory.New()
	admin := auth.NewFakeAdminClient()
	return &UserHandlers{Users: store.Users(), Admin: admin}, store, admin
}

func requestAs(actor *domain.User, method, target string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	if actor != nil {
		req = req.WithContext(auth.ContextWithUser(context.Background(), actor))
	}
	return req
}

func TestUserHandlersCreate(t *testing.T) {
	h, store, admin := newTestUserHandlers(t)
	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}

	body, _ := json.Marshal(createUserRequest{Email: "vendor@example.com", DisplayName: "Jane Vendor"})
	req := requestAs(actor, http.MethodPost, "/users", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var created domain.User
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.TenantID != "tenant-1" || created.Role != domain.RoleVendor || created.Email != "vendor@example.com" {
		t.Fatalf("unexpected created user: %+v", created)
	}
	if _, ok := admin.Created[created.ZitadelSubjectID]; !ok {
		t.Fatalf("expected admin client to have created subject %q", created.ZitadelSubjectID)
	}

	stored, err := store.Users().Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("stored user not found: %v", err)
	}
	if stored.Email != created.Email {
		t.Fatalf("stored user email = %q, want %q", stored.Email, created.Email)
	}
}

func TestUserHandlersCreateValidation(t *testing.T) {
	h, _, _ := newTestUserHandlers(t)
	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}

	body, _ := json.Marshal(createUserRequest{Email: "", DisplayName: ""})
	req := requestAs(actor, http.MethodPost, "/users", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUserHandlersCreateUnauthenticated(t *testing.T) {
	h, _, _ := newTestUserHandlers(t)
	body, _ := json.Marshal(createUserRequest{Email: "a@b.com", DisplayName: "A"})
	req := requestAs(nil, http.MethodPost, "/users", body)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlersListScopedToTenant(t *testing.T) {
	h, store, _ := newTestUserHandlers(t)
	ctx := context.Background()
	_ = store.Users().Create(ctx, &domain.User{ID: "u1", TenantID: "tenant-1"})
	_ = store.Users().Create(ctx, &domain.User{ID: "u2", TenantID: "tenant-1"})
	_ = store.Users().Create(ctx, &domain.User{ID: "u3", TenantID: "tenant-2"})

	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	req := requestAs(actor, http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var users []domain.User
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2 (tenant-scoped)", len(users))
	}
}

func TestUserHandlersDelete(t *testing.T) {
	h, store, admin := newTestUserHandlers(t)
	ctx := context.Background()
	target := &domain.User{ID: "u1", TenantID: "tenant-1", ZitadelSubjectID: "sub-1"}
	_ = store.Users().Create(ctx, target)
	admin.Created["sub-1"] = auth.FakeCreatedUser{}

	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	req := requestAs(actor, http.MethodDelete, "/users/u1", nil)
	req.SetPathValue("id", "u1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !admin.Deleted["sub-1"] {
		t.Fatal("expected the identity to be deleted via the admin client")
	}
	if _, err := store.Users().Get(ctx, "u1"); err == nil {
		t.Fatal("expected the local user record to be deleted")
	}
}

func TestUserHandlersDeleteNotFound(t *testing.T) {
	h, _, _ := newTestUserHandlers(t)
	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	req := requestAs(actor, http.MethodDelete, "/users/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUserHandlersDeleteCrossTenantReportsNotFound(t *testing.T) {
	h, store, _ := newTestUserHandlers(t)
	ctx := context.Background()
	_ = store.Users().Create(ctx, &domain.User{ID: "u1", TenantID: "tenant-2", ZitadelSubjectID: "sub-1"})

	actor := &domain.User{ID: "admin-1", TenantID: "tenant-1", Role: domain.RoleAdmin}
	req := requestAs(actor, http.MethodDelete, "/users/u1", nil)
	req.SetPathValue("id", "u1")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (cross-tenant delete should not leak existence)", rec.Code, http.StatusNotFound)
	}
}

func TestUserHandlersMe(t *testing.T) {
	h, _, _ := newTestUserHandlers(t)
	actor := &domain.User{ID: "u1", TenantID: "tenant-1", Role: domain.RoleVendor, DisplayName: "Jane"}
	req := requestAs(actor, http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	h.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got domain.User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "u1" || got.DisplayName != "Jane" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestUserHandlersMeUnauthenticated(t *testing.T) {
	h, _, _ := newTestUserHandlers(t)
	req := requestAs(nil, http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	h.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
