package api

import (
	"net/http"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/storage"
)

// Server is core-api's HTTP surface.
type Server struct {
	mux *http.ServeMux
}

// NewServer wires storage, auth, and every handler group into a Server.
func NewServer(store storage.Store, verifier auth.TokenVerifier, admin auth.AdminClient) *Server {
	s := &Server{mux: http.NewServeMux()}
	authenticated := auth.Middleware(verifier, store.Users())
	adminOnly := func(h http.HandlerFunc) http.Handler {
		return authenticated(auth.RequireRole(domain.RoleAdmin, h))
	}
	anyRole := func(h http.HandlerFunc) http.Handler {
		return authenticated(h)
	}

	users := &UserHandlers{Users: store.Users(), Admin: admin}
	s.mux.Handle("POST /users", adminOnly(users.Create))
	s.mux.Handle("GET /users", adminOnly(users.List))
	s.mux.Handle("DELETE /users/{id}", adminOnly(users.Delete))

	products := &ProductHandlers{Products: store.Products()}
	s.mux.Handle("GET /products", anyRole(products.List))
	s.mux.Handle("POST /products", adminOnly(products.Create))
	s.mux.Handle("PUT /products/{id}", adminOnly(products.Update))
	s.mux.Handle("DELETE /products/{id}", adminOnly(products.Delete))

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
