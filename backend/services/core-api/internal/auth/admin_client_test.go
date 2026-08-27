package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRauthyAdminClientCreateVendorUser(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody newUserRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(userResponse{ID: "new-user-id"})
	}))
	defer srv.Close()

	client := &RauthyAdminClient{BaseURL: srv.URL, APIKey: "bootstrap$service-secret"}
	subjectID, err := client.CreateVendorUser(t.Context(), "vendor@example.com", "Jane Vendor")
	if err != nil {
		t.Fatalf("CreateVendorUser returned error: %v", err)
	}

	if subjectID != "new-user-id" {
		t.Fatalf("subjectID = %q, want %q", subjectID, "new-user-id")
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPath != "/users" {
		t.Fatalf("path = %q, want %q", gotPath, "/users")
	}
	if gotAuth != "API-Key bootstrap$service-secret" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "API-Key bootstrap$service-secret")
	}
	if gotBody.Email != "vendor@example.com" {
		t.Fatalf("Email = %q, want %q", gotBody.Email, "vendor@example.com")
	}
	if gotBody.GivenName != "Jane Vendor" || gotBody.FamilyName != "Jane Vendor" {
		t.Fatalf("GivenName/FamilyName = %q/%q, want both %q", gotBody.GivenName, gotBody.FamilyName, "Jane Vendor")
	}
	if gotBody.Language != "en" {
		t.Fatalf("Language = %q, want %q", gotBody.Language, "en")
	}
	if gotBody.Roles == nil || len(gotBody.Roles) != 0 {
		t.Fatalf("Roles = %v, want an empty (non-nil) slice", gotBody.Roles)
	}
}

func TestRauthyAdminClientCreateVendorUserErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"user already exists"}`))
	}))
	defer srv.Close()

	client := &RauthyAdminClient{BaseURL: srv.URL, APIKey: "bootstrap$service-secret"}
	if _, err := client.CreateVendorUser(t.Context(), "vendor@example.com", "Jane Vendor"); err == nil {
		t.Fatal("expected an error for a non-2xx response, got nil")
	}
}

func TestRauthyAdminClientDeleteUser(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := &RauthyAdminClient{BaseURL: srv.URL, APIKey: "bootstrap$service-secret"}
	if err := client.DeleteUser(t.Context(), "user-1"); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodDelete)
	}
	if gotPath != "/users/user-1" {
		t.Fatalf("path = %q, want %q", gotPath, "/users/user-1")
	}
	if gotAuth != "API-Key bootstrap$service-secret" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "API-Key bootstrap$service-secret")
	}
}

func TestFakeAdminClientRoundTrip(t *testing.T) {
	client := NewFakeAdminClient()

	id, err := client.CreateVendorUser(t.Context(), "vendor@example.com", "Jane Vendor")
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
