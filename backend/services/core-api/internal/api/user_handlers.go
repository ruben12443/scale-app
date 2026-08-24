package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/idgen"
	"scale-app/backend/services/core-api/internal/storage"
)

// UserHandlers implements admin-only management of a tenant's vendor users.
// Every handler assumes auth.Middleware has already run and RequireRole has
// confirmed the caller is a domain.RoleAdmin; it always acts within the
// caller's own tenant.
type UserHandlers struct {
	Users storage.UserRepository
	Admin auth.AdminClient
}

type createUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// Create provisions a new vendor user: a Zitadel identity via Admin, then a
// local User record scoped to the caller's tenant.
func (h *UserHandlers) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Email == "" || req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "email and display_name are required")
		return
	}

	subjectID, err := h.Admin.CreateVendorUser(r.Context(), actor.TenantID, req.Email, req.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create identity: "+err.Error())
		return
	}

	user := &domain.User{
		ID:               idgen.New(),
		TenantID:         actor.TenantID,
		ZitadelSubjectID: subjectID,
		DisplayName:      req.DisplayName,
		Email:            req.Email,
		Role:             domain.RoleVendor,
		CreatedAt:        time.Now().UTC(),
	}
	if err := h.Users.Create(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "store user: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// Me returns the caller's own user record. It's the only way a client can
// learn its own tenant/role/display name after logging in via Zitadel,
// since the ID token's subject alone doesn't carry that.
func (h *UserHandlers) Me(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	writeJSON(w, http.StatusOK, actor)
}

// List returns every user in the caller's tenant.
func (h *UserHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	users, err := h.Users.ListByTenant(r.Context(), actor.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list users: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// Delete removes a user from the caller's tenant, both locally and in
// Zitadel. A target from a different tenant is reported as not found rather
// than forbidden, so the endpoint doesn't leak cross-tenant existence.
func (h *UserHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	id := r.PathValue("id")
	target, err := h.Users.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get user: "+err.Error())
		return
	}
	if target.TenantID != actor.TenantID {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := h.Admin.DeleteUser(r.Context(), target.ZitadelSubjectID); err != nil {
		writeError(w, http.StatusBadGateway, "delete identity: "+err.Error())
		return
	}
	if err := h.Users.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete user: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
