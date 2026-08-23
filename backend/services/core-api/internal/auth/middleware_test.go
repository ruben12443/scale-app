package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
	"scale-app/backend/services/core-api/internal/storage/memory"
)

type fakeVerifier struct {
	subject string
	err     error
}

func (f fakeVerifier) Verify(ctx context.Context, rawToken string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.subject, nil
}

func newTestUsers(t *testing.T, u *domain.User) storage.UserRepository {
	t.Helper()
	s := memory.New()
	if u != nil {
		if err := s.Users().Create(context.Background(), u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	return s.Users()
}

func TestMiddlewareMissingToken(t *testing.T) {
	users := newTestUsers(t, nil)
	handler := Middleware(fakeVerifier{}, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without a token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareInvalidToken(t *testing.T) {
	users := newTestUsers(t, nil)
	handler := Middleware(fakeVerifier{err: errors.New("bad signature")}, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with an invalid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer whatever")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareUnknownSubject(t *testing.T) {
	users := newTestUsers(t, nil)
	handler := Middleware(fakeVerifier{subject: "sub-1"}, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for an unknown local user")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer whatever")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestMiddlewareSuccessAttachesUser(t *testing.T) {
	want := &domain.User{ID: "u1", TenantID: "t1", ZitadelSubjectID: "sub-1", Role: domain.RoleVendor}
	users := newTestUsers(t, want)

	var gotUser *domain.User
	handler := Middleware(fakeVerifier{subject: "sub-1"}, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			t.Fatal("expected a user in context")
		}
		gotUser = u
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer whatever")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUser == nil || gotUser.ID != want.ID {
		t.Fatalf("context user = %+v, want ID %q", gotUser, want.ID)
	}
}

func TestRequireRoleAllowsMatchingRole(t *testing.T) {
	called := false
	handler := RequireRole(domain.RoleAdmin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ContextWithUser(req.Context(), &domain.User{Role: domain.RoleAdmin}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected the wrapped handler to be called for a matching role")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRoleDeniesMismatchedRole(t *testing.T) {
	handler := RequireRole(domain.RoleAdmin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for a mismatched role")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ContextWithUser(req.Context(), &domain.User{Role: domain.RoleVendor}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireRoleDeniesMissingUser(t *testing.T) {
	handler := RequireRole(domain.RoleAdmin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without an authenticated user")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
