package api

import (
	"net/http"

	"scale-app/backend/services/core-api/internal/auth"
	"scale-app/backend/services/core-api/internal/domain"
	"scale-app/backend/services/core-api/internal/payment"
	"scale-app/backend/services/core-api/internal/receipt"
	"scale-app/backend/services/core-api/internal/storage"
)

// Server is core-api's HTTP surface.
type Server struct {
	mux *http.ServeMux
}

// ServerConfig configures NewServer.
type ServerConfig struct {
	Store               storage.Store
	Verifier            auth.TokenVerifier
	Admin               auth.AdminClient
	EmailSender         receipt.EmailSender
	PaymentProcessor    payment.Processor
	StripeWebhookSecret string
	// Currency is the ISO currency code (lowercase, e.g. "chf", "usd") used
	// for payment intents. core-api handles a single currency per
	// deployment for now.
	Currency string
}

// NewServer wires storage, auth, and every handler group into a Server.
func NewServer(cfg ServerConfig) *Server {
	store := cfg.Store
	s := &Server{mux: http.NewServeMux()}
	authenticated := auth.Middleware(cfg.Verifier, store.Users())
	adminOnly := func(h http.HandlerFunc) http.Handler {
		return authenticated(auth.RequireRole(domain.RoleAdmin, h))
	}
	anyRole := func(h http.HandlerFunc) http.Handler {
		return authenticated(h)
	}

	users := &UserHandlers{Users: store.Users(), Admin: cfg.Admin}
	s.mux.Handle("GET /me", anyRole(users.Me))
	s.mux.Handle("POST /users", adminOnly(users.Create))
	s.mux.Handle("GET /users", adminOnly(users.List))
	s.mux.Handle("DELETE /users/{id}", adminOnly(users.Delete))

	products := &ProductHandlers{Products: store.Products()}
	s.mux.Handle("GET /products", anyRole(products.List))
	s.mux.Handle("POST /products", adminOnly(products.Create))
	s.mux.Handle("PUT /products/{id}", adminOnly(products.Update))
	s.mux.Handle("DELETE /products/{id}", adminOnly(products.Delete))

	transactions := &TransactionHandlers{Products: store.Products(), Transactions: store.Transactions(), Receipts: store.Receipts()}
	s.mux.Handle("POST /transactions", anyRole(transactions.Create))

	receipts := &ReceiptHandlers{Receipts: store.Receipts(), Transactions: store.Transactions()}
	s.mux.Handle("GET /receipts/current", anyRole(receipts.GetCurrent))
	s.mux.Handle("DELETE /receipts/current/lines/{transactionId}", anyRole(receipts.RemoveLine))

	finalize := &ReceiptFinalizeHandlers{
		Receipts:     store.Receipts(),
		Transactions: store.Transactions(),
		Tenants:      store.Tenants(),
		EmailSender:  cfg.EmailSender,
	}
	s.mux.Handle("POST /receipts/current/finalize", anyRole(finalize.Finalize))
	s.mux.Handle("POST /receipts/{id}/email", anyRole(finalize.Email))

	payments := &PaymentHandlers{
		Processor:     cfg.PaymentProcessor,
		Payments:      store.Payments(),
		Receipts:      store.Receipts(),
		Transactions:  store.Transactions(),
		WebhookSecret: cfg.StripeWebhookSecret,
		Currency:      cfg.Currency,
	}
	s.mux.Handle("POST /payments/connection-token", anyRole(payments.ConnectionToken))
	s.mux.Handle("POST /receipts/{id}/payment", anyRole(payments.CreatePayment))
	// Stripe calls this directly with a signature, not a bearer token, so it
	// is deliberately not wrapped in the auth middleware.
	s.mux.HandleFunc("POST /webhooks/stripe", payments.StripeWebhook)

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
