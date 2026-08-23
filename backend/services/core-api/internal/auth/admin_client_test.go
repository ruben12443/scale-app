package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestZitadelAdminClientCreateVendorUser(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody createHumanUserRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(createHumanUserResponse{UserID: "new-user-id"})
	}))
	defer srv.Close()

	client := &ZitadelAdminClient{BaseURL: srv.URL, BearerToken: "service-token"}
	subjectID, err := client.CreateVendorUser(t.Context(), "org-1", "vendor@example.com", "Jane Vendor")
	if err != nil {
		t.Fatalf("CreateVendorUser returned error: %v", err)
	}

	if subjectID != "new-user-id" {
		t.Fatalf("subjectID = %q, want %q", subjectID, "new-user-id")
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPath != "/v2/users/human" {
		t.Fatalf("path = %q, want %q", gotPath, "/v2/users/human")
	}
	if gotAuth != "Bearer service-token" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer service-token")
	}
	if gotBody.Organization.OrgID != "org-1" {
		t.Fatalf("Organization.OrgID = %q, want %q", gotBody.Organization.OrgID, "org-1")
	}
	if gotBody.Email.Email != "vendor@example.com" {
		t.Fatalf("Email.Email = %q, want %q", gotBody.Email.Email, "vendor@example.com")
	}
	if gotBody.Password.Password == "" || !gotBody.Password.ChangeRequired {
		t.Fatalf("expected a non-empty temporary password with ChangeRequired=true, got %+v", gotBody.Password)
	}
}

func TestZitadelAdminClientCreateVendorUserErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"user already exists"}`))
	}))
	defer srv.Close()

	client := &ZitadelAdminClient{BaseURL: srv.URL, BearerToken: "service-token"}
	if _, err := client.CreateVendorUser(t.Context(), "org-1", "vendor@example.com", "Jane Vendor"); err == nil {
		t.Fatal("expected an error for a non-2xx response, got nil")
	}
}

func TestZitadelAdminClientDeleteUser(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &ZitadelAdminClient{BaseURL: srv.URL, BearerToken: "service-token"}
	if err := client.DeleteUser(t.Context(), "user-1"); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodDelete)
	}
	if gotPath != "/v2/users/user-1" {
		t.Fatalf("path = %q, want %q", gotPath, "/v2/users/user-1")
	}
}

func TestFakeAdminClientRoundTrip(t *testing.T) {
	client := NewFakeAdminClient()

	id, err := client.CreateVendorUser(t.Context(), "org-1", "vendor@example.com", "Jane Vendor")
	if err != nil {
		t.Fatalf("CreateVendorUser returned error: %v", err)
	}
	if created, ok := client.Created[id]; !ok || created.Email != "vendor@example.com" {
		t.Fatalf("Created[%q] = %+v, ok=%v", id, created, ok)
	}

	if err := client.DeleteUser(t.Context(), id); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if !client.Deleted[id] {
		t.Fatalf("expected Deleted[%q] to be true", id)
	}

	if err := client.DeleteUser(t.Context(), "unknown"); err == nil {
		t.Fatal("expected an error deleting an unknown subject, got nil")
	}
}
